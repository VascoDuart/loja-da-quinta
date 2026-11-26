package sales

import (
	"bytes"
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// Service has the sales use cases. It checks the role again, as the other
// modules do.
type Service struct {
	repo     Repo
	products Products
}

// NewService creates the service.
func NewService(repo Repo, products Products) *Service {
	return &Service{repo: repo, products: products}
}

// Register records a counter sale (UC-06). Prices and VAT come from the
// catalog; the stock is taken FEFO in the same transaction. The same
// idempotency key never records two sales: a retry of the same cart returns
// the sale already recorded (created false); a different cart with that key
// gets ErrKeyReused.
func (s *Service) Register(ctx context.Context, actor identity.Principal, key string, in NewSale) (Sale, bool, error) {
	if err := requireStaff(actor); err != nil {
		return Sale{}, false, err
	}
	in = in.Normalized()
	if err := in.Validate(); err != nil {
		return Sale{}, false, err
	}
	hash := in.Fingerprint()
	// A retry after the sale was recorded must not depend on the catalog or
	// the stock of now: answer it first.
	if sale, err := s.replay(ctx, actor.UserID, key, hash); !errors.Is(err, ErrSaleNotFound) {
		return sale, false, err
	}
	ids := make([]uuid.UUID, len(in.Items))
	for i, it := range in.Items {
		ids[i] = it.ProductID
	}
	products, err := s.products.ProductsByID(ctx, ids)
	if err != nil {
		return Sale{}, false, err
	}
	draft, err := Price(in, products)
	if err != nil {
		return Sale{}, false, err
	}
	sale, err := s.repo.Create(ctx, NewRecord{UserID: actor.UserID, IdempotencyKey: key, RequestHash: hash, Draft: draft})
	var short *InsufficientStockError
	switch {
	case errors.Is(err, errKeyTaken):
		sale, err := s.replay(ctx, actor.UserID, key, hash)
		return sale, false, err
	case errors.As(err, &short):
		short.ProductName = products[short.ProductID].Name
		if short.Unit == "" {
			short.Unit = products[short.ProductID].Unit
		}
		return Sale{}, false, short
	case err != nil:
		return Sale{}, false, err
	}
	return sale, true, nil
}

// replay returns the user's sale recorded with key, if the request is the
// same one (ErrKeyReused if not). ErrSaleNotFound if the key is unused.
func (s *Service) replay(ctx context.Context, userID uuid.UUID, key string, hash []byte) (Sale, error) {
	sale, stored, err := s.repo.SaleByKey(ctx, userID, key)
	if err != nil {
		return Sale{}, err
	}
	if !bytes.Equal(stored, hash) {
		return Sale{}, ErrKeyReused
	}
	return sale, nil
}

// requireStaff accepts the clerk and the manager (docs/security/authz-matrix.md).
func requireStaff(actor identity.Principal) error {
	if !actor.Role.Valid() || actor.UserID == uuid.Nil {
		return identity.ErrForbidden
	}
	return nil
}
