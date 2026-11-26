// Package sales records counter sales (UC-06, RF-07..09). It is the only
// module that reads or writes sales and sale_items. A sale is an internal
// record, not a tax document (ADR-0011).
package sales

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Payment is how the customer paid.
type Payment string

const (
	PaymentCash  Payment = "numerario"
	PaymentMBWay Payment = "mbway"
)

// Status of a recorded sale. Voiding (anulada) comes with #16.
type Status string

const (
	StatusCompleted Status = "concluida"
	StatusVoided    Status = "anulada"
)

const (
	// MaxItems is the most lines a sale can have (the contract says the same).
	MaxItems = 50
	// MaxQuantityMilli is the most of one product in a sale: 100 000 kg,
	// litres or units, as for a lot.
	MaxQuantityMilli = 100_000_000
)

// ItemInput is one line as the client sends it: only the product and the
// quantity. Prices always come from the catalog (UC-06).
type ItemInput struct {
	ProductID uuid.UUID
	// QuantityMilli is in thousandths of the product's unit (2500 = 2,5 kg).
	QuantityMilli int64
}

// NewSale is a sale as the client asks for it.
type NewSale struct {
	Payment Payment
	Items   []ItemInput
}

// Normalized merges lines of the same product (adding the quantities) and
// sorts them by product id, so the same cart always gives the same sale and
// the same fingerprint.
func (in NewSale) Normalized() NewSale {
	byID := make(map[uuid.UUID]int64, len(in.Items))
	for _, it := range in.Items {
		byID[it.ProductID] += it.QuantityMilli
	}
	items := make([]ItemInput, 0, len(byID))
	for id, q := range byID {
		items = append(items, ItemInput{ProductID: id, QuantityMilli: q})
	}
	slices.SortFunc(items, func(a, b ItemInput) int { return slices.Compare(a.ProductID[:], b.ProductID[:]) })
	in.Items = items
	return in
}

// Validate checks what does not depend on the catalog. The sale is already
// normalized.
func (in NewSale) Validate() error {
	if in.Payment != PaymentCash && in.Payment != PaymentMBWay {
		return &ValidationError{Field: "payment", Message: "Escolhe o pagamento: numerário ou MB Way."}
	}
	if len(in.Items) == 0 {
		return &ValidationError{Field: "items", Message: "A venda não tem produtos."}
	}
	if len(in.Items) > MaxItems {
		return &ValidationError{Field: "items", Message: "Uma venda pode ter no máximo 50 produtos."}
	}
	for _, it := range in.Items {
		if it.QuantityMilli <= 0 || it.QuantityMilli > MaxQuantityMilli {
			return &ValidationError{Field: "quantityMilli", Message: "Cada quantidade tem de ser maior do que 0 e no máximo 100 000."}
		}
	}
	return nil
}

// Fingerprint identifies the request behind an idempotency key: a retry of
// the same cart gives the same bytes. The sale is already normalized.
func (in NewSale) Fingerprint() []byte {
	h := sha256.New()
	// Writing to a hash never fails.
	_, _ = fmt.Fprintf(h, "payment=%s\n", in.Payment)
	for _, it := range in.Items {
		_, _ = fmt.Fprintf(h, "%s=%d\n", it.ProductID, it.QuantityMilli)
	}
	return h.Sum(nil)
}

// Product is what a sale needs from the catalog.
type Product struct {
	ID   uuid.UUID
	Name string
	Unit string
	// Decimal: kg and litres take decimals; the other units do not.
	Decimal bool
	Active  bool
	// PriceCents is the price per unit, VAT included.
	PriceCents int64
	VatRate    int
}

// Line is a priced line of a sale. Name, unit, price and VAT are the
// catalog's at the time of the sale.
type Line struct {
	ProductID      uuid.UUID
	ProductName    string
	Unit           string
	QuantityMilli  int64
	UnitPriceCents int64
	VatRate        int
	LineTotalCents int64
	// VatCents is the VAT contained in LineTotalCents.
	VatCents int64
}

// Draft is a priced sale, not yet recorded.
type Draft struct {
	Payment    Payment
	Lines      []Line
	TotalCents int64
	VatCents   int64
}

// Price prices a normalized sale with the catalog's products (RF-09): each
// line is rounded to the cent, half up, and the total is the sum of the lines
// (ADR-0009). products holds the products that exist; a missing one is a
// validation error, as is an inactive product or decimals in a unit without
// them.
func Price(in NewSale, products map[uuid.UUID]Product) (Draft, error) {
	d := Draft{Payment: in.Payment, Lines: make([]Line, 0, len(in.Items))}
	for _, it := range in.Items {
		p, ok := products[it.ProductID]
		if !ok {
			return Draft{}, &ValidationError{Field: "productId", Message: "Um dos produtos da venda já não existe. Recarrega a página e volta a juntá-lo."}
		}
		if !p.Active {
			return Draft{}, &ValidationError{Field: "productId", Message: p.Name + " está desativado: não se pode vender."}
		}
		if !p.Decimal && it.QuantityMilli%1000 != 0 {
			return Draft{}, &ValidationError{Field: "quantityMilli", Message: p.Name + " vende-se sem casas decimais: a quantidade tem de ser um número inteiro."}
		}
		total := LineTotalCents(p.PriceCents, it.QuantityMilli)
		vat := VatIncludedCents(total, p.VatRate)
		d.Lines = append(d.Lines, Line{
			ProductID: p.ID, ProductName: p.Name, Unit: p.Unit, QuantityMilli: it.QuantityMilli,
			UnitPriceCents: p.PriceCents, VatRate: p.VatRate, LineTotalCents: total, VatCents: vat,
		})
		d.TotalCents += total
		d.VatCents += vat
	}
	SortLines(d.Lines)
	return d, nil
}

// SortLines puts lines in the order they are shown: by product name, then id.
func SortLines(lines []Line) {
	slices.SortFunc(lines, func(a, b Line) int {
		if c := strings.Compare(a.ProductName, b.ProductName); c != 0 {
			return c
		}
		return slices.Compare(a.ProductID[:], b.ProductID[:])
	})
}

// LineTotalCents is price × quantity, rounded to the cent half up. The
// numbers stay well inside int64: at most 10^7 cents × 10^8 thousandths.
func LineTotalCents(priceCents, quantityMilli int64) int64 {
	return (priceCents*quantityMilli + 500) / 1000
}

// VatIncludedCents is the VAT contained in a price that includes it, at
// ratePercent, rounded to the cent half up: total × rate / (100 + rate).
func VatIncludedCents(totalCents int64, ratePercent int) int64 {
	r := int64(ratePercent)
	d := 100 + r
	return (2*totalCents*r + d) / (2 * d)
}

// Sale is a recorded sale.
type Sale struct {
	ID         uuid.UUID
	Number     string
	UserID     uuid.UUID
	Payment    Payment
	Status     Status
	TotalCents int64
	VatCents   int64
	CreatedAt  time.Time
	Lines      []Line
}
