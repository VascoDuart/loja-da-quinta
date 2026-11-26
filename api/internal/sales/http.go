package sales

import (
	"context"
	"errors"
	"net/http"

	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/problems"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/quantity"
)

// Endpoints implements the operations of the sales tag of the OpenAPI.
type Endpoints struct{ svc *Service }

// NewEndpoints creates the handlers.
func NewEndpoints(svc *Service) *Endpoints { return &Endpoints{svc: svc} }

const (
	msgForbidden = "Não tens permissão para registar vendas."
	msgKeyReused = "Este pedido de venda já foi usado para outro carrinho. Recarrega a página e volta a confirmar."
)

func (h *Endpoints) CreateSale(ctx context.Context, req gen.CreateSaleRequestObject) (gen.CreateSaleResponseObject, error) {
	in := NewSale{Payment: Payment(req.Body.Payment), Items: make([]ItemInput, len(req.Body.Items))}
	for i, it := range req.Body.Items {
		in.Items[i] = ItemInput{ProductID: it.ProductId, QuantityMilli: it.QuantityMilli}
	}
	sale, created, err := h.svc.Register(ctx, actor(ctx), req.Params.IdempotencyKey, in)
	var (
		ve    *ValidationError
		short *InsufficientStockError
	)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.CreateSale403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbidden)), nil
	case errors.As(err, &short):
		return gen.CreateSale409ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusConflict, shortageMessage(short))), nil
	case errors.Is(err, ErrKeyReused):
		return gen.CreateSale422ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnprocessableEntity, msgKeyReused)), nil
	case errors.As(err, &ve):
		return gen.CreateSale422ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnprocessableEntity, ve.Message)), nil
	case err != nil:
		return nil, err
	}
	if created {
		return gen.CreateSale201JSONResponse(toSale(sale)), nil
	}
	return gen.CreateSale200JSONResponse(toSale(sale)), nil
}

// shortageMessage names the product and how much can still be sold (UC-06
// 7a): the clerk fixes that line and confirms again.
func shortageMessage(e *InsufficientStockError) string {
	if e.AvailableMilli == 0 {
		return e.ProductName + " está esgotado: tira-o da venda para continuar. Nada foi registado."
	}
	return "Não há stock que chegue de " + e.ProductName + ": só há " + quantity.Format(e.AvailableMilli, e.Unit) +
		" à venda. Ajusta a quantidade e confirma de novo. Nada foi registado."
}

func toSale(s Sale) gen.Sale {
	items := make([]gen.SaleItem, len(s.Lines))
	for i, l := range s.Lines {
		items[i] = gen.SaleItem{
			ProductId: l.ProductID, ProductName: l.ProductName, Unit: gen.Unit(l.Unit), QuantityMilli: l.QuantityMilli,
			UnitPriceCents: l.UnitPriceCents, VatRate: gen.VatRate(l.VatRate), LineTotalCents: l.LineTotalCents, VatCents: l.VatCents,
		}
	}
	return gen.Sale{
		Id: s.ID, Number: s.Number, Payment: gen.Payment(s.Payment), Status: gen.SaleStatus(s.Status),
		TotalCents: s.TotalCents, VatCents: s.VatCents, CreatedAt: s.CreatedAt, Items: items,
	}
}

// actor returns who makes the request. Without a session it is an empty
// principal, which the service refuses (the API's RBAC never lets one through).
func actor(ctx context.Context) identity.Principal {
	p, _ := identity.From(ctx)
	return p
}
