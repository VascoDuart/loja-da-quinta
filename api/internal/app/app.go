// Package app compõe a API: cria os adaptadores, injeta-os nos módulos e
// devolve o handler HTTP. É o mesmo código no cmd/api e nos testes de
// integração, para os testes exercitarem exatamente o que vai para produção.
package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VascoDuart/loja-da-quinta/api/internal/auth"
	"github.com/VascoDuart/loja-da-quinta/api/internal/catalog"
	"github.com/VascoDuart/loja-da-quinta/api/internal/health"
	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/config"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/httpserver"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/password"
	"github.com/VascoDuart/loja-da-quinta/api/internal/users"
)

// Cálculos de argon2id em simultâneo: 4 × 19 MiB cabem folgados nos 512 MiB
// do plano gratuito da Render.
const maxConcurrentHashes = 4

// New devolve o handler HTTP completo da API.
func New(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool) (http.Handler, error) {
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
	catalogStore := catalog.NewPostgresStore(pool)

	api := httpapi.New(
		health.NewHandler(pool, log, cfg.Version),
		auth.NewEndpoints(authSvc),
		users.NewEndpoints(userSvc),
		catalog.NewEndpoints(catalog.NewService(catalogStore, catalogStore)),
	)
	return httpserver.NewHandler(cfg, log, api, httpserver.Security{
		Authenticate: authSvc.Authenticate,
		Authorize:    auth.Authorize,
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
