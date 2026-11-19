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
	h := NewHandler(fakeDB{err: errors.New("em baixo")}, discard)
	res, err := h.GetHealth(context.Background(), gen.GetHealthRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := res.(gen.GetHealth200JSONResponse); !ok {
		t.Errorf("o /healthz não deve depender da BD: %T", res)
	}
}

func TestReadiness(t *testing.T) {
	ready, _ := NewHandler(fakeDB{}, discard).GetReadiness(context.Background(), gen.GetReadinessRequestObject{})
	if _, ok := ready.(gen.GetReadiness200JSONResponse); !ok {
		t.Errorf("BD acessível devia dar 200: %T", ready)
	}

	down, _ := NewHandler(fakeDB{err: errors.New("ligação recusada")}, discard).GetReadiness(context.Background(), gen.GetReadinessRequestObject{})
	p, ok := down.(gen.GetReadiness503ApplicationProblemPlusJSONResponse)
	if !ok {
		t.Fatalf("BD em baixo devia dar 503: %T", down)
	}
	if *p.Detail == "ligação recusada" {
		t.Error("o erro interno não deve chegar ao cliente")
	}
}
