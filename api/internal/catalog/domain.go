// Package catalog gere as categorias, os produtos e as imagens dos produtos
// (UC-03, RF-04, ADR-0014). É o único módulo que lê ou escreve as tabelas
// categories, products e product_images.
package catalog

import (
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Unit é a unidade de venda de um produto.
type Unit string

const (
	UnitKg    Unit = "kg"
	UnitLitre Unit = "l"
	UnitUnit  Unit = "un"
	UnitBunch Unit = "molho"
	UnitDozen Unit = "duzia"
)

// Valid indica se a unidade existe.
func (u Unit) Valid() bool {
	switch u {
	case UnitKg, UnitLitre, UnitUnit, UnitBunch, UnitDozen:
		return true
	}
	return false
}

// Decimal indica se a unidade aceita casas decimais (pesos e volumes).
func (u Unit) Decimal() bool { return u == UnitKg || u == UnitLitre }

// VatRates são as taxas de IVA em Portugal continental (RF-04).
var VatRates = []int{6, 13, 23}

// MaxPriceCents é o preço máximo por unidade: 100 000 €, para apanhar
// enganos de digitação (um zero a mais) sem limitar uma loja real.
const MaxPriceCents = 10_000_000

// Category agrupa produtos.
type Category struct {
	ID           uuid.UUID
	Name         string
	ProductCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ProductInput são os dados que o gestor define num produto.
type ProductInput struct {
	CategoryID uuid.UUID
	Name       string
	Unit       Unit
	PriceCents int
	VatRate    int
	// SeasonMonths são os meses da época (1 = janeiro); vazio = todo o ano.
	SeasonMonths []int
	// LowStockMilli é o limiar de stock baixo em milésimos da unidade
	// (2500 = 2,5 kg). As quantidades nunca passam por vírgula flutuante.
	LowStockMilli int64
	IsPublic      bool
	Active        bool
}

// Product é um produto do catálogo.
type Product struct {
	ProductInput
	ID           uuid.UUID
	CategoryName string
	// ImageVersion é o hash da imagem grande, ou "" sem imagem.
	ImageVersion string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Version é a versão do produto para o If-Match: muda a cada gravação.
func (p Product) Version() string { return strconv.FormatInt(p.UpdatedAt.UnixMicro(), 10) }

// ParseVersion lê uma versão vinda de um If-Match ("123", "\"123\"" ou W/"123").
func ParseVersion(s string) (time.Time, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "W/")
	s = strings.Trim(s, `"`)
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}, false
	}
	return time.UnixMicro(n), true
}

// NormalizeName tira os espaços a mais de um nome ("  Laranja   do  Algarve ").
func NormalizeName(s string) string { return strings.Join(strings.Fields(s), " ") }

// ValidateCategoryName verifica o nome de uma categoria, já normalizado.
func ValidateCategoryName(name string) error {
	if name == "" || utf8.RuneCountInString(name) > 60 {
		return &ValidationError{Field: "name", Message: "O nome da categoria tem de ter entre 1 e 60 caracteres."}
	}
	return validateText("name", name)
}

// validateText recusa texto com caracteres de controlo ou com o carácter de
// substituição (U+FFFD), que aparece quando um cliente envia texto que não é
// UTF-8: "Hortícolas" ficaria "Hort�colas" para sempre.
func validateText(field, s string) error {
	for _, r := range s {
		if r == utf8.RuneError || unicode.IsControl(r) {
			return &ValidationError{Field: field, Message: "O texto tem caracteres inválidos. Escreve-o de novo."}
		}
	}
	return nil
}

// Validate verifica os dados de um produto, com o nome já normalizado.
func (in ProductInput) Validate() error {
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 100 {
		return &ValidationError{Field: "name", Message: "O nome do produto tem de ter entre 1 e 100 caracteres."}
	}
	if err := validateText("name", in.Name); err != nil {
		return err
	}
	if !in.Unit.Valid() {
		return &ValidationError{Field: "unit", Message: "A unidade tem de ser kg, litro, unidade, molho ou dúzia."}
	}
	if in.PriceCents < 0 || in.PriceCents > MaxPriceCents {
		return &ValidationError{Field: "priceCents", Message: "O preço tem de estar entre 0 € e 100 000 €."}
	}
	if !slices.Contains(VatRates, in.VatRate) {
		return &ValidationError{Field: "vatRate", Message: "A taxa de IVA tem de ser 6, 13 ou 23 %."}
	}
	seen := map[int]bool{}
	for _, m := range in.SeasonMonths {
		if m < 1 || m > 12 || seen[m] {
			return &ValidationError{Field: "seasonMonths", Message: "Os meses da época têm de ser de 1 a 12, sem repetições."}
		}
		seen[m] = true
	}
	if in.LowStockMilli < 0 || in.LowStockMilli > 999_999_999_999 {
		return &ValidationError{Field: "lowStockMilli", Message: "O limiar de stock baixo não é válido."}
	}
	if !in.Unit.Decimal() && in.LowStockMilli%1000 != 0 {
		return &ValidationError{Field: "lowStockMilli", Message: "Para unidades, molhos e dúzias, o limiar de stock baixo tem de ser um número inteiro."}
	}
	return nil
}

// Normalized devolve os dados com o nome limpo e os meses por ordem.
func (in ProductInput) Normalized() ProductInput {
	in.Name = NormalizeName(in.Name)
	in.SeasonMonths = slices.Clone(in.SeasonMonths)
	slices.Sort(in.SeasonMonths)
	if in.SeasonMonths == nil {
		in.SeasonMonths = []int{}
	}
	return in
}

// Availability é o estado do stock de um produto como o público o vê: sem
// quantidades exatas (information disclosure, modelo de ameaças).
type Availability string

const (
	Available Availability = "disponivel"
	Low       Availability = "pouco"
	SoldOut   Availability = "esgotado"
)

// AvailabilityOf decide a disponibilidade a partir do stock e do limiar de
// stock baixo do produto, ambos em milésimos da unidade:
//
//   - esgotado: stock 0 (ou menos);
//   - pouco: stock abaixo do limiar (low_stock); um limiar 0 nunca dá "pouco";
//   - disponivel: nos outros casos.
func AvailabilityOf(stockMilli, lowStockMilli int64) Availability {
	switch {
	case stockMilli <= 0:
		return SoldOut
	case stockMilli < lowStockMilli:
		return Low
	default:
		return Available
	}
}

// PublicProduct é o que o catálogo público mostra de um produto. De
// propósito não tem IVA, limiar de stock, "ativo" nem datas, e a
// disponibilidade substitui a quantidade.
type PublicProduct struct {
	ID           uuid.UUID
	Name         string
	CategoryName string
	Unit         Unit
	PriceCents   int
	SeasonMonths []int
	// ImageVersion é o hash da imagem grande, ou "" sem imagem.
	ImageVersion string
	Availability Availability
}
