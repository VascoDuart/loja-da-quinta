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
	msgForbiddenAdjust = "Só o gestor pode registar ajustes e quebras."
	msgLotNotFound     = "Este lote não existe."
	msgProductNotFound = "O produto escolhido não existe."
	msgUnitChanged     = "A unidade deste produto mudou enquanto registavas a entrada. Recarrega a página, confirma a quantidade e regista de novo."
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
	case errors.Is(err, ErrUnitChanged):
		return gen.CreateStockLot409ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusConflict, msgUnitChanged)), nil
	case errors.Is(err, ErrProductNotFound):
		return gen.CreateStockLot422ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnprocessableEntity, msgProductNotFound)), nil
	case errors.As(err, &ve):
		return gen.CreateStockLot422ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnprocessableEntity, ve.Message)), nil
	case err != nil:
		return nil, err
	}
	return gen.CreateStockLot201JSONResponse(toStockLot(lot)), nil
}

func (h *Endpoints) CreateStockAdjustment(ctx context.Context, req gen.CreateStockAdjustmentRequestObject) (gen.CreateStockAdjustmentResponseObject, error) {
	b := req.Body
	lot, mv, err := h.svc.Adjust(ctx, actor(ctx), Adjustment{
		LotID: b.LotId, Type: MovementType(b.Type), QuantityMilli: b.QuantityMilli, Reason: b.Reason,
	})
	var (
		ve    *ValidationError
		short *InsufficientStockError
	)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.CreateStockAdjustment403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbiddenAdjust)), nil
	case errors.Is(err, ErrLotNotFound):
		return gen.CreateStockAdjustment404ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusNotFound, msgLotNotFound)), nil
	case errors.As(err, &short):
		return gen.CreateStockAdjustment409ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusConflict, insufficientMessage(short))), nil
	case errors.As(err, &ve):
		return gen.CreateStockAdjustment422ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnprocessableEntity, ve.Message)), nil
	case err != nil:
		return nil, err
	}
	return gen.CreateStockAdjustment201JSONResponse{
		Movement: gen.StockMovement{
			Id: mv.ID, LotId: mv.LotID, Type: gen.StockAdjustmentType(mv.Type),
			QuantityMilli: mv.QuantityMilli, Reason: mv.Reason, CreatedAt: mv.CreatedAt,
		},
		Lot: toStockLot(lot),
	}, nil
}

func (h *Endpoints) ListStockLots(ctx context.Context, req gen.ListStockLotsRequestObject) (gen.ListStockLotsResponseObject, error) {
	lots, err := h.svc.ListLots(ctx, actor(ctx), req.Params.ProductId)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.ListStockLots403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbidden)), nil
	case err != nil:
		return nil, err
	}
	out := make(gen.ListStockLots200JSONResponse, len(lots))
	for i, l := range lots {
		out[i] = toStockLot(l)
	}
	return out, nil
}

// insufficientMessage diz quanto resta no lote, porque quem pediu pode estar
// a ver um valor antigo (outra pessoa mexeu no lote entretanto).
func insufficientMessage(e *InsufficientStockError) string {
	if e.LeftMilli == 0 {
		return "Este lote já não tem stock: não se pode tirar mais (o stock nunca fica negativo). Recarrega os lotes."
	}
	return "Este lote só tem " + FormatQuantity(e.LeftMilli, e.Unit) +
		": não se pode tirar mais do que isso (o stock nunca fica negativo). Recarrega os lotes para veres o valor atual."
}

func toStockLot(l Lot) gen.StockLot {
	return gen.StockLot{
		Id: l.ID, ProductId: l.ProductID, Unit: gen.Unit(l.Unit),
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
