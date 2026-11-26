// Package inventory manages stock by lots (UC-04, RF-05, ADR-0010). It is the
// only module that reads or writes the stock_lots and stock_movements tables.
package inventory

import (
	"fmt"
	"slices"
	"strings"
	"time"
	_ "time/tzdata" // Lisbon time must not depend on the image's system
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/quantity"
)

// MaxLotMilli is the largest quantity of a lot: 100 000 kg, litres or units,
// to catch typing mistakes (extra zeros) without limiting a real shop.
const MaxLotMilli = 100_000_000

// lisbon is the shop's time zone: "today" is always the day in Lisbon, even
// with the server in UTC.
var lisbon = mustLoad("Europe/Lisbon")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// Day returns the calendar day of t in Lisbon time, as midnight UTC (the way
// days travel between the API and the database).
func Day(t time.Time) time.Time {
	y, m, d := t.In(lisbon).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// asDay drops the time and zone of a day that already came as a calendar day
// (for example "2026-11-25" read from JSON).
func asDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Product is what the stock needs to know about a catalog product.
type Product struct {
	ID   uuid.UUID
	Name string
	// Unit is the product's unit (kg, l, un, molho, duzia). Each lot keeps the
	// unit its quantity was validated in.
	Unit string
	// Decimal: kg and litres take decimals; the other units do not.
	Decimal bool
	Active  bool
	// LowStockMilli is the low stock threshold, in thousandths of the unit.
	LowStockMilli int64
}

// NewLot is a stock entry asked for by a clerk or a manager.
type NewLot struct {
	ProductID uuid.UUID
	// QuantityMilli is the quantity in thousandths of the unit (12500 = 12,5 kg).
	QuantityMilli int64
	HarvestedOn   time.Time
	ExpiresOn     time.Time
}

// Normalized returns the dates as calendar days.
func (in NewLot) Normalized() NewLot {
	in.HarvestedOn = asDay(in.HarvestedOn)
	in.ExpiresOn = asDay(in.ExpiresOn)
	return in
}

// Validate checks a stock entry of p, today being the current day in Lisbon
// (see Day). The dates are already normalized.
func (in NewLot) Validate(p Product, today time.Time) error {
	if !p.Active {
		return &ValidationError{Field: "productId", Message: "Este produto está desativado: não se pode registar stock dele."}
	}
	if in.QuantityMilli <= 0 || in.QuantityMilli > MaxLotMilli {
		return &ValidationError{Field: "quantityMilli", Message: "A quantidade tem de ser maior do que 0 e no máximo 100 000."}
	}
	if !p.Decimal && in.QuantityMilli%1000 != 0 {
		return &ValidationError{Field: "quantityMilli", Message: "Para unidades, molhos e dúzias, a quantidade tem de ser um número inteiro."}
	}
	if in.HarvestedOn.After(today) {
		return &ValidationError{Field: "harvestedOn", Message: "A data de colheita não pode ser no futuro."}
	}
	if in.ExpiresOn.Before(in.HarvestedOn) {
		return &ValidationError{Field: "expiresOn", Message: "A validade não pode ser anterior à data de colheita."}
	}
	if in.ExpiresOn.Before(today) {
		return &ValidationError{Field: "expiresOn", Message: "A validade já passou: um lote fora de validade não se pode vender."}
	}
	return nil
}

// Lot is a stock lot of a product.
type Lot struct {
	ID               uuid.UUID
	ProductID        uuid.UUID
	Unit             string
	QtyReceivedMilli int64
	QtyLeftMilli     int64
	HarvestedOn      time.Time
	ExpiresOn        time.Time
	CreatedAt        time.Time
}

// MovementType is the type of a stock movement (ADR-0010).
type MovementType string

// Voiding a sale (anulacao) comes with #16.
const (
	MovementEntry MovementType = "entrada"
	// MovementAdjust corrects the stock of a lot, up or down (a count that
	// does not match).
	MovementAdjust MovementType = "ajuste"
	// MovementBreakage takes out stock that spoiled or was lost.
	MovementBreakage MovementType = "quebra"
	// MovementSale takes stock out of a lot for a counter sale (FEFO).
	MovementSale MovementType = "venda"
)

// MaxReasonRunes is the longest reason of an adjustment or breakage, in
// characters (the same limit as the database).
const MaxReasonRunes = 500

// Movement is a change to a lot: QuantityMilli, signed, is added to what is
// left in it. Adjustments, breakage and sales all go through applyMovement,
// with the lot locked.
type Movement struct {
	LotID         uuid.UUID
	Type          MovementType
	QuantityMilli int64
	// Reason is empty for movements that do not ask for one (entrada, venda).
	Reason string
	UserID uuid.UUID
	// SaleID is the sale behind a sale (venda) or void (anulacao) movement;
	// uuid.Nil for every other movement.
	SaleID uuid.UUID
}

// RecordedMovement is a movement already recorded.
type RecordedMovement struct {
	ID            int64
	LotID         uuid.UUID
	Type          MovementType
	QuantityMilli int64
	Reason        string
	CreatedAt     time.Time
}

// LeftAfter returns what would be left in the lot after adding deltaMilli.
// Stock never goes negative (InsufficientStockError) nor above a lot's
// maximum (ValidationError). It is the rule of every stock outflow: call it
// with the lot locked, so QtyLeftMilli is current.
func (l Lot) LeftAfter(deltaMilli int64) (int64, error) {
	left := l.QtyLeftMilli + deltaMilli
	if left < 0 {
		return 0, &InsufficientStockError{LeftMilli: l.QtyLeftMilli, Unit: l.Unit}
	}
	if left > MaxLotMilli {
		return 0, &ValidationError{Field: "quantityMilli", Message: "Com este ajuste, o lote ficaria com mais de 100 000: confirma a quantidade."}
	}
	return left, nil
}

// FormatQuantity writes a quantity in thousandths for people, as the app
// does: "12,5 kg", "2 L", "3 dúzias", "1 molho".
func FormatQuantity(milli int64, unit string) string { return quantity.Format(milli, unit) }

// Adjustment is an adjustment or breakage asked for by the manager (RF-06).
type Adjustment struct {
	LotID uuid.UUID
	Type  MovementType
	// QuantityMilli is signed: it is added to the lot. Negative for breakage.
	QuantityMilli int64
	Reason        string
}

// Normalized trims the reason.
func (a Adjustment) Normalized() Adjustment {
	a.Reason = strings.TrimSpace(a.Reason)
	return a
}

// Validate checks what does not depend on the lot: the type, the sign and
// size of the quantity, and the reason (already normalized).
func (a Adjustment) Validate() error {
	if a.Type != MovementAdjust && a.Type != MovementBreakage {
		return &ValidationError{Field: "type", Message: "O tipo tem de ser ajuste ou quebra."}
	}
	if a.QuantityMilli == 0 {
		return &ValidationError{Field: "quantityMilli", Message: "A quantidade não pode ser 0."}
	}
	if a.QuantityMilli < -MaxLotMilli || a.QuantityMilli > MaxLotMilli {
		return &ValidationError{Field: "quantityMilli", Message: "A quantidade tem de ser no máximo 100 000."}
	}
	if a.Type == MovementBreakage && a.QuantityMilli > 0 {
		return &ValidationError{Field: "quantityMilli", Message: "Uma quebra tira stock do lote: a quantidade tem de ser negativa."}
	}
	n := utf8.RuneCountInString(a.Reason)
	if n == 0 {
		return &ValidationError{Field: "reason", Message: "Indica o motivo."}
	}
	if n > MaxReasonRunes {
		return &ValidationError{Field: "reason", Message: "O motivo pode ter no máximo 500 caracteres."}
	}
	if strings.IndexFunc(a.Reason, unicode.IsControl) >= 0 {
		return &ValidationError{Field: "reason", Message: "O motivo tem de ser uma linha de texto, sem caracteres de controlo."}
	}
	return nil
}

// ValidateFor checks the quantity in the unit of the lot's product: un, molho
// and duzia only take whole numbers.
func (a Adjustment) ValidateFor(p Product) error {
	if !p.Decimal && a.QuantityMilli%1000 != 0 {
		return &ValidationError{Field: "quantityMilli", Message: "Para unidades, molhos e dúzias, a quantidade tem de ser um número inteiro."}
	}
	return nil
}

// ExpiringDays: a lot "expires soon" if its expiry date is from today up to
// this many days from now (RF-11).
const ExpiringDays = 3

// LotStatus tells where a lot with stock stands against its expiry date.
type LotStatus string

const (
	LotValid LotStatus = "valido"
	// LotExpiringSoon includes the lot that expires today: it still sells.
	LotExpiringSoon LotStatus = "expira_em_breve"
	// LotExpired no longer sells nor counts as stock: it is for breakage.
	LotExpired LotStatus = "fora_de_validade"
)

// StatusOf classifies an expiry date, today being the current day in Lisbon.
func StatusOf(expiresOn, today time.Time) LotStatus {
	switch {
	case expiresOn.Before(today):
		return LotExpired
	case expiresOn.After(today.AddDate(0, 0, ExpiringDays)):
		return LotValid
	default:
		return LotExpiringSoon
	}
}

// StockLot is a lot with stock, already classified.
type StockLot struct {
	Lot
	Status LotStatus
}

// ProductStock is the stock of a product.
type ProductStock struct {
	Product Product
	// SellableMilli is what is left in the lots within their expiry date.
	SellableMilli int64
	// Low: an active product with sellable stock below its threshold.
	Low  bool
	Lots []StockLot
}

// Overview is the shop's stock on a day.
type Overview struct {
	Today         time.Time
	ExpiringUntil time.Time
	Products      []ProductStock
}

// BuildOverview joins the catalog's products to the lots with stock. It keeps
// the order of the products and of the lots (FEFO). It keeps active products,
// even without stock, and inactive ones that still have lots with stock.
func BuildOverview(products []Product, lots []Lot, today time.Time) Overview {
	byProduct := map[uuid.UUID][]Lot{}
	for _, l := range lots {
		byProduct[l.ProductID] = append(byProduct[l.ProductID], l)
	}
	o := Overview{Today: today, ExpiringUntil: today.AddDate(0, 0, ExpiringDays), Products: []ProductStock{}}
	for _, p := range products {
		own := byProduct[p.ID]
		if !p.Active && len(own) == 0 {
			continue
		}
		ps := ProductStock{Product: p, Lots: make([]StockLot, len(own))}
		for i, l := range own {
			status := StatusOf(l.ExpiresOn, today)
			ps.Lots[i] = StockLot{Lot: l, Status: status}
			if status != LotExpired {
				ps.SellableMilli += l.QtyLeftMilli
			}
		}
		// The same rule as the public catalog: below the threshold; 0 never is.
		ps.Low = p.Active && ps.SellableMilli < p.LowStockMilli
		o.Products = append(o.Products, ps)
	}
	return o
}

// ProductLot is a lot with the product it belongs to.
type ProductLot struct {
	Product Product
	Lot     Lot
}

// Report is what to warn about on a day.
type Report struct {
	Day           time.Time
	ExpiringUntil time.Time
	Low           []ProductStock
	Expiring      []ProductLot
	Expired       []ProductLot
}

// Report takes from the overview what needs attention. Lots come by expiry
// date (most urgent first) and, on the same day, in product order.
func (o Overview) Report() Report {
	r := Report{Day: o.Today, ExpiringUntil: o.ExpiringUntil}
	for _, p := range o.Products {
		if p.Low {
			r.Low = append(r.Low, p)
		}
		for _, l := range p.Lots {
			switch l.Status {
			case LotExpiringSoon:
				r.Expiring = append(r.Expiring, ProductLot{Product: p.Product, Lot: l.Lot})
			case LotExpired:
				r.Expired = append(r.Expired, ProductLot{Product: p.Product, Lot: l.Lot})
			}
		}
	}
	byExpiry := func(a, b ProductLot) int { return a.Lot.ExpiresOn.Compare(b.Lot.ExpiresOn) }
	slices.SortStableFunc(r.Expiring, byExpiry)
	slices.SortStableFunc(r.Expired, byExpiry)
	return r
}

// Empty: there is nothing to warn about.
func (r Report) Empty() bool { return len(r.Low)+len(r.Expiring)+len(r.Expired) == 0 }

// Dates as written in Portugal.
const ptDate = "02/01/2006"

// Message writes the alert e-mail, in plain text (pt-PT, for people). It only
// shows the sections that have something.
func (r Report) Message() (subject, text string) {
	subject = "Loja da Quinta: alertas de stock de " + r.Day.Format(ptDate)

	var b strings.Builder
	section := func(title string, lines []string) {
		if len(lines) == 0 {
			return
		}
		fmt.Fprintf(&b, "\n%s (%d)\n", title, len(lines))
		for _, l := range lines {
			b.WriteString("- " + l + "\n")
		}
	}
	lots := func(list []ProductLot) []string {
		out := make([]string, len(list))
		for i, pl := range list {
			out[i] = fmt.Sprintf("%s: %s, validade %s",
				pl.Product.Name, FormatQuantity(pl.Lot.QtyLeftMilli, pl.Lot.Unit), pl.Lot.ExpiresOn.Format(ptDate))
		}
		return out
	}
	low := make([]string, len(r.Low))
	for i, p := range r.Low {
		low[i] = fmt.Sprintf("%s: %s para vender (limiar: %s)", p.Product.Name,
			FormatQuantity(p.SellableMilli, p.Product.Unit), FormatQuantity(p.Product.LowStockMilli, p.Product.Unit))
	}

	b.WriteString("Bom dia. Isto é o que pede atenção no stock a " + r.Day.Format(ptDate) + ".\n")
	section("Stock baixo", low)
	section("A expirar até "+r.ExpiringUntil.Format(ptDate), lots(r.Expiring))
	section("Fora de validade, para dar como quebra", lots(r.Expired))
	return subject, b.String()
}
