// Package httpapi joins the handlers of every module into a single
// implementation of the interface generated from the OpenAPI.
//
// Each module implements the operations it owns; here they are only
// composed. If an operation of the spec is missing, the code does not compile.
package httpapi

import (
	"github.com/VascoDuart/loja-da-quinta/api/internal/auth"
	"github.com/VascoDuart/loja-da-quinta/api/internal/catalog"
	"github.com/VascoDuart/loja-da-quinta/api/internal/health"
	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/inventory"
	"github.com/VascoDuart/loja-da-quinta/api/internal/sales"
	"github.com/VascoDuart/loja-da-quinta/api/internal/users"
)

// Every module calls its adapter "Endpoints"; the aliases give each embedded
// field a different name.
type (
	healthEndpoints    = health.Handler
	authEndpoints      = auth.Endpoints
	usersEndpoints     = users.Endpoints
	catalogEndpoints   = catalog.Endpoints
	inventoryEndpoints = inventory.Endpoints
	salesEndpoints     = sales.Endpoints
)

// Server implements gen.StrictServerInterface.
type Server struct {
	*healthEndpoints
	*authEndpoints
	*usersEndpoints
	*catalogEndpoints
	*inventoryEndpoints
	*salesEndpoints
}

var _ gen.StrictServerInterface = (*Server)(nil)

// New joins the handlers of the modules.
func New(h *health.Handler, a *auth.Endpoints, u *users.Endpoints, c *catalog.Endpoints, i *inventory.Endpoints, sa *sales.Endpoints) *Server {
	return &Server{
		healthEndpoints: h, authEndpoints: a, usersEndpoints: u, catalogEndpoints: c, inventoryEndpoints: i,
		salesEndpoints: sa,
	}
}
