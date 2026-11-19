package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/httpx"
)

// RateLimit limita os pedidos por IP com um token bucket: rps pedidos por
// segundo, com rajadas até burst. Acima disso responde 429.
//
// Os limites ficam em memória: chega para uma única instância (ADR-0001).
// O limite por conta no login chega com a autenticação (#8).
func RateLimit(rps float64, burst, trustedHops int) Middleware {
	l := &ipLimiter{
		rps:     rate.Limit(rps),
		burst:   burst,
		clients: make(map[string]*client),
		now:     time.Now,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.allow(ClientIP(r, trustedHops)) {
				w.Header().Set("Retry-After", "1")
				httpx.WriteProblem(w, r, http.StatusTooManyRequests, "Demasiados pedidos. Tenta de novo daqui a pouco.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipLimiter struct {
	mu        sync.Mutex
	rps       rate.Limit
	burst     int
	clients   map[string]*client
	lastSweep time.Time
	now       func() time.Time
}

// Um IP sem pedidos há mais de idleTTL é esquecido, para o mapa não crescer
// sem limite.
const idleTTL = 10 * time.Minute

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Sub(l.lastSweep) > time.Minute {
		for k, c := range l.clients {
			if now.Sub(c.lastSeen) > idleTTL {
				delete(l.clients, k)
			}
		}
		l.lastSweep = now
	}

	c, ok := l.clients[ip]
	if !ok {
		c = &client{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.clients[ip] = c
	}
	c.lastSeen = now
	return c.limiter.AllowN(now, 1)
}
