//go:build integration

package integration

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VascoDuart/loja-da-quinta/api/internal/app"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/config"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/db"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/password"
	"github.com/VascoDuart/loja-da-quinta/api/internal/users"
	"github.com/VascoDuart/loja-da-quinta/api/migrations"
)

// env é um PostgreSQL real com as migrações aplicadas.
type env struct {
	cfg  config.Config
	pool *pgxpool.Pool
}

func newEnv(t *testing.T) *env {
	t.Helper()
	url := startPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sqlDB.Close() }()
	if err := migrations.Up(ctx, sqlDB); err != nil {
		t.Fatal(err)
	}

	// Todos os pedidos dos testes vêm de 127.0.0.1, seguidos: o limite
	// global por IP (testado nos unitários do middleware) subiria logo. Os
	// limites do login, por conta e por IP, ficam os reais.
	vars := map[string]string{"DATABASE_URL": url, "RATE_LIMIT_RPS": "1000", "RATE_LIMIT_BURST": "1000"}
	cfg, err := config.FromLookup(func(k string) (string, bool) {
		v, ok := vars[k]
		return v, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.NewPool(context.Background(), cfg.DatabaseURL, cfg.DatabaseMaxConn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return &env{cfg: cfg, pool: pool}
}

// server arranca a API completa, montada pelo mesmo código do cmd/api. Cada
// chamada dá uma instância nova, com os limites de login a zero.
func (e *env) server(t *testing.T, opts ...app.Option) *httptest.Server {
	t.Helper()
	h, err := app.New(context.Background(), e.cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), e.pool, opts...)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// createUser cria um utilizador com o serviço real (argon2id incluído).
func (e *env) createUser(t *testing.T, email string, role identity.Role, pw string) users.User {
	t.Helper()
	svc, err := users.NewService(context.Background(), users.NewPostgresRepo(e.pool), password.New(password.OWASP, 2), nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := svc.Create(context.Background(), users.NewUser{Email: email, Name: "Teste " + string(role), Role: role, Password: pw})
	if err != nil {
		t.Fatal(err)
	}
	return u
}
