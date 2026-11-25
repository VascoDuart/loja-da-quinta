// Package app compõe a API: cria os adaptadores, injeta-os nos módulos e
// devolve o handler HTTP. É o mesmo código no cmd/api e nos testes de
// integração, para os testes exercitarem exatamente o que vai para produção.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VascoDuart/loja-da-quinta/api/internal/auth"
	"github.com/VascoDuart/loja-da-quinta/api/internal/catalog"
	"github.com/VascoDuart/loja-da-quinta/api/internal/health"
	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi"
	"github.com/VascoDuart/loja-da-quinta/api/internal/inventory"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/config"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/httpserver"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/password"
	"github.com/VascoDuart/loja-da-quinta/api/internal/users"
)

// Cálculos de argon2id em simultâneo: 4 × 19 MiB cabem folgados nos 512 MiB
// do plano gratuito da Render.
const maxConcurrentHashes = 4

// Option troca uma peça da composição. Só os testes de integração as usam.
type Option func(*options)

type options struct {
	now      func() time.Time
	notifier inventory.Notifier
}

// WithClock fixa o relógio do stock e dos alertas.
func WithClock(now func() time.Time) Option { return func(o *options) { o.now = now } }

// WithNotifier troca o envio dos alertas por outro (um fake).
func WithNotifier(n inventory.Notifier) Option { return func(o *options) { o.notifier = n } }

// New devolve o handler HTTP completo da API.
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
	// O users usa o store das sessões para as terminar (desativar, mudar o
	// papel); o auth usa o users para verificar credenciais.
	sessions := auth.NewPostgresStore(pool)
	userSvc, err := users.NewService(ctx, users.NewPostgresRepo(pool), password.New(password.OWASP, maxConcurrentHashes), sessions)
	if err != nil {
		return nil, err
	}
	authSvc := auth.NewService(userSvc, sessions, tokens, log)
	// O stock pergunta ao catálogo pelos produtos, e o catálogo pergunta ao
	// stock quanto há e se um produto tem lotes: cada um só conhece o serviço
	// do outro.
	catalogStore := catalog.NewPostgresStore(pool)
	catalogSvc := catalog.NewService(catalogStore, catalogStore)
	inventoryStore := inventory.NewPostgresStore(pool)
	inventorySvc := inventory.NewService(inventoryStore, catalogProducts{catalogSvc}, o.now)
	catalogSvc.WithStock(inventorySvc)
	alerts := inventory.NewAlerts(inventorySvc, inventoryStore, newRecipients(cfg, userSvc), o.notifier, log)

	api := httpapi.New(
		health.NewHandler(pool, log, cfg.Version),
		auth.NewEndpoints(authSvc),
		users.NewEndpoints(userSvc),
		catalog.NewEndpoints(catalogSvc),
		inventory.NewEndpoints(inventorySvc, alerts),
	)
	return httpserver.NewHandler(cfg, log, api, httpserver.Security{
		Authenticate: authSvc.Authenticate,
		Authorize:    auth.Authorizer(cfg.JobsToken),
	})
}

// newTokens lê a chave dos access tokens. Localmente, sem chave, usa uma
// temporária; a configuração já recusa arrancar sem ela nos outros
// ambientes.
func newTokens(cfg config.Config, log *slog.Logger) (*auth.JWT, error) {
	if cfg.JWTSigningKey == "" {
		log.Warn("sem JWT_SIGNING_KEY: a usar uma chave temporária (só em desenvolvimento)")
		return auth.NewEphemeralJWT(), nil
	}
	return auth.NewJWT(cfg.JWTSigningKey, cfg.JWTVerifyKeys)
}

// catalogProducts dá ao stock o que ele precisa de um produto, a partir do
// serviço do catálogo (o inventory nunca lê a tabela products).
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
