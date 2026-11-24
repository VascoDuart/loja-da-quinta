package ratelimit

import (
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestAllowPerKey(t *testing.T) {
	k := New(rate.Every(time.Hour), 2, time.Hour)
	first, second := k.Allow("a"), k.Allow("a")
	if !first || !second {
		t.Fatal("as duas primeiras deviam passar")
	}
	if k.Allow("a") {
		t.Error("a terceira devia ser recusada")
	}
	if !k.Allow("b") {
		t.Error("outra chave não devia ser afetada")
	}
}

func TestExhaustedDoesNotSpend(t *testing.T) {
	k := New(rate.Every(time.Hour), 1, time.Hour)
	if k.Exhausted("a") {
		t.Error("uma chave nova não está esgotada")
	}
	k.Allow("a")
	if !k.Exhausted("a") {
		t.Error("depois de gastar o único token devia estar esgotada")
	}
	k.Reset("a")
	if k.Exhausted("a") || !k.Allow("a") {
		t.Error("depois do reset devia voltar a ter tokens")
	}
}

func TestRefillsOverTime(t *testing.T) {
	now := time.Now()
	k := New(rate.Every(time.Minute), 1, time.Hour)
	k.now = func() time.Time { return now }
	k.Allow("a")
	if k.Allow("a") {
		t.Fatal("devia estar esgotada")
	}
	now = now.Add(time.Minute)
	if !k.Allow("a") {
		t.Error("ao fim de um minuto devia ter um token novo")
	}
}

func TestForgetsIdleKeys(t *testing.T) {
	now := time.Now()
	k := New(rate.Limit(1), 1, 10*time.Minute)
	k.now = func() time.Time { return now }
	k.Allow("1.1.1.1")
	now = now.Add(12 * time.Minute)
	k.Allow("2.2.2.2")
	if _, ok := k.entries["1.1.1.1"]; ok {
		t.Error("a chave inativa devia ter sido esquecida")
	}
}
