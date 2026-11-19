package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/logging"
)

// HeaderRequestID é o cabeçalho com o identificador do pedido.
const HeaderRequestID = "X-Request-ID"

// Só se aceita um request ID vindo do cliente se for curto e só tiver
// caracteres seguros, para não permitir injeção de texto nos logs.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9-]{8,64}$`)

// RequestID garante que cada pedido tem um ID, devolve-o na resposta e
// guarda-o no contexto para os logs e os erros problem+json.
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(HeaderRequestID)
			if !validRequestID.MatchString(id) {
				id = newID()
			}
			w.Header().Set(HeaderRequestID, id)
			next.ServeHTTP(w, r.WithContext(logging.WithRequestID(r.Context(), id)))
		})
	}
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand.Read nunca falha (Go 1.24+)
	return hex.EncodeToString(b)
}
