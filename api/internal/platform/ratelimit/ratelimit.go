// Package ratelimit tem um limitador token bucket por chave (IP, e-mail…).
//
// Os limites ficam em memória: chega para uma única instância (ADR-0001).
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Keyed guarda um token bucket por chave. As chaves sem uso há mais de ttl
// são esquecidas, para o mapa não crescer sem limite.
type Keyed struct {
	mu        sync.Mutex
	limit     rate.Limit
	burst     int
	ttl       time.Duration
	entries   map[string]*entry
	lastSweep time.Time
	now       func() time.Time
}

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// New cria um limitador: limit eventos por segundo, rajadas até burst.
func New(limit rate.Limit, burst int, ttl time.Duration) *Keyed {
	return &Keyed{limit: limit, burst: burst, ttl: ttl, entries: make(map[string]*entry), now: time.Now}
}

// Allow gasta um token da chave e diz se havia algum.
func (k *Keyed) Allow(key string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	now := k.now()
	return k.get(key, now).limiter.AllowN(now, 1)
}

// Exhausted diz se a chave já não tem tokens, sem gastar nenhum.
func (k *Keyed) Exhausted(key string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	e, ok := k.entries[key]
	if !ok {
		return false
	}
	return e.limiter.TokensAt(k.now()) < 1
}

// Reset esquece a chave (por exemplo, depois de um login certo).
func (k *Keyed) Reset(key string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.entries, key)
}

func (k *Keyed) get(key string, now time.Time) *entry {
	if now.Sub(k.lastSweep) > time.Minute {
		for key, e := range k.entries {
			if now.Sub(e.lastSeen) > k.ttl {
				delete(k.entries, key)
			}
		}
		k.lastSweep = now
	}
	e, ok := k.entries[key]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(k.limit, k.burst)}
		k.entries[key] = e
	}
	e.lastSeen = now
	return e
}
