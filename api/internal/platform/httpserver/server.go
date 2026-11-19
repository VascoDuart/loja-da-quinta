// Package httpserver monta o router, a cadeia de middleware e o servidor
// HTTP, e trata do encerramento ordenado.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/config"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/httpx"
	mw "github.com/VascoDuart/loja-da-quinta/api/internal/platform/middleware"
)

// BasePath é o prefixo de todos os endpoints (servers.url na OpenAPI).
const BasePath = "/api/v1"

// NewHandler devolve o handler HTTP completo: middleware, validação contra a
// OpenAPI e as rotas geradas.
func NewHandler(cfg config.Config, log *slog.Logger, api gen.StrictServerInterface) (http.Handler, error) {
	spec, err := gen.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("ler a spec OpenAPI: %w", err)
	}

	// Os pedidos que não cumprem a spec (parâmetros, corpo, tipos) são
	// recusados com 400 antes de chegarem aos handlers.
	validate := nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
		SilenceServersWarning: true,
		ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, r *http.Request, opts nethttpmiddleware.ErrorHandlerOpts) {
			status := opts.StatusCode
			if status == 0 {
				status = http.StatusBadRequest
			}
			log.InfoContext(r.Context(), "pedido recusado pela validação OpenAPI", "err", err)
			httpx.WriteProblem(w, r, status, "O pedido não cumpre o contrato da API.")
		},
	})

	strict := gen.NewStrictHandlerWithOptions(api, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "O pedido é inválido.")
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			log.ErrorContext(r.Context(), "erro no handler", "err", err)
			httpx.WriteProblem(w, r, http.StatusInternalServerError, "Ocorreu um erro inesperado.")
		},
	})

	mux := http.NewServeMux()
	gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{
		BaseURL:     BasePath,
		BaseRouter:  mux,
		Middlewares: []gen.MiddlewareFunc{validate},
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "Parâmetros inválidos.")
		},
	})
	// Qualquer caminho sem rota recebe um 404 problem+json em vez de texto.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteProblem(w, r, http.StatusNotFound, "Este endereço não existe.")
	})

	return mw.Chain(mux,
		mw.Recover(log),
		mw.RequestID(),
		mw.AccessLog(log),
		mw.SecurityHeaders(cfg.Env != config.EnvDevelopment),
		mw.RateLimit(cfg.RateLimitRPS, cfg.RateLimitBurst, cfg.TrustedProxyHops),
		mw.MaxBody(cfg.MaxBodyBytes),
		mw.Timeout(cfg.RequestTimeout),
	), nil
}

// Run arranca o servidor e bloqueia até o contexto ser cancelado (SIGTERM ou
// SIGINT). Depois espera até ShutdownTimeout que os pedidos em curso acabem.
func Run(ctx context.Context, cfg config.Config, log *slog.Logger, h http.Handler) error {
	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second, // ADR-0007: abaixo do limite do proxy da Vercel
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("API a escutar", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("a encerrar: à espera dos pedidos em curso", "timeout", cfg.ShutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("encerrar o servidor: %w", err)
	}
	return <-errCh
}
