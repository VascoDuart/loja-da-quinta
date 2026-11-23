package config

import (
	"strings"
	"testing"
	"time"
)

func lookupFrom(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestDefaults(t *testing.T) {
	cfg, err := FromLookup(lookupFrom(map[string]string{
		"DATABASE_URL": "postgres://app:x@localhost:5432/loja?sslmode=disable",
	}))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if cfg.Env != EnvDevelopment || cfg.Port != 8080 || cfg.LogLevel != "info" {
		t.Errorf("valores por omissão errados: %+v", cfg)
	}
	if cfg.RequestTimeout != 10*time.Second || cfg.MaxBodyBytes != 1<<20 {
		t.Errorf("limites por omissão errados: %+v", cfg)
	}
	if cfg.Addr() != ":8080" {
		t.Errorf("Addr = %q", cfg.Addr())
	}
}

func TestVersion(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{}, "dev"},
		{map[string]string{"RENDER_GIT_COMMIT": "abc123"}, "abc123"},
		{map[string]string{"RENDER_GIT_COMMIT": "abc123", "APP_VERSION": "fixa"}, "fixa"},
	}
	for _, c := range cases {
		c.env["DATABASE_URL"] = "postgres://app:x@localhost/loja"
		cfg, err := FromLookup(lookupFrom(c.env))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Version != c.want {
			t.Errorf("Version = %q, queria %q", cfg.Version, c.want)
		}
	}
}

func TestInvalidConfigListsEveryProblem(t *testing.T) {
	_, err := FromLookup(lookupFrom(map[string]string{
		"APP_ENV":         "prod",
		"PORT":            "abc",
		"LOG_LEVEL":       "verbose",
		"REQUEST_TIMEOUT": "dez segundos",
	}))
	if err == nil {
		t.Fatal("esperava erro")
	}
	for _, want := range []string{"DATABASE_URL é obrigatória", "APP_ENV", "PORT", "LOG_LEVEL", "REQUEST_TIMEOUT"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("o erro devia mencionar %q: %v", want, err)
		}
	}
}

func TestTLSRequiredOutsideDevelopment(t *testing.T) {
	cases := []struct {
		env, url string
		ok       bool
	}{
		{"development", "postgres://app:x@localhost/loja?sslmode=disable", true},
		{"staging", "postgres://app:x@neon.tech/loja?sslmode=disable", false},
		{"production", "postgres://app:x@neon.tech/loja", false},
		{"production", "postgres://app:x@neon.tech/loja?sslmode=require", true},
		{"production", "mysql://app:x@neon.tech/loja?sslmode=require", false},
	}
	for _, c := range cases {
		_, err := FromLookup(lookupFrom(map[string]string{"APP_ENV": c.env, "DATABASE_URL": c.url}))
		if (err == nil) != c.ok {
			t.Errorf("%s %s: ok=%v, erro=%v", c.env, c.url, c.ok, err)
		}
	}
}
