// Package migrations embebe as migrações SQL (goose) no binário, para o
// comando migrate e os testes de integração usarem exatamente os mesmos
// ficheiros.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var files embed.FS

// Up aplica as migrações em falta.
func Up(ctx context.Context, db *sql.DB) error {
	p, err := goose.NewProvider(goose.DialectPostgres, db, files)
	if err != nil {
		return fmt.Errorf("preparar as migrações: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("aplicar as migrações: %w", err)
	}
	return nil
}
