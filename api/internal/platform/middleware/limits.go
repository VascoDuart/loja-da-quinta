package middleware

import (
	"context"
	"net/http"
	"time"
)

// Limits são os limites de um pedido: tamanho do corpo e tempo.
type Limits struct {
	MaxBody int64
	Timeout time.Duration
}

// RouteLimits troca os limites por omissão nas rotas em que Match é
// verdadeiro (por exemplo, o upload de imagens: 5 MB e 30 s em vez de 1 MiB
// e 10 s). Fica a lista explícita, para os limites maiores nunca se
// aplicarem por engano a outras rotas.
type RouteLimits struct {
	Match  func(*http.Request) bool
	Limits Limits
}

func limitsFor(r *http.Request, def Limits, routes []RouteLimits) Limits {
	for _, rl := range routes {
		if rl.Match(r) {
			return rl.Limits
		}
	}
	return def
}

// MaxBody limita o tamanho do corpo dos pedidos. Ler para além do limite
// devolve um *http.MaxBytesError, que a API transforma em 413.
func MaxBody(n int64, routes ...RouteLimits) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limitsFor(r, Limits{MaxBody: n}, routes).MaxBody)
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout cancela o contexto do pedido ao fim de d. O cancelamento chega à
// base de dados através do contexto, e a query é interrompida.
func Timeout(d time.Duration, routes ...RouteLimits) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), limitsFor(r, Limits{Timeout: d}, routes).Timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
