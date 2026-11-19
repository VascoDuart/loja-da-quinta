// Package httpapi junta os handlers de todos os módulos numa só
// implementação da interface gerada a partir da OpenAPI.
//
// Cada módulo implementa as operações que lhe pertencem; aqui só se
// compõem. Se faltar uma operação da spec, o código não compila.
package httpapi

import (
	"github.com/VascoDuart/loja-da-quinta/api/internal/health"
	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
)

// Server implementa gen.StrictServerInterface.
type Server struct {
	*health.Handler
}

var _ gen.StrictServerInterface = (*Server)(nil)
