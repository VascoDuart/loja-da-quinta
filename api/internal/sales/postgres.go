package sales

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/db/sqlcgen"
)

// StockLine is what a sale takes from one product, in thousandths of its unit.
type StockLine struct {
	ProductID     uuid.UUID
	QuantityMilli int64
}

// Stock takes a sale's stock inside the sale's transaction. It belongs to
// the inventory module, which owns the lots: this module never reads them. In
// the composition (internal/app) it is the inventory store. A product that
// falls short is reported as *InsufficientStockError.
type Stock interface {
	TakeForSale(ctx context.Context, tx pgx.Tx, saleID, userID uuid.UUID, lines []StockLine) error
}

// PostgresStore implements Repo with the sqlc queries (queries/sales.sql).
type PostgresStore struct {
	pool  *pgxpool.Pool
	stock Stock
}

// NewPostgresStore creates the store.
func NewPostgresStore(pool *pgxpool.Pool, stock Stock) *PostgresStore {
	return &PostgresStore{pool: pool, stock: stock}
}

var _ Repo = (*PostgresStore)(nil)

func (s *PostgresStore) Create(ctx context.Context, r NewRecord) (Sale, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Sale{}, fmt.Errorf("begin the transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit
	q := sqlcgen.New(tx)
	d := r.Draft
	// The sale goes in first: a concurrent request with the same key waits
	// here, on the unique index, and never touches the stock twice.
	row, err := q.InsertSale(ctx, sqlcgen.InsertSaleParams{
		UserID: r.UserID, Payment: string(d.Payment), TotalCents: d.TotalCents, VatCents: d.VatCents,
		IdempotencyKey: r.IdempotencyKey, RequestHash: r.RequestHash,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Sale{}, errKeyTaken
	}
	if err != nil {
		return Sale{}, fmt.Errorf("insert the sale: %w", err)
	}
	lines := make([]StockLine, len(d.Lines))
	for i, l := range d.Lines {
		lines[i] = StockLine{ProductID: l.ProductID, QuantityMilli: l.QuantityMilli}
	}
	if err := s.stock.TakeForSale(ctx, tx, row.ID, r.UserID, lines); err != nil {
		return Sale{}, err
	}
	for _, l := range d.Lines {
		if err := q.InsertSaleItem(ctx, sqlcgen.InsertSaleItemParams{
			SaleID: row.ID, ProductID: l.ProductID, ProductName: l.ProductName, Unit: l.Unit,
			QuantityMilli: l.QuantityMilli, UnitPriceCents: l.UnitPriceCents, VatRate: int16(l.VatRate), //nolint:gosec // 6, 13 or 23
			LineTotalCents: l.LineTotalCents, VatCents: l.VatCents,
		}); err != nil {
			return Sale{}, fmt.Errorf("insert a sale line: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Sale{}, fmt.Errorf("commit the transaction: %w", err)
	}
	return Sale{
		ID: row.ID, Number: row.Number, UserID: row.UserID, Payment: Payment(row.Payment), Status: Status(row.Status),
		TotalCents: row.TotalCents, VatCents: row.VatCents, CreatedAt: row.CreatedAt, Lines: d.Lines,
	}, nil
}

func (s *PostgresStore) SaleByKey(ctx context.Context, userID uuid.UUID, key string) (Sale, []byte, error) {
	q := sqlcgen.New(s.pool)
	row, err := q.GetSaleByKey(ctx, sqlcgen.GetSaleByKeyParams{UserID: userID, IdempotencyKey: key})
	if errors.Is(err, pgx.ErrNoRows) {
		return Sale{}, nil, ErrSaleNotFound
	}
	if err != nil {
		return Sale{}, nil, fmt.Errorf("read the sale: %w", err)
	}
	items, err := q.ListSaleItems(ctx, row.ID)
	if err != nil {
		return Sale{}, nil, fmt.Errorf("read the sale lines: %w", err)
	}
	lines := make([]Line, len(items))
	for i, it := range items {
		lines[i] = Line{
			ProductID: it.ProductID, ProductName: it.ProductName, Unit: it.Unit, QuantityMilli: it.QuantityMilli,
			UnitPriceCents: it.UnitPriceCents, VatRate: int(it.VatRate), LineTotalCents: it.LineTotalCents, VatCents: it.VatCents,
		}
	}
	SortLines(lines)
	return Sale{
		ID: row.ID, Number: row.Number, UserID: row.UserID, Payment: Payment(row.Payment), Status: Status(row.Status),
		TotalCents: row.TotalCents, VatCents: row.VatCents, CreatedAt: row.CreatedAt, Lines: lines,
	}, row.RequestHash, nil
}
