package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/logging"
)

func TestWriteProblem(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r = r.WithContext(logging.WithRequestID(r.Context(), "abc123"))
	w := httptest.NewRecorder()

	WriteProblem(w, r, http.StatusNotFound, "O recurso não existe.")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var p Problem
	if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	want := Problem{Type: "about:blank", Title: "Not Found", Status: 404, Detail: "O recurso não existe.", Instance: "abc123"}
	if p != want {
		t.Errorf("problem = %+v, queria %+v", p, want)
	}
}
