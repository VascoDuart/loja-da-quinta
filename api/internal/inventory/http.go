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
type Endpoints struct {
	svc    *Service
	alerts *Alerts
}

// NewEndpoints cria os handlers.
func NewEndpoints(svc *Service, alerts *Alerts) *Endpoints {
	return &Endpoints{svc: svc, alerts: alerts}
}

const (
	msgForbidden       = "Não tens permissão para registar stock."
	msgForbiddenView   = "Não tens permissão para ver o stock."
	msgAlertNotSent    = "O e-mail não foi enviado. O próximo pedido volta a tentar."
	msgNoRecipients    = "Não há a quem enviar: é preciso um gestor ativo ou ALERTS_TO."
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

func (h *Endpoints) GetStockOverview(ctx context.Context, _ gen.GetStockOverviewRequestObject) (gen.GetStockOverviewResponseObject, error) {
	o, err := h.svc.Overview(ctx, actor(ctx))
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.GetStockOverview403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbiddenView)), nil
	case err != nil:
		return nil, err
	}
	out := gen.GetStockOverview200JSONResponse{
		Today:         openapi_types.Date{Time: o.Today},
		ExpiringUntil: openapi_types.Date{Time: o.ExpiringUntil},
		Products:      make([]gen.StockProduct, len(o.Products)),
	}
	for i, p := range o.Products {
		lots := make([]gen.StockOverviewLot, len(p.Lots))
		for j, l := range p.Lots {
			base := toStockLot(l.Lot)
			lots[j] = gen.StockOverviewLot{
				Id: base.Id, ProductId: base.ProductId, Unit: base.Unit,
				QuantityReceivedMilli: base.QuantityReceivedMilli, QuantityLeftMilli: base.QuantityLeftMilli,
				HarvestedOn: base.HarvestedOn, ExpiresOn: base.ExpiresOn, CreatedAt: base.CreatedAt,
				Status: gen.LotStatus(l.Status),
			}
		}
		out.Products[i] = gen.StockProduct{
			ProductId: p.Product.ID, Name: p.Product.Name, Unit: gen.Unit(p.Product.Unit), Active: p.Product.Active,
			SellableMilli: p.SellableMilli, LowStockMilli: p.Product.LowStockMilli, Low: p.Low, Lots: lots,
		}
	}
	return out, nil
}

// RunStockAlerts é o job diário. Quem o pode chamar é decidido antes, pelo
// esquema jobToken da OpenAPI.
func (h *Endpoints) RunStockAlerts(ctx context.Context, _ gen.RunStockAlertsRequestObject) (gen.RunStockAlertsResponseObject, error) {
	run, err := h.alerts.Run(ctx)
	switch {
	case errors.Is(err, ErrNoRecipients):
		h.alerts.log.ErrorContext(ctx, "alerta de stock sem destinatários", "err", err)
		return gen.RunStockAlerts502ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusBadGateway, msgNoRecipients)), nil
	case errors.Is(err, ErrAlertNotSent):
		h.alerts.log.ErrorContext(ctx, "o alerta de stock não foi enviado", "err", err)
		return gen.RunStockAlerts502ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusBadGateway, msgAlertNotSent)), nil
	case err != nil:
		return nil, err
	}
	out := gen.RunStockAlerts200JSONResponse{
		Status: gen.StockAlertsRunStatus(run.Status), Day: openapi_types.Date{Time: run.Day},
		LowStockCount: run.Low, ExpiringCount: run.Expiring, ExpiredCount: run.Expired,
	}
	if run.Delivery != "" {
		d := gen.StockAlertsRunDelivery(run.Delivery)
		out.Delivery = &d
	}
	h.alerts.log.InfoContext(ctx, "job dos alertas de stock", "estado", run.Status, "entrega", run.Delivery)
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
