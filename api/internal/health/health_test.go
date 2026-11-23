package health

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestHealthIgnoresDatabase(t *testing.T) {
	h := NewHandler(fakeDB{err: errors.New("em baixo")}, discard, "abc123")
	res, err := h.GetHealth(context.Background(), gen.GetHealthRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	ok, isOK := res.(gen.GetHealth200JSONResponse)
	if !isOK {
		t.Fatalf("o /healthz não deve depender da BD: %T", res)
	}
	if ok.Version != "abc123" {
		t.Errorf("versão = %q", ok.Version)
	}
}

func TestReadiness(t *testing.T) {
	ready, _ := NewHandler(fakeDB{}, discard, "abc123").GetReadiness(context.Background(), gen.GetReadinessRequestObject{})
	if _, ok := ready.(gen.GetReadiness200JSONResponse); !ok {
		t.Errorf("BD acessível devia dar 200: %T", ready)
	}

	down, _ := NewHandler(fakeDB{err: errors.New("ligação recusada")}, discard, "abc123").GetReadiness(context.Background(), gen.GetReadinessRequestObject{})
	p, ok := down.(gen.GetReadiness503ApplicationProblemPlusJSONResponse)
	if !ok {
		t.Fatalf("BD em baixo devia dar 503: %T", down)
	}
	if *p.Detail == "ligação recusada" {
		t.Error("o erro interno não deve chegar ao cliente")
	}
}
