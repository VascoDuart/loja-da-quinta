// Comando migrate: aplica as migrações à base de dados.
//
// Corre com o papel migrator (MIGRATIONS_DATABASE_URL), nunca com o papel app
// da API: a API não tem permissões de DDL (docs/architecture/data-model.md).
package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/VascoDuart/loja-da-quinta/api/migrations"
)

func main() {
	url := os.Getenv("MIGRATIONS_DATABASE_URL")
	if url == "" {
		slog.Error("MIGRATIONS_DATABASE_URL é obrigatória")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, err := sql.Open("pgx", url)
	if err != nil {
		slog.Error("abrir a ligação", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	if err := migrations.Up(ctx, db); err != nil {
		slog.Error("migrações falharam", "err", err)
		os.Exit(1)
	}
	slog.Info("migrações aplicadas")
}
