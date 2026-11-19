// Package db cria o pool de ligações ao PostgreSQL com pgx.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool cria o pool sem abrir ligações logo. Assim a API arranca mesmo com
// a base de dados em baixo: o /healthz responde e o /readyz mostra o problema.
func NewPool(ctx context.Context, url string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		// Não se inclui o erro original: pode conter a palavra-passe do URL.
		return nil, fmt.Errorf("DATABASE_URL inválido")
	}
	// O plano gratuito da Neon tem poucas ligações; 5 chegam para uma loja.
	cfg.MaxConns = maxConns
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	cfg.ConnConfig.RuntimeParams["application_name"] = "loja-da-quinta-api"

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("criar o pool: %w", err)
	}
	return pool, nil
}
