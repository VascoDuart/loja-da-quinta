package inventory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/db/sqlcgen"
)

// PostgresStore implementa Repo com as queries do sqlc (queries/inventory.sql).
type PostgresStore struct{ pool *pgxpool.Pool }

// NewPostgresStore cria o store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

var _ Repo = (*PostgresStore)(nil)

func (s *PostgresStore) CreateLot(ctx context.Context, in NewLot, unit string, userID uuid.UUID) (Lot, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Lot{}, fmt.Errorf("iniciar a transação: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // não faz nada depois do Commit
	q := sqlcgen.New(tx)

	r, err := q.InsertLot(ctx, sqlcgen.InsertLotParams{
		ProductID: in.ProductID, Unit: unit, QuantityMilli: in.QuantityMilli,
		HarvestedOn: in.HarvestedOn, ExpiresOn: in.ExpiresOn,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			// A chave (product_id, unit) falhou: o produto foi apagado ou
			// mudou de unidade entretanto. Quem distingue é o serviço,
			// perguntando ao catálogo.
			return Lot{}, errProductKey
		}
		return Lot{}, fmt.Errorf("gravar o lote: %w", err)
	}
	if err := q.InsertMovement(ctx, sqlcgen.InsertMovementParams{
		LotID: r.ID, Type: string(MovementEntry), QuantityMilli: in.QuantityMilli, UserID: userID,
	}); err != nil {
		return Lot{}, fmt.Errorf("gravar o movimento de entrada: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Lot{}, fmt.Errorf("gravar o lote: %w", err)
	}
	return Lot{
		ID: r.ID, ProductID: r.ProductID, Unit: r.Unit, QtyReceivedMilli: r.QtyReceivedMilli, QtyLeftMilli: r.QtyLeftMilli,
		HarvestedOn: r.HarvestedOn, ExpiresOn: r.ExpiresOn, CreatedAt: r.CreatedAt,
	}, nil
}

func (s *PostgresStore) HasLots(ctx context.Context, productID uuid.UUID) (bool, error) {
	has, err := sqlcgen.New(s.pool).ProductHasLots(ctx, productID)
	if err != nil {
		return false, fmt.Errorf("ver os lotes do produto: %w", err)
	}
	return has, nil
}

func (s *PostgresStore) StockMilli(ctx context.Context, productIDs []uuid.UUID, today time.Time) (map[uuid.UUID]int64, error) {
	rows, err := sqlcgen.New(s.pool).StockByProduct(ctx, sqlcgen.StockByProductParams{ProductIds: productIDs, Today: today})
	if err != nil {
		return nil, fmt.Errorf("ler o stock: %w", err)
	}
	out := make(map[uuid.UUID]int64, len(rows))
	for _, r := range rows {
		out[r.ProductID] = r.StockMilli
	}
	return out, nil
}
