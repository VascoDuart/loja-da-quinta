// Package health implementa os endpoints /healthz e /readyz.
//
//   - /healthz diz se o processo está vivo. Nunca depende da base de dados,
//     para a Render não reiniciar a API quando o problema é a Neon.
//   - /readyz diz se a API consegue servir pedidos, ou seja, se a base de
//     dados responde.
package health

import (
	"context"
	"log/slog"
	"time"

	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/logging"
)

// Pinger é a porta para verificar a base de dados.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handler implementa a parte de saúde da interface gerada da OpenAPI.
type Handler struct {
	db      Pinger
	log     *slog.Logger
	timeout time.Duration
}

// NewHandler cria o handler. O ping à base de dados tem um prazo curto, para
// o /readyz responder depressa mesmo com a base de dados em baixo.
func NewHandler(db Pinger, log *slog.Logger) *Handler {
	return &Handler{db: db, log: log, timeout: 2 * time.Second}
}

func (h *Handler) GetHealth(context.Context, gen.GetHealthRequestObject) (gen.GetHealthResponseObject, error) {
	return gen.GetHealth200JSONResponse{Status: gen.Ok}, nil
}

func (h *Handler) GetReadiness(ctx context.Context, _ gen.GetReadinessRequestObject) (gen.GetReadinessResponseObject, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		h.log.WarnContext(ctx, "base de dados indisponível", "err", err)
		return gen.GetReadiness503ApplicationProblemPlusJSONResponse{
			ProblemApplicationProblemPlusJSONResponse: gen.ProblemApplicationProblemPlusJSONResponse{
				Type:     "about:blank",
				Title:    "Service Unavailable",
				Status:   503,
				Detail:   ptr("A base de dados não está acessível."),
				Instance: ptr(logging.RequestID(ctx)),
			},
		}, nil
	}
	return gen.GetReadiness200JSONResponse{Status: gen.Ok}, nil
}

func ptr[T any](v T) *T { return &v }
