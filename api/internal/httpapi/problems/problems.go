// Package problems cria os corpos de erro problem+json (RFC 9457) das
// respostas geradas pelo oapi-codegen, com o request ID em instance.
package problems

import (
	"context"
	"net/http"

	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/logging"
)

// New devolve um problem com o título do status e um detalhe escrito para
// quem usa a app. Nunca levar aqui detalhes internos.
func New(ctx context.Context, status int, detail string) gen.Problem {
	p := gen.Problem{Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: &detail}
	if id := logging.RequestID(ctx); id != "" {
		p.Instance = &id
	}
	return p
}
