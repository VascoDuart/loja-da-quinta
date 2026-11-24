// Comando api: arranca a API da Loja da Quinta.
//
// Aqui só se lê a configuração, se abre a base de dados e se arranca o
// servidor. A composição dos módulos está em internal/app, partilhada com os
// testes de integração. Não há framework de injeção de dependências.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/VascoDuart/loja-da-quinta/api/internal/app"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/config"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/db"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/httpserver"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/logging"
)

func main() {
	if err := run(); err != nil {
		slog.Error("a API terminou com erro", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(os.Stdout, cfg.LogLevel, cfg.Env != config.EnvDevelopment)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConn)
	if err != nil {
		return err
	}
	defer pool.Close()

	handler, err := app.New(ctx, cfg, log, pool)
	if err != nil {
		return err
	}
	return httpserver.Run(ctx, cfg, log, handler)
}
