package inventory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/db/sqlcgen"
)

// PostgresStore implements Repo with the sqlc queries (queries/inventory.sql).
type PostgresStore struct{ pool *pgxpool.Pool }

// NewPostgresStore creates the store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

var (
	_ Repo      = (*PostgresStore)(nil)
	_ AlertDays = (*PostgresStore)(nil)
)

func (s *PostgresStore) CreateLot(ctx context.Context, in NewLot, unit string, userID uuid.UUID) (Lot, error) {
	var lot Lot
	err := s.inTx(ctx, func(q *sqlcgen.Queries) error {
		r, err := q.InsertLot(ctx, sqlcgen.InsertLotParams{
			ProductID: in.ProductID, Unit: unit, QuantityMilli: in.QuantityMilli,
			HarvestedOn: in.HarvestedOn, ExpiresOn: in.ExpiresOn,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23503" {
				// The (product_id, unit) key failed: the product was deleted
				// or changed unit meanwhile. The service tells which, by
				// asking the catalog.
				return errProductKey
			}
			return fmt.Errorf("insert the lot: %w", err)
		}
		if _, err := q.InsertMovement(ctx, sqlcgen.InsertMovementParams{
			LotID: r.ID, Type: string(MovementEntry), QuantityMilli: in.QuantityMilli, UserID: userID,
		}); err != nil {
			return fmt.Errorf("insert the entry movement: %w", err)
		}
		lot = toLot(sqlcgen.LockLotRow(r))
		return nil
	})
	return lot, err
}

func (s *PostgresStore) Lot(ctx context.Context, id uuid.UUID) (Lot, error) {
	r, err := sqlcgen.New(s.pool).GetLot(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Lot{}, ErrLotNotFound
	}
	if err != nil {
		return Lot{}, fmt.Errorf("read the lot: %w", err)
	}
	return toLot(sqlcgen.LockLotRow(r)), nil
}

func (s *PostgresStore) ListLots(ctx context.Context, productID uuid.UUID, today time.Time) ([]Lot, error) {
	rows, err := sqlcgen.New(s.pool).ListLots(ctx, sqlcgen.ListLotsParams{ProductID: productID, Today: today})
	if err != nil {
		return nil, fmt.Errorf("list the lots: %w", err)
	}
	out := make([]Lot, len(rows))
	for i, r := range rows {
		out[i] = toLot(sqlcgen.LockLotRow(r))
	}
	return out, nil
}

func (s *PostgresStore) Move(ctx context.Context, m Movement) (Lot, RecordedMovement, error) {
	var (
		lot Lot
		rec RecordedMovement
	)
	err := s.inTx(ctx, func(q *sqlcgen.Queries) error {
		locked, err := lockLot(ctx, q, m.LotID)
		if err != nil {
			return err
		}
		lot, rec, err = applyMovement(ctx, q, locked, m)
		return err
	})
	return lot, rec, err
}

// inTx runs fn in a transaction: commits if fn succeeds, rolls back if not.
func (s *PostgresStore) inTx(ctx context.Context, fn func(q *sqlcgen.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin the transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit
	if err := fn(sqlcgen.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit the transaction: %w", err)
	}
	return nil
}

// lockLot reads the lot and locks it until the end of the transaction
// (SELECT ... FOR UPDATE). Whoever locks the same lot next waits, and then
// reads what this one left. Sales (TakeForSale) lock a product's lots in FEFO
// order instead and apply applyMovement to them, one by one.
func lockLot(ctx context.Context, q *sqlcgen.Queries, id uuid.UUID) (Lot, error) {
	r, err := q.LockLot(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Lot{}, ErrLotNotFound
	}
	if err != nil {
		return Lot{}, fmt.Errorf("lock the lot: %w", err)
	}
	return toLot(r), nil
}

// applyMovement applies m to a lot already locked in the same transaction
// (lockLot, LockSellableLots): it checks that the stock does not go negative,
// adds the quantity to what is left and inserts the movement, with who made
// it. The database's CHECK (qty_left >= 0) is the last line of defence, if
// someone calls this without the lock.
func applyMovement(ctx context.Context, q *sqlcgen.Queries, locked Lot, m Movement) (Lot, RecordedMovement, error) {
	if _, err := locked.LeftAfter(m.QuantityMilli); err != nil {
		return Lot{}, RecordedMovement{}, err
	}
	r, err := q.AddToLot(ctx, sqlcgen.AddToLotParams{ID: locked.ID, DeltaMilli: m.QuantityMilli})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" { // check_violation: it would go negative
			return Lot{}, RecordedMovement{}, &InsufficientStockError{LeftMilli: locked.QtyLeftMilli, Unit: locked.Unit}
		}
		return Lot{}, RecordedMovement{}, fmt.Errorf("update the lot: %w", err)
	}
	var reason *string
	if m.Reason != "" {
		reason = &m.Reason
	}
	mv, err := q.InsertMovement(ctx, sqlcgen.InsertMovementParams{
		LotID: locked.ID, Type: string(m.Type), QuantityMilli: m.QuantityMilli, UserID: m.UserID, Reason: reason,
		SaleID: pgtype.UUID{Bytes: m.SaleID, Valid: m.SaleID != uuid.Nil},
	})
	if err != nil {
		return Lot{}, RecordedMovement{}, fmt.Errorf("insert the movement: %w", err)
	}
	return toLot(sqlcgen.LockLotRow(r)), RecordedMovement{
		ID: mv.ID, LotID: locked.ID, Type: m.Type, QuantityMilli: m.QuantityMilli, Reason: m.Reason, CreatedAt: mv.CreatedAt,
	}, nil
}

func toLot(r sqlcgen.LockLotRow) Lot {
	return Lot{
		ID: r.ID, ProductID: r.ProductID, Unit: r.Unit, QtyReceivedMilli: r.QtyReceivedMilli, QtyLeftMilli: r.QtyLeftMilli,
		HarvestedOn: r.HarvestedOn, ExpiresOn: r.ExpiresOn, CreatedAt: r.CreatedAt,
	}
}

func (s *PostgresStore) StockedLots(ctx context.Context) ([]Lot, error) {
	rows, err := sqlcgen.New(s.pool).ListStockedLots(ctx)
	if err != nil {
		return nil, fmt.Errorf("list the lots with stock: %w", err)
	}
	out := make([]Lot, len(rows))
	for i, r := range rows {
		out[i] = toLot(sqlcgen.LockLotRow(r))
	}
	return out, nil
}

func (s *PostgresStore) ClaimDay(ctx context.Context, day time.Time) (bool, error) {
	n, err := sqlcgen.New(s.pool).ClaimAlertDay(ctx, day)
	if err != nil {
		return false, fmt.Errorf("claim the alert day: %w", err)
	}
	return n == 1, nil
}

func (s *PostgresStore) ReleaseDay(ctx context.Context, day time.Time) error {
	if err := sqlcgen.New(s.pool).ReleaseAlertDay(ctx, day); err != nil {
		return fmt.Errorf("release the alert day: %w", err)
	}
	return nil
}

func (s *PostgresStore) HasLots(ctx context.Context, productID uuid.UUID) (bool, error) {
	has, err := sqlcgen.New(s.pool).ProductHasLots(ctx, productID)
	if err != nil {
		return false, fmt.Errorf("check the lots of the product: %w", err)
	}
	return has, nil
}

func (s *PostgresStore) StockMilli(ctx context.Context, productIDs []uuid.UUID, today time.Time) (map[uuid.UUID]int64, error) {
	rows, err := sqlcgen.New(s.pool).StockByProduct(ctx, sqlcgen.StockByProductParams{ProductIds: productIDs, Today: today})
	if err != nil {
		return nil, fmt.Errorf("read the stock: %w", err)
	}
	out := make(map[uuid.UUID]int64, len(rows))
	for _, r := range rows {
		out[r.ProductID] = r.StockMilli
	}
	return out, nil
}
