package inventory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	var lot Lot
	err := s.inTx(ctx, func(q *sqlcgen.Queries) error {
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
				return errProductKey
			}
			return fmt.Errorf("gravar o lote: %w", err)
		}
		if _, err := q.InsertMovement(ctx, sqlcgen.InsertMovementParams{
			LotID: r.ID, Type: string(MovementEntry), QuantityMilli: in.QuantityMilli, UserID: userID,
		}); err != nil {
			return fmt.Errorf("gravar o movimento de entrada: %w", err)
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
		return Lot{}, fmt.Errorf("ler o lote: %w", err)
	}
	return toLot(sqlcgen.LockLotRow(r)), nil
}

func (s *PostgresStore) ListLots(ctx context.Context, productID uuid.UUID, today time.Time) ([]Lot, error) {
	rows, err := sqlcgen.New(s.pool).ListLots(ctx, sqlcgen.ListLotsParams{ProductID: productID, Today: today})
	if err != nil {
		return nil, fmt.Errorf("listar os lotes: %w", err)
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

// inTx corre fn numa transação: confirma se fn não der erro, desfaz se der.
func (s *PostgresStore) inTx(ctx context.Context, fn func(q *sqlcgen.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar a transação: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // não faz nada depois do Commit
	if err := fn(sqlcgen.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("confirmar a transação: %w", err)
	}
	return nil
}

// lockLot lê o lote e bloqueia-o até ao fim da transação (SELECT ... FOR
// UPDATE). Quem bloquear o mesmo lote a seguir espera, e lê depois o que
// este deixar. As vendas por FEFO (#15) bloqueiam os lotes de um produto pela
// ordem da validade e aplicam-lhes applyMovement, um a um.
func lockLot(ctx context.Context, q *sqlcgen.Queries, id uuid.UUID) (Lot, error) {
	r, err := q.LockLot(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Lot{}, ErrLotNotFound
	}
	if err != nil {
		return Lot{}, fmt.Errorf("bloquear o lote: %w", err)
	}
	return toLot(r), nil
}

// applyMovement aplica m a um lote já bloqueado (lockLot, na mesma
// transação): confirma que o stock não fica negativo, soma a quantidade ao
// que resta e grava o movimento, com quem o fez. O CHECK (qty_left >= 0) da
// base de dados é a última defesa, se alguém chamar isto sem o bloqueio.
func applyMovement(ctx context.Context, q *sqlcgen.Queries, locked Lot, m Movement) (Lot, RecordedMovement, error) {
	if _, err := locked.LeftAfter(m.QuantityMilli); err != nil {
		return Lot{}, RecordedMovement{}, err
	}
	r, err := q.AddToLot(ctx, sqlcgen.AddToLotParams{ID: locked.ID, DeltaMilli: m.QuantityMilli})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" { // check_violation: ficaria negativo
			return Lot{}, RecordedMovement{}, &InsufficientStockError{LeftMilli: locked.QtyLeftMilli, Unit: locked.Unit}
		}
		return Lot{}, RecordedMovement{}, fmt.Errorf("atualizar o lote: %w", err)
	}
	var reason *string
	if m.Reason != "" {
		reason = &m.Reason
	}
	mv, err := q.InsertMovement(ctx, sqlcgen.InsertMovementParams{
		LotID: locked.ID, Type: string(m.Type), QuantityMilli: m.QuantityMilli, UserID: m.UserID, Reason: reason,
	})
	if err != nil {
		return Lot{}, RecordedMovement{}, fmt.Errorf("gravar o movimento: %w", err)
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
