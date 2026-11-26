package inventory

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/db/sqlcgen"
)

// SaleLine is what a sale takes from the stock of one product, in thousandths
// of the product's unit.
type SaleLine struct {
	ProductID     uuid.UUID
	QuantityMilli int64
}

// SaleTake is the stock outflow of one sale (#15).
type SaleTake struct {
	SaleID uuid.UUID
	UserID uuid.UUID
	// Today is the day in Lisbon (Day): lots that expired before it are not
	// sold.
	Today time.Time
	Lines []SaleLine
}

// ShortageError: a product does not have enough sellable stock for the sale.
// AvailableMilli is what its lots within their expiry date still hold, in the
// unit Unit ("" when the product has no sellable lot).
type ShortageError struct {
	ProductID      uuid.UUID
	AvailableMilli int64
	Unit           string
}

func (e *ShortageError) Error() string { return "insufficient sellable stock for the sale" }

// TakeForSale takes the stock of a sale, inside the caller's transaction tx
// (the sale, its lines and these movements commit or roll back together).
// For each product, in id order, it locks the sellable lots in FEFO order
// (SELECT ... FOR UPDATE) and takes from the earliest expiry first, one
// "venda" movement per lot, all pointing to the sale. Locking products in id
// order and lots in expiry order means two sales always lock in the same
// order and never deadlock; the second one waits and then sees what the first
// left. If any product falls short it returns *ShortageError and the caller
// must roll back.
func (s *PostgresStore) TakeForSale(ctx context.Context, tx pgx.Tx, take SaleTake) error {
	lines := slices.Clone(take.Lines)
	slices.SortFunc(lines, func(a, b SaleLine) int { return slices.Compare(a.ProductID[:], b.ProductID[:]) })
	q := sqlcgen.New(tx)
	for _, line := range lines {
		rows, err := q.LockSellableLots(ctx, sqlcgen.LockSellableLotsParams{ProductID: line.ProductID, Today: take.Today})
		if err != nil {
			return fmt.Errorf("lock the lots of the product: %w", err)
		}
		lots := make([]Lot, len(rows))
		var available int64
		for i, r := range rows {
			lots[i] = toLot(sqlcgen.LockLotRow(r))
			available += lots[i].QtyLeftMilli
		}
		plan, err := PlanFEFO(lots, line.QuantityMilli)
		if err != nil {
			unit := ""
			if len(lots) > 0 {
				unit = lots[0].Unit
			}
			return &ShortageError{ProductID: line.ProductID, AvailableMilli: available, Unit: unit}
		}
		for i, takeMilli := range plan {
			if takeMilli == 0 {
				continue
			}
			if _, _, err := applyMovement(ctx, q, lots[i], Movement{
				LotID: lots[i].ID, Type: MovementSale, QuantityMilli: -takeMilli, UserID: take.UserID, SaleID: take.SaleID,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// errShortage: the lots do not hold the quantity asked for.
var errShortage = fmt.Errorf("not enough stock in the lots")

// PlanFEFO splits wantMilli over lots, already in FEFO order: how much to
// take from each one, earliest expiry first. errShortage if they do not hold
// that much together.
func PlanFEFO(lots []Lot, wantMilli int64) ([]int64, error) {
	plan := make([]int64, len(lots))
	left := wantMilli
	for i, l := range lots {
		if left == 0 {
			break
		}
		plan[i] = min(l.QtyLeftMilli, left)
		left -= plan[i]
	}
	if left > 0 {
		return nil, errShortage
	}
	return plan, nil
}
