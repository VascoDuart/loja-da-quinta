// Package config lê a configuração da API a partir de variáveis de ambiente
// e valida-a no arranque. A API recusa arrancar com configuração inválida.
package config

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
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

	// Version é o commit em execução. A Render define RENDER_GIT_COMMIT em
	// cada deploy; APP_VERSION permite fixá-la noutros ambientes.
	Version string

	DatabaseURL     string
	DatabaseMaxConn int32

	// TrustedProxyHops é o número de proxies de confiança à frente da API
	// (Vercel + Render = 2). Serve para descobrir o IP real do cliente no
	// cabeçalho X-Forwarded-For sem aceitar valores forjados.
	TrustedProxyHops int

	// RateLimitRPS e RateLimitBurst limitam os pedidos por IP. A rajada tem
	// de chegar para abrir uma página inteira (sessão, listas e uma miniatura
	// por produto), mesmo com vários dispositivos da loja atrás do mesmo IP.
	RateLimitRPS   float64
	RateLimitBurst int

	// JWTSigningKey assina os access tokens ("kid:seed", gerada com
	// go run ./cmd/jwtkey). Obrigatória fora do ambiente local; localmente,
	// sem ela, a API usa uma chave temporária.
	JWTSigningKey string
	// JWTVerifyKeys são chaves públicas antigas que ainda verificam tokens
	// depois de uma troca de chave ("kid:pub,kid:pub").
	JWTVerifyKeys string

	// JobsToken abre os endpoints dos jobs agendados (cabeçalho X-Job-Token,
	// ADR-0015). Obrigatório fora do ambiente local; localmente, sem ele, os
	// jobs recusam todos os pedidos.
	JobsToken string
	// ResendAPIKey envia os alertas pelo Resend. Sem ela, vão para o log.
	ResendAPIKey string
	// AlertsFrom é o remetente dos alertas. Sem domínio próprio no Resend, só
	// o endereço de testes onboarding@resend.dev.
	AlertsFrom string
	// AlertsTo, se tiver endereços, substitui os gestores como destinatários.
	AlertsTo []string

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
		Version:          r.str("APP_VERSION", r.str("RENDER_GIT_COMMIT", "dev")),
		DatabaseURL:      r.required("DATABASE_URL"),
		DatabaseMaxConn:  r.int32("DATABASE_MAX_CONNS", 5),
		TrustedProxyHops: r.int("TRUSTED_PROXY_HOPS", 0),
		RateLimitRPS:     r.float("RATE_LIMIT_RPS", 20),
		RateLimitBurst:   r.int("RATE_LIMIT_BURST", 100),
		JWTSigningKey:    r.str("JWT_SIGNING_KEY", ""),
		JWTVerifyKeys:    r.str("JWT_VERIFY_KEYS", ""),
		// Colados num painel ou lidos de um ficheiro, os segredos trazem às
		// vezes uma mudança de linha.
		JobsToken:       strings.TrimSpace(r.str("JOBS_TOKEN", "")),
		ResendAPIKey:    strings.TrimSpace(r.str("RESEND_API_KEY", "")),
		AlertsFrom:      r.str("ALERTS_FROM", "Loja da Quinta <onboarding@resend.dev>"),
		AlertsTo:        r.list("ALERTS_TO"),
		MaxBodyBytes:    int64(r.int("MAX_BODY_BYTES", 1<<20)),
		RequestTimeout:  r.duration("REQUEST_TIMEOUT", 10*time.Second),
		ShutdownTimeout: r.duration("SHUTDOWN_TIMEOUT", 15*time.Second),
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
	errs = append(errs, c.validateJobs()...)
	if c.DatabaseURL != "" {
		errs = append(errs, c.validateDatabaseURL()...)
	}
	if c.Env != EnvDevelopment && c.JWTSigningKey == "" {
		errs = append(errs, fmt.Errorf("JWT_SIGNING_KEY é obrigatória em %s", c.Env))
	}
	return errs
}

// MinJobsTokenLen é o tamanho mínimo do token dos jobs
// (openssl rand -hex 32 dá 64).
const MinJobsTokenLen = 32

// As mensagens nunca repetem o valor do token.
func (c Config) validateJobs() []error {
	var errs []error
	switch {
	case c.JobsToken == "" && c.Env != EnvDevelopment:
		errs = append(errs, fmt.Errorf("JOBS_TOKEN é obrigatória em %s", c.Env))
	case c.JobsToken != "" && (len(c.JobsToken) < MinJobsTokenLen ||
		strings.IndexFunc(c.JobsToken, func(r rune) bool { return r <= ' ' || r > '~' }) >= 0):
		errs = append(errs, fmt.Errorf("JOBS_TOKEN tem de ter pelo menos %d caracteres ASCII, sem espaços", MinJobsTokenLen))
	}
	if _, err := mail.ParseAddress(c.AlertsFrom); err != nil {
		errs = append(errs, errors.New("ALERTS_FROM tem de ser um endereço de e-mail"))
	}
	for _, to := range c.AlertsTo {
		if a, err := mail.ParseAddress(to); err != nil || a.Address != to {
			errs = append(errs, errors.New("ALERTS_TO tem de ser uma lista de endereços separados por vírgulas"))
			break
		}
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

// list lê uma lista separada por vírgulas; vazia sem a variável.
func (r *reader) list(key string) []string {
	var out []string
	for _, item := range strings.Split(r.str(key, ""), ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
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
