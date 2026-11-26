// Package app composes the API: it creates the adapters, injects them into
// the modules and returns the HTTP handler. It is the same code in cmd/api and
// in the integration tests, so the tests exercise exactly what goes to
// production.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VascoDuart/loja-da-quinta/api/internal/auth"
	"github.com/VascoDuart/loja-da-quinta/api/internal/catalog"
	"github.com/VascoDuart/loja-da-quinta/api/internal/health"
	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi"
	"github.com/VascoDuart/loja-da-quinta/api/internal/inventory"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/config"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/httpserver"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/password"
	"github.com/VascoDuart/loja-da-quinta/api/internal/sales"
	"github.com/VascoDuart/loja-da-quinta/api/internal/users"
)

// Concurrent argon2id computations: 4 × 19 MiB fit easily in the 512 MiB of
// Render's free plan.
const maxConcurrentHashes = 4

// Option swaps a piece of the composition. Only the integration tests use them.
type Option func(*options)

type options struct {
	now      func() time.Time
	notifier inventory.Notifier
}

// WithClock fixes the clock of the stock, the sales and the alerts.
func WithClock(now func() time.Time) Option { return func(o *options) { o.now = now } }

// WithNotifier swaps how alerts are sent (for a fake).
func WithNotifier(n inventory.Notifier) Option { return func(o *options) { o.notifier = n } }

// New returns the complete HTTP handler of the API.
func New(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, opts ...Option) (http.Handler, error) {
	o := options{now: time.Now}
	for _, opt := range opts {
		opt(&o)
	}
	if o.notifier == nil {
		o.notifier = newNotifier(cfg, log)
	}
	tokens, err := newTokens(cfg, log)
	if err != nil {
		return nil, err
	}
	// users uses the session store to end sessions (deactivating a user,
	// changing the role); auth uses users to check credentials.
	sessions := auth.NewPostgresStore(pool)
	userSvc, err := users.NewService(ctx, users.NewPostgresRepo(pool), password.New(password.OWASP, maxConcurrentHashes), sessions)
	if err != nil {
		return nil, err
	}
	authSvc := auth.NewService(userSvc, sessions, tokens, log)
	// The stock asks the catalog for the products, and the catalog asks the
	// stock how much there is and whether a product has lots: each one only
	// knows the other's service.
	catalogStore := catalog.NewPostgresStore(pool)
	catalogSvc := catalog.NewService(catalogStore, catalogStore)
	inventoryStore := inventory.NewPostgresStore(pool)
	inventorySvc := inventory.NewService(inventoryStore, catalogProducts{catalogSvc}, o.now)
	catalogSvc.WithStock(inventorySvc)
	alerts := inventory.NewAlerts(inventorySvc, inventoryStore, newRecipients(cfg, userSvc), o.notifier, log)
	// A sale prices its lines with the catalog and takes the stock through
	// the inventory store, inside the sale's own transaction.
	salesSvc := sales.NewService(
		sales.NewPostgresStore(pool, saleStock{store: inventoryStore, now: o.now}),
		saleProducts{catalogSvc},
	)

	api := httpapi.New(
		health.NewHandler(pool, log, cfg.Version),
		auth.NewEndpoints(authSvc),
		users.NewEndpoints(userSvc),
		catalog.NewEndpoints(catalogSvc),
		inventory.NewEndpoints(inventorySvc, alerts),
		sales.NewEndpoints(salesSvc),
	)
	return httpserver.NewHandler(cfg, log, api, httpserver.Security{
		Authenticate: authSvc.Authenticate,
		Authorize:    auth.Authorizer(cfg.JobsToken),
	})
}

// newTokens reads the access token key. Locally, without a key, it uses a
// temporary one; the configuration already refuses to start without it in
// the other environments.
func newTokens(cfg config.Config, log *slog.Logger) (*auth.JWT, error) {
	if cfg.JWTSigningKey == "" {
		log.Warn("no JWT_SIGNING_KEY: using a temporary key (development only)")
		return auth.NewEphemeralJWT(), nil
	}
	return auth.NewJWT(cfg.JWTSigningKey, cfg.JWTVerifyKeys)
}

// catalogProducts gives the stock what it needs from a product, from the
// catalog service (inventory never reads the products table).
type catalogProducts struct{ svc *catalog.Service }

func (c catalogProducts) Product(ctx context.Context, id uuid.UUID) (inventory.Product, error) {
	p, err := c.svc.GetProduct(ctx, id)
	if errors.Is(err, catalog.ErrNotFound) {
		return inventory.Product{}, inventory.ErrProductNotFound
	}
	if err != nil {
		return inventory.Product{}, err
	}
	return stockProduct(p), nil
}

func (c catalogProducts) Products(ctx context.Context) ([]inventory.Product, error) {
	all, err := c.svc.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]inventory.Product, len(all))
	for i, p := range all {
		out[i] = stockProduct(p)
	}
	return out, nil
}

func stockProduct(p catalog.Product) inventory.Product {
	return inventory.Product{
		ID: p.ID, Name: p.Name, Unit: string(p.Unit), Decimal: p.Unit.Decimal(), Active: p.Active,
		LowStockMilli: p.LowStockMilli,
	}
}

// saleProducts gives sales the catalog's products: name, unit, price and VAT
// (the sales module never reads the products table).
type saleProducts struct{ svc *catalog.Service }

func (c saleProducts) ProductsByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]sales.Product, error) {
	out := make(map[uuid.UUID]sales.Product, len(ids))
	for _, id := range ids {
		p, err := c.svc.GetProduct(ctx, id)
		if errors.Is(err, catalog.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[id] = sales.Product{
			ID: p.ID, Name: p.Name, Unit: string(p.Unit), Decimal: p.Unit.Decimal(), Active: p.Active,
			PriceCents: int64(p.PriceCents), VatRate: p.VatRate,
		}
	}
	return out, nil
}

// saleStock takes a sale's stock with the inventory store, in the sale's
// transaction, and turns a shortage into the sales module's error. "Today" is
// the day in Lisbon on the stock's clock.
type saleStock struct {
	store *inventory.PostgresStore
	now   func() time.Time
}

func (s saleStock) TakeForSale(ctx context.Context, tx pgx.Tx, saleID, userID uuid.UUID, lines []sales.StockLine) error {
	take := inventory.SaleTake{SaleID: saleID, UserID: userID, Today: inventory.Day(s.now()), Lines: make([]inventory.SaleLine, len(lines))}
	for i, l := range lines {
		take.Lines[i] = inventory.SaleLine{ProductID: l.ProductID, QuantityMilli: l.QuantityMilli}
	}
	err := s.store.TakeForSale(ctx, tx, take)
	var short *inventory.ShortageError
	if errors.As(err, &short) {
		return &sales.InsufficientStockError{ProductID: short.ProductID, AvailableMilli: short.AvailableMilli, Unit: short.Unit}
	}
	return err
}
