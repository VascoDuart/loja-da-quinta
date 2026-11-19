// Package httpx tem utilitários HTTP partilhados, como as respostas de erro
// no formato problem+json (RFC 9457).
package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/logging"
)

// Problem é o corpo de uma resposta de erro (RFC 9457).
// Nunca contém detalhes internos (stack traces, SQL, mensagens de bibliotecas).
type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// WriteProblem escreve um erro problem+json. O instance é o request ID, para
// o utilizador poder indicá-lo e encontrarmos o pedido nos logs.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, detail string) {
	p := Problem{
		Type:     "about:blank",
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   detail,
		Instance: logging.RequestID(r.Context()),
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}
