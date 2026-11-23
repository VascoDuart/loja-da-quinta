//go:build integration

// Testes de integração: arrancam um PostgreSQL real num contentor
// (testcontainers), aplicam as migrações e testam a API completa.
//
//	go test -tags=integration ./test/...
//
// Precisam de Docker. Sem a tag, `go test ./...` não os corre.
package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/VascoDuart/loja-da-quinta/api/internal/health"
	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/config"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/db"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/httpserver"
	"github.com/VascoDuart/loja-da-quinta/api/migrations"
)

func startPostgres(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("loja"),
		tcpostgres.WithUsername("teste"),
		tcpostgres.WithPassword("teste"),
		tcpostgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("arrancar o PostgreSQL: %v", err)
	}
	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	return url
}

func TestMigrationsApplyAndAreIdempotent(t *testing.T) {
	url := startPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sqlDB.Close() }()

	for i := range 2 {
		if err := migrations.Up(ctx, sqlDB); err != nil {
			t.Fatalf("execução %d: %v", i+1, err)
		}
	}

	var exists bool
	if err := sqlDB.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_proc WHERE proname = 'set_updated_at')").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("a função set_updated_at devia existir")
	}
}

func TestAPIAgainstRealDatabase(t *testing.T) {
	url := startPostgres(t)
	ctx := context.Background()

	cfg, err := config.FromLookup(func(k string) (string, bool) {
		if k == "DATABASE_URL" {
			return url, true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.NewPool(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := httpserver.NewHandler(cfg, log, &httpapi.Server{Handler: health.NewHandler(pool, log, cfg.Version)})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	for _, path := range []string{"/api/v1/healthz", "/api/v1/readyz"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]string
		_ = json.NewDecoder(res.Body).Decode(&body)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK || body["status"] != "ok" {
			t.Errorf("%s: status %d, corpo %v", path, res.StatusCode, body)
		}
	}
}
