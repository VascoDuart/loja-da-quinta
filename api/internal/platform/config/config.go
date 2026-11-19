// Package config lê a configuração da API a partir de variáveis de ambiente
// e valida-a no arranque. A API recusa arrancar com configuração inválida.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

// Env identifica o ambiente onde a API corre.
type Env string

const (
	EnvDevelopment Env = "development"
	EnvStaging     Env = "staging"
	EnvProduction  Env = "production"
)

// Config é a configuração completa da API.
type Config struct {
	Env      Env
	Port     int
	LogLevel string

	DatabaseURL     string
	DatabaseMaxConn int32

	// TrustedProxyHops é o número de proxies de confiança à frente da API
	// (Vercel + Render = 2). Serve para descobrir o IP real do cliente no
	// cabeçalho X-Forwarded-For sem aceitar valores forjados.
	TrustedProxyHops int

	RateLimitRPS   float64
	RateLimitBurst int

	MaxBodyBytes    int64
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
}

// Load lê as variáveis de ambiente do processo.
func Load() (Config, error) {
	return FromLookup(os.LookupEnv)
}

// FromLookup lê a configuração através de uma função de lookup, para os
// testes não dependerem do ambiente real.
func FromLookup(lookup func(string) (string, bool)) (Config, error) {
	r := reader{lookup: lookup}

	cfg := Config{
		Env:              Env(r.str("APP_ENV", string(EnvDevelopment))),
		Port:             r.int("PORT", 8080),
		LogLevel:         r.str("LOG_LEVEL", "info"),
		DatabaseURL:      r.required("DATABASE_URL"),
		DatabaseMaxConn:  r.int32("DATABASE_MAX_CONNS", 5),
		TrustedProxyHops: r.int("TRUSTED_PROXY_HOPS", 0),
		RateLimitRPS:     r.float("RATE_LIMIT_RPS", 10),
		RateLimitBurst:   r.int("RATE_LIMIT_BURST", 20),
		MaxBodyBytes:     int64(r.int("MAX_BODY_BYTES", 1<<20)),
		RequestTimeout:   r.duration("REQUEST_TIMEOUT", 10*time.Second),
		ShutdownTimeout:  r.duration("SHUTDOWN_TIMEOUT", 15*time.Second),
	}

	errs := r.errs
	errs = append(errs, cfg.validate()...)
	if len(errs) > 0 {
		return Config{}, fmt.Errorf("configuração inválida: %w", errors.Join(errs...))
	}
	return cfg, nil
}

func (c Config) validate() []error {
	var errs []error
	switch c.Env {
	case EnvDevelopment, EnvStaging, EnvProduction:
	default:
		errs = append(errs, fmt.Errorf("APP_ENV tem de ser development, staging ou production (recebido %q)", c.Env))
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("LOG_LEVEL tem de ser debug, info, warn ou error (recebido %q)", c.LogLevel))
	}
	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("PORT fora do intervalo 1-65535: %d", c.Port))
	}
	if c.DatabaseMaxConn < 1 {
		errs = append(errs, errors.New("DATABASE_MAX_CONNS tem de ser pelo menos 1"))
	}
	if c.TrustedProxyHops < 0 {
		errs = append(errs, errors.New("TRUSTED_PROXY_HOPS não pode ser negativo"))
	}
	if c.RateLimitRPS <= 0 || c.RateLimitBurst < 1 {
		errs = append(errs, errors.New("RATE_LIMIT_RPS e RATE_LIMIT_BURST têm de ser positivos"))
	}
	if c.MaxBodyBytes < 1 {
		errs = append(errs, errors.New("MAX_BODY_BYTES tem de ser positivo"))
	}
	if c.RequestTimeout <= 0 || c.ShutdownTimeout <= 0 {
		errs = append(errs, errors.New("REQUEST_TIMEOUT e SHUTDOWN_TIMEOUT têm de ser positivos"))
	}
	if c.DatabaseURL != "" {
		errs = append(errs, c.validateDatabaseURL()...)
	}
	return errs
}

// Fora do ambiente local, a ligação à base de dados tem de usar TLS.
func (c Config) validateDatabaseURL() []error {
	u, err := url.Parse(c.DatabaseURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return []error{errors.New("DATABASE_URL tem de ser um URL postgres:// válido")}
	}
	if c.Env == EnvDevelopment {
		return nil
	}
	switch u.Query().Get("sslmode") {
	case "require", "verify-ca", "verify-full":
		return nil
	default:
		return []error{fmt.Errorf("DATABASE_URL tem de usar sslmode=require ou mais forte em %s", c.Env)}
	}
}

// IsProduction indica se a API corre em produção.
func (c Config) IsProduction() bool { return c.Env == EnvProduction }

// Addr devolve o endereço onde o servidor HTTP escuta.
func (c Config) Addr() string { return ":" + strconv.Itoa(c.Port) }

type reader struct {
	lookup func(string) (string, bool)
	errs   []error
}

func (r *reader) str(key, def string) string {
	if v, ok := r.lookup(key); ok && v != "" {
		return v
	}
	return def
}

func (r *reader) required(key string) string {
	v, ok := r.lookup(key)
	if !ok || v == "" {
		r.errs = append(r.errs, fmt.Errorf("%s é obrigatória", key))
	}
	return v
}

func (r *reader) int(key string, def int) int {
	v, ok := r.lookup(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s tem de ser um número inteiro (recebido %q)", key, v))
		return def
	}
	return n
}

func (r *reader) int32(key string, def int32) int32 {
	v, ok := r.lookup(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s tem de ser um número inteiro de 32 bits (recebido %q)", key, v))
		return def
	}
	return int32(n)
}

func (r *reader) float(key string, def float64) float64 {
	v, ok := r.lookup(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s tem de ser um número (recebido %q)", key, v))
		return def
	}
	return n
}

func (r *reader) duration(key string, def time.Duration) time.Duration {
	v, ok := r.lookup(key)
	if !ok || v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s tem de ser uma duração como 10s ou 1m (recebido %q)", key, v))
		return def
	}
	return d
}
