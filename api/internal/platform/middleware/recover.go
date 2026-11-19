package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/httpx"
)

// Recover transforma um panic num 500 problem+json. O stack trace vai só
// para os logs, nunca para o cliente.
func Recover(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(rec) // pedido abortado de propósito: o net/http trata disto
				}
				log.ErrorContext(r.Context(), "panic no handler",
					"panic", rec, "stack", string(debug.Stack()))
				httpx.WriteProblem(w, r, http.StatusInternalServerError, "Ocorreu um erro inesperado.")
			}()
			next.ServeHTTP(w, r)
		})
	}
}
