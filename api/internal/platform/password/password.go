// Package password calcula e verifica hashes de palavras-passe com argon2id,
// no formato PHC: $argon2id$v=19$m=19456,t=2,p=1$<sal>$<hash>.
package password

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params são os parâmetros do argon2id.
type Params struct {
	Memory  uint32 // KiB
	Time    uint32
	Threads uint8
	SaltLen uint32
	KeyLen  uint32
}

// OWASP são os parâmetros mínimos recomendados pela OWASP: 19 MiB, 2
// iterações, 1 thread (Password Storage Cheat Sheet).
var OWASP = Params{Memory: 19 * 1024, Time: 2, Threads: 1, SaltLen: 16, KeyLen: 32}

// Limites ao ler um hash, para um valor estranho na base de dados não
// conseguir esgotar a memória ou o CPU da API.
const (
	maxMemory = 256 * 1024
	maxTime   = 10
)

// ErrMalformed indica um hash que não está no formato esperado.
var ErrMalformed = errors.New("hash de palavra-passe mal formado")

var b64 = base64.RawStdEncoding

// Hasher calcula hashes com um número máximo de cálculos em simultâneo: cada
// um usa 19 MiB, e o plano gratuito da Render tem 512 MiB.
type Hasher struct {
	params Params
	slots  chan struct{}
}

// New cria um Hasher. maxConcurrent é pelo menos 1.
func New(p Params, maxConcurrent int) *Hasher {
	return &Hasher{params: p, slots: make(chan struct{}, max(1, maxConcurrent))}
}

func (h *Hasher) acquire(ctx context.Context) error {
	select {
	case h.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hasher) release() { <-h.slots }

// Hash calcula o hash de uma palavra-passe com um sal aleatório.
func (h *Hasher) Hash(ctx context.Context, plain string) (string, error) {
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()

	salt := make([]byte, h.params.SaltLen)
	_, _ = rand.Read(salt) // crypto/rand.Read nunca falha (Go 1.24+)
	p := h.params
	key := argon2.IDKey([]byte(plain), salt, p.Time, p.Memory, p.Threads, p.KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Time, p.Threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify compara uma palavra-passe com um hash, em tempo constante. Usa os
// parâmetros guardados no hash, para os hashes antigos continuarem a
// funcionar se os parâmetros mudarem.
func (h *Hasher) Verify(ctx context.Context, plain, encoded string) (bool, error) {
	p, salt, want, err := decode(encoded)
	if err != nil {
		return false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, err
	}
	defer h.release()

	got := argon2.IDKey([]byte(plain), salt, p.Time, p.Memory, p.Threads, p.KeyLen)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func decode(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, ErrMalformed
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Params{}, nil, nil, ErrMalformed
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil {
		return Params{}, nil, nil, ErrMalformed
	}
	if p.Memory < 8 || p.Memory > maxMemory || p.Time < 1 || p.Time > maxTime || p.Threads < 1 {
		return Params{}, nil, nil, ErrMalformed
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return Params{}, nil, nil, ErrMalformed
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) < 16 || len(key) > 64 {
		return Params{}, nil, nil, ErrMalformed
	}
	p.KeyLen = uint32(len(key)) //nolint:gosec // G115: entre 16 e 64, verificado acima
	return p, salt, key, nil
}
