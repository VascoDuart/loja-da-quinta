// Package httpserver monta o router, a cadeia de middleware e o servidor
// HTTP, e trata do encerramento ordenado.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/config"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/httpx"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
	mw "github.com/VascoDuart/loja-da-quinta/api/internal/platform/middleware"
)

// BasePath é o prefixo de todos os endpoints (servers.url na OpenAPI).
const BasePath = "/api/v1"

// Security liga a autenticação ao servidor sem a plataforma importar o
// módulo auth.
type Security struct {
	// Authenticate lê o access token e põe o principal no contexto.
	Authenticate mw.Middleware
	// Authorize decide, para cada operação com bearerAuth, se o principal
	// tem um dos papéis da spec. Sem ela, todas essas operações dão 401.
	Authorize openapi3filter.AuthenticationFunc
}

// Upload de imagens dos produtos (ADR-0014): até 5 MB, e uma foto grande
// demora mais a processar do que um pedido normal.
var imageUpload = regexp.MustCompile(`^` + BasePath + `/products/[^/]+/image$`)

var uploadLimits = mw.RouteLimits{
	Match:  func(r *http.Request) bool { return r.Method == http.MethodPut && imageUpload.MatchString(r.URL.Path) },
	Limits: mw.Limits{MaxBody: 5<<20 + 64<<10, Timeout: 30 * time.Second},
}

// NewHandler devolve o handler HTTP completo: middleware, validação contra a
// OpenAPI (incluindo os papéis de cada operação) e as rotas geradas.
func NewHandler(cfg config.Config, log *slog.Logger, api gen.StrictServerInterface, sec Security) (http.Handler, error) {
	spec, err := gen.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("ler a spec OpenAPI: %w", err)
	}

	// Os pedidos que não cumprem a spec (parâmetros, corpo, tipos) são
	// recusados com 400 antes de chegarem aos handlers.
	// A segurança é verificada antes dos parâmetros e do corpo: um pedido sem
	// sessão recebe 401 sem revelar o formato que o endpoint espera.
	validate := nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
		SilenceServersWarning: true,
		Options:               openapi3filter.Options{AuthenticationFunc: sec.Authorize},
		ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, r *http.Request, opts nethttpmiddleware.ErrorHandlerOpts) {
			// Antes da segurança: o validador lê o corpo antes de verificar a
			// sessão, e um corpo grande demais chegaria aqui como falha de
			// segurança (401) em vez de 413.
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				httpx.WriteProblem(w, r, http.StatusRequestEntityTooLarge, "O pedido é demasiado grande.")
				return
			}
			if writeSecurityError(w, r, err) {
				return
			}
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

	authenticate := sec.Authenticate
	if authenticate == nil {
		authenticate = func(next http.Handler) http.Handler { return next }
	}

	return mw.Chain(mux,
		mw.Recover(log),
		mw.RequestID(),
		mw.AccessLog(log),
		mw.SecurityHeaders(cfg.Env != config.EnvDevelopment),
		mw.ClientIPContext(cfg.TrustedProxyHops),
		mw.RateLimit(cfg.RateLimitRPS, cfg.RateLimitBurst),
		mw.MaxBody(cfg.MaxBodyBytes, uploadLimits),
		mw.Timeout(cfg.RequestTimeout, uploadLimits),
		authenticate,
	), nil
}

// writeSecurityError traduz uma falha de segurança da validação em 401 ou
// 403. Devolve false se o erro não for de segurança.
func writeSecurityError(w http.ResponseWriter, r *http.Request, err error) bool {
	var secErr *openapi3filter.SecurityRequirementsError
	if !errors.As(err, &secErr) {
		return false
	}
	switch {
	case errors.Is(err, identity.ErrForbidden):
		httpx.WriteProblem(w, r, http.StatusForbidden, "Não tens permissão para fazer isto.")
	case errors.Is(err, identity.ErrTokenExpired):
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token", error_description="expired"`)
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "A sessão expirou.")
	default:
		w.Header().Set("WWW-Authenticate", "Bearer")
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "É preciso iniciar sessão.")
	}
	return true
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
