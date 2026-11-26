package sales

import (
	"context"

	"github.com/google/uuid"
)

// Products is what sales need from the catalog, which owns the products:
// this module never reads the products table. In the composition
// (internal/app) it is the catalog service.
type Products interface {
	// ProductsByID returns the products that exist among ids; missing ones
	// are left out of the map.
	ProductsByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Product, error)
}

// NewRecord is a priced sale to record for a user under an idempotency key.
type NewRecord struct {
	UserID         uuid.UUID
	IdempotencyKey string
	RequestHash    []byte
	Draft          Draft
}

// Repo stores sales.
type Repo interface {
	// Create records the sale, its lines and its stock outflow (FEFO, through
	// the inventory module) in one transaction. *InsufficientStockError (with
	// ProductID, AvailableMilli and Unit) if a product falls short; then
	// nothing is recorded. errKeyTaken if the user already has a sale with
	// that key (a concurrent request committed it first).
	Create(ctx context.Context, r NewRecord) (Sale, error)
	// SaleByKey returns the user's sale recorded with key and the fingerprint
	// of the request that recorded it. ErrSaleNotFound if there is none.
	SaleByKey(ctx context.Context, userID uuid.UUID, key string) (Sale, []byte, error)
}
