package httpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VascoDuart/loja-da-quinta/api/internal/health"
	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/config"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/httpserver"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func newTestServer(t *testing.T, dbErr error) *httptest.Server {
	t.Helper()
	cfg, err := config.FromLookup(func(k string) (string, bool) {
		if k == "DATABASE_URL" {
			return "postgres://app:x@localhost:5432/loja", true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := httpserver.NewHandler(cfg, log, httpapi.New(health.NewHandler(fakeDB{dbErr}, log, "test"), nil, nil, nil), httpserver.Security{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, url string) (*http.Response, map[string]any) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var body map[string]any
	_ = json.NewDecoder(res.Body).Decode(&body)
	return res, body
}

func TestHealthz(t *testing.T) {
	srv := newTestServer(t, nil)
	res, body := get(t, srv.URL+"/api/v1/healthz")
	if res.StatusCode != 200 || body["status"] != "ok" {
		t.Fatalf("status = %d, corpo = %v", res.StatusCode, body)
	}
	if res.Header.Get("X-Request-ID") == "" {
		t.Error("falta o X-Request-ID")
	}
	if res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("faltam os cabeçalhos de segurança")
	}
}

func TestReadyzWithDatabaseDown(t *testing.T) {
	srv := newTestServer(t, errors.New("ligação recusada"))
	res, body := get(t, srv.URL+"/api/v1/readyz")
	if res.StatusCode != 503 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if body["instance"] != res.Header.Get("X-Request-ID") {
		t.Errorf("o instance devia ser o request ID: %v", body)
	}
}

func TestUnknownPathReturnsProblem(t *testing.T) {
	srv := newTestServer(t, nil)
	res, body := get(t, srv.URL+"/api/v1/nao-existe")
	if res.StatusCode != 404 || res.Header.Get("Content-Type") != "application/problem+json" {
		t.Fatalf("status = %d, Content-Type = %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	if body["title"] != "Not Found" {
		t.Errorf("corpo = %v", body)
	}
}
