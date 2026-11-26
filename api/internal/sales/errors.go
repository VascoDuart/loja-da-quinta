package sales

import (
	"errors"

	"github.com/google/uuid"
)

// ErrKeyReused: the idempotency key was already used for a different cart
// (422). Nothing is recorded.
var ErrKeyReused = errors.New("idempotency key already used for another sale")

// ErrSaleNotFound: no sale with that idempotency key.
var ErrSaleNotFound = errors.New("sale not found")

// errKeyTaken: Repo.Create found the key already taken (another request with
// it committed first). The service reads that sale back.
var errKeyTaken = errors.New("idempotency key taken")

// InsufficientStockError: a product does not have enough sellable stock (lots
// within their expiry date). Nothing is recorded (409). AvailableMilli is
// what there is, in the unit Unit.
type InsufficientStockError struct {
	ProductID      uuid.UUID
	ProductName    string
	AvailableMilli int64
	Unit           string
}

func (e *InsufficientStockError) Error() string { return "insufficient stock for the sale" }

// ValidationError is invalid input, with the message for the person using
// the app.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }
