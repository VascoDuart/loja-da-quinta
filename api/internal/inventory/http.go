package inventory

import (
	"context"
	"errors"
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/problems"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// Endpoints implementa as operações da tag inventory da OpenAPI.
type Endpoints struct{ svc *Service }

// NewEndpoints cria os handlers.
func NewEndpoints(svc *Service) *Endpoints { return &Endpoints{svc: svc} }

const (
	msgForbidden       = "Não tens permissão para registar stock."
	msgProductNotFound = "O produto escolhido não existe."
)

func (h *Endpoints) CreateStockLot(ctx context.Context, req gen.CreateStockLotRequestObject) (gen.CreateStockLotResponseObject, error) {
	b := req.Body
	lot, err := h.svc.RegisterLot(ctx, actor(ctx), NewLot{
		ProductID: b.ProductId, QuantityMilli: b.QuantityMilli,
		HarvestedOn: b.HarvestedOn.Time, ExpiresOn: b.ExpiresOn.Time,
	})
	var ve *ValidationError
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.CreateStockLot403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbidden)), nil
	case errors.Is(err, ErrProductNotFound):
		return gen.CreateStockLot422ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnprocessableEntity, msgProductNotFound)), nil
	case errors.As(err, &ve):
		return gen.CreateStockLot422ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnprocessableEntity, ve.Message)), nil
	case err != nil:
		return nil, err
	}
	return gen.CreateStockLot201JSONResponse(toStockLot(lot)), nil
}

func toStockLot(l Lot) gen.StockLot {
	return gen.StockLot{
		Id: l.ID, ProductId: l.ProductID,
		QuantityReceivedMilli: l.QtyReceivedMilli, QuantityLeftMilli: l.QtyLeftMilli,
		HarvestedOn: openapi_types.Date{Time: l.HarvestedOn}, ExpiresOn: openapi_types.Date{Time: l.ExpiresOn},
		CreatedAt: l.CreatedAt,
	}
}

// actor devolve quem faz o pedido. Sem sessão devolve um principal vazio,
// que o serviço recusa (o RBAC da API nunca deixa chegar aqui sem sessão).
func actor(ctx context.Context) identity.Principal {
	p, _ := identity.From(ctx)
	return p
}
