package middleware

import (
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/httpx"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/ratelimit"
)

// RateLimit limita os pedidos por IP com um token bucket: rps pedidos por
// segundo, com rajadas até burst. Acima disso responde 429.
//
// O IP vem do contexto (ClientIPContext). Os limites próprios do login, por
// IP e por conta, estão no módulo auth.
func RateLimit(rps float64, burst int) Middleware {
	l := ratelimit.New(rate.Limit(rps), burst, 10*time.Minute)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow(httpx.ClientIP(r.Context())) {
				w.Header().Set("Retry-After", "1")
				httpx.WriteProblem(w, r, http.StatusTooManyRequests, "Demasiados pedidos. Tenta de novo daqui a pouco.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
