package middleware

import (
	"context"
	"net/http"
	"time"
)

// MaxBody limita o tamanho do corpo dos pedidos. Ler para além do limite
// devolve um erro, que os handlers transformam em 413.
func MaxBody(n int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout cancela o contexto do pedido ao fim de d. O cancelamento chega à
// base de dados através do contexto, e a query é interrompida.
func Timeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
