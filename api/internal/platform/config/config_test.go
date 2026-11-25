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
		_, err := FromLookup(lookupFrom(map[string]string{"APP_ENV": c.env, "DATABASE_URL": c.url, "JWT_SIGNING_KEY": "k:x", "JOBS_TOKEN": jobsToken}))
		if (err == nil) != c.ok {
			t.Errorf("%s %s: ok=%v, erro=%v", c.env, c.url, c.ok, err)
		}
	}
}

func TestJWTSigningKeyRequiredOutsideDevelopment(t *testing.T) {
	for env, ok := range map[string]bool{"development": true, "staging": false, "production": false} {
		_, err := FromLookup(lookupFrom(map[string]string{
			"APP_ENV": env, "DATABASE_URL": "postgres://app:x@neon.tech/loja?sslmode=require", "JOBS_TOKEN": jobsToken,
		}))
		if (err == nil) != ok {
			t.Errorf("%s sem JWT_SIGNING_KEY: ok=%v, erro=%v", env, ok, err)
		}
	}
}

var jobsToken = strings.Repeat("j", MinJobsTokenLen)

func TestJobsTokenRequiredOutsideDevelopment(t *testing.T) {
	for env, ok := range map[string]bool{"development": true, "staging": false, "production": false} {
		_, err := FromLookup(lookupFrom(map[string]string{
			"APP_ENV": env, "DATABASE_URL": "postgres://app:x@neon.tech/loja?sslmode=require", "JWT_SIGNING_KEY": "k:x",
		}))
		if (err == nil) != ok {
			t.Errorf("%s sem JOBS_TOKEN: ok=%v, erro=%v", env, ok, err)
		}
		if !ok && !strings.Contains(err.Error(), "JOBS_TOKEN") {
			t.Errorf("o erro devia falar do JOBS_TOKEN: %v", err)
		}
	}
}

func TestAlertSettings(t *testing.T) {
	base := func(extra map[string]string) (Config, error) {
		extra["DATABASE_URL"] = "postgres://app:x@localhost/loja"
		return FromLookup(lookupFrom(extra))
	}

	cfg, err := base(map[string]string{})
	if err != nil || cfg.JobsToken != "" || cfg.ResendAPIKey != "" || len(cfg.AlertsTo) != 0 ||
		cfg.AlertsFrom != "Loja da Quinta <onboarding@resend.dev>" {
		t.Errorf("por omissão: %+v %v", cfg, err)
	}

	cfg, err = base(map[string]string{
		"JOBS_TOKEN": jobsToken + "\r\n", "RESEND_API_KEY": " re_x\n", "ALERTS_TO": "a@quinta.pt, b@quinta.pt,",
	})
	if err != nil || cfg.JobsToken != jobsToken || cfg.ResendAPIKey != "re_x" || len(cfg.AlertsTo) != 2 || cfg.AlertsTo[1] != "b@quinta.pt" {
		t.Errorf("com valores: %+v %v", cfg, err)
	}

	for name, vars := range map[string]map[string]string{
		"token curto":        {"JOBS_TOKEN": jobsToken[1:]},
		"token com espaços":  {"JOBS_TOKEN": jobsToken + " " + jobsToken},
		"remetente inválido": {"ALERTS_FROM": "sem arroba"},
		"destinatário":       {"ALERTS_TO": "a@quinta.pt,isto-nao"},
		"nome no ALERTS_TO":  {"ALERTS_TO": "Ana <a@quinta.pt>"},
	} {
		_, err := base(vars)
		if err == nil {
			t.Errorf("%s: devia ser recusado", name)
			continue
		}
		if v := vars["JOBS_TOKEN"]; v != "" && strings.Contains(err.Error(), v) {
			t.Errorf("%s: o erro repete o token: %v", name, err)
		}
	}
}
