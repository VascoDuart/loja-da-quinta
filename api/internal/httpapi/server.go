// Package httpapi junta os handlers de todos os módulos numa só
// implementação da interface gerada a partir da OpenAPI.
//
// Cada módulo implementa as operações que lhe pertencem; aqui só se
// compõem. Se faltar uma operação da spec, o código não compila.
package httpapi

import (
	"github.com/VascoDuart/loja-da-quinta/api/internal/auth"
	"github.com/VascoDuart/loja-da-quinta/api/internal/catalog"
	"github.com/VascoDuart/loja-da-quinta/api/internal/health"
	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/users"
)

// Os módulos chamam todos ao seu adaptador "Endpoints"; os aliases dão a
// cada campo embebido um nome diferente.
type (
	healthEndpoints  = health.Handler
	authEndpoints    = auth.Endpoints
	usersEndpoints   = users.Endpoints
	catalogEndpoints = catalog.Endpoints
)

// Server implementa gen.StrictServerInterface.
type Server struct {
	*healthEndpoints
	*authEndpoints
	*usersEndpoints
	*catalogEndpoints
}

var _ gen.StrictServerInterface = (*Server)(nil)

// New junta os handlers dos módulos.
func New(h *health.Handler, a *auth.Endpoints, u *users.Endpoints, c *catalog.Endpoints) *Server {
	return &Server{healthEndpoints: h, authEndpoints: a, usersEndpoints: u, catalogEndpoints: c}
}
