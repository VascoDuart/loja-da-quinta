package password

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Parâmetros baixos só para os testes serem rápidos.
var fast = Params{Memory: 64, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}

func TestHashAndVerify(t *testing.T) {
	h := New(fast, 2)
	ctx := context.Background()

	encoded, err := h.Hash(ctx, "palavra-passe-certa")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=64,t=1,p=1$") {
		t.Errorf("formato inesperado: %s", encoded)
	}

	ok, err := h.Verify(ctx, "palavra-passe-certa", encoded)
	if err != nil || !ok {
		t.Errorf("a palavra-passe certa devia passar: ok=%v err=%v", ok, err)
	}
	ok, err = h.Verify(ctx, "palavra-passe-errada", encoded)
	if err != nil || ok {
		t.Errorf("a palavra-passe errada devia falhar: ok=%v err=%v", ok, err)
	}
}

func TestSaltIsRandom(t *testing.T) {
	h := New(fast, 1)
	a, _ := h.Hash(context.Background(), "igual")
	b, _ := h.Hash(context.Background(), "igual")
	if a == b {
		t.Error("dois hashes da mesma palavra-passe deviam ser diferentes")
	}
}

func TestOWASPParams(t *testing.T) {
	encoded, err := New(OWASP, 1).Hash(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded, "$m=19456,t=2,p=1$") {
		t.Errorf("os parâmetros por omissão deviam ser os da OWASP: %s", encoded)
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	h := New(fast, 1)
	for _, bad := range []string{
		"",
		"texto",
		"$2a$10$bcryptnaoserve",
		"$argon2i$v=19$m=64,t=1,p=1$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA",
		"$argon2id$v=19$m=999999999,t=1,p=1$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA", // memória absurda
		"$argon2id$v=19$m=64,t=1,p=1$***$aGFzaGhhc2hoYXNoaGFzaA",
	} {
		if _, err := h.Verify(context.Background(), "x", bad); !errors.Is(err, ErrMalformed) {
			t.Errorf("Verify(%q) devia dar ErrMalformed, deu %v", bad, err)
		}
	}
}

func TestConcurrencyLimitRespectsContext(t *testing.T) {
	h := New(fast, 1)
	h.slots <- struct{}{} // ocupa o único lugar
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := h.Hash(ctx, "x"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("devia desistir quando o prazo acaba, deu %v", err)
	}
}
