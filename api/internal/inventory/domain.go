// Package inventory gere o stock por lotes (UC-04, RF-05, ADR-0010). É o
// único módulo que lê ou escreve as tabelas stock_lots e stock_movements.
package inventory

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // a hora de Lisboa não pode depender do sistema da imagem
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// MaxLotMilli é a quantidade máxima de um lote: 100 000 kg, litros ou
// unidades, para apanhar enganos de digitação (zeros a mais) sem limitar uma
// loja real.
const MaxLotMilli = 100_000_000

// lisbon é o fuso da loja: "hoje" é sempre o dia em Lisboa, mesmo com o
// servidor em UTC.
var lisbon = mustLoad("Europe/Lisbon")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// Day devolve o dia do calendário de t na hora de Lisboa, como meia-noite UTC
// (a forma como os dias andam entre a API e a base de dados).
func Day(t time.Time) time.Time {
	y, m, d := t.In(lisbon).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// asDay tira a hora e o fuso de um dia que já veio como dia do calendário
// (por exemplo, "2026-11-25" lido do JSON).
func asDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Product é o que o stock precisa de saber de um produto do catálogo.
type Product struct {
	ID   uuid.UUID
	Name string
	// Unit é a unidade do produto (kg, l, un, molho, duzia). Cada lote guarda
	// a unidade em que a quantidade foi validada.
	Unit string
	// Decimal: kg e litros aceitam casas decimais; as outras unidades não.
	Decimal bool
	Active  bool
}

// NewLot é uma entrada de stock pedida por um funcionário ou gestor.
type NewLot struct {
	ProductID uuid.UUID
	// QuantityMilli é a quantidade em milésimos da unidade (12500 = 12,5 kg).
	QuantityMilli int64
	HarvestedOn   time.Time
	ExpiresOn     time.Time
}

// Normalized devolve as datas como dias do calendário.
func (in NewLot) Normalized() NewLot {
	in.HarvestedOn = asDay(in.HarvestedOn)
	in.ExpiresOn = asDay(in.ExpiresOn)
	return in
}

// Validate verifica uma entrada de stock de p, sendo today o dia de hoje em
// Lisboa (ver Day). As datas já vêm normalizadas.
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

// Lot é um lote de stock de um produto.
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

// MovementType é o tipo de um movimento de stock (ADR-0010).
type MovementType string

// Venda e anulação chegam com as vendas (#15).
const (
	MovementEntry MovementType = "entrada"
	// MovementAdjust corrige o stock de um lote, para mais ou para menos
	// (uma contagem que não bate certo).
	MovementAdjust MovementType = "ajuste"
	// MovementBreakage tira stock que se estragou ou perdeu.
	MovementBreakage MovementType = "quebra"
)

// MaxReasonRunes é o tamanho máximo do motivo de um ajuste ou quebra, em
// caracteres (o mesmo limite que a base de dados).
const MaxReasonRunes = 500

// Movement é uma alteração a um lote: QuantityMilli, com sinal, soma-se ao
// que resta nele. Ajustes, quebras e, no #15, as vendas e as anulações
// passam todos por aqui (Repo.Move), com o lote bloqueado.
type Movement struct {
	LotID         uuid.UUID
	Type          MovementType
	QuantityMilli int64
	// Reason é o motivo; vazio nos movimentos que não o pedem (entrada).
	Reason string
	UserID uuid.UUID
}

// RecordedMovement é um movimento já gravado.
type RecordedMovement struct {
	ID            int64
	LotID         uuid.UUID
	Type          MovementType
	QuantityMilli int64
	Reason        string
	CreatedAt     time.Time
}

// LeftAfter devolve o que restaria no lote depois de lhe somar deltaMilli.
// O stock nunca fica negativo (InsufficientStockError) nem passa do máximo de
// um lote (ValidationError). É a regra de todas as saídas de stock: chama-se
// com o lote bloqueado, para o valor de QtyLeftMilli ser o atual.
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

// FormatQuantity escreve uma quantidade em milésimos para pessoas, como a
// app: "12,5 kg", "2 L", "3 dúzias", "1 molho".
func FormatQuantity(milli int64, unit string) string {
	whole, frac := milli/1000, milli%1000
	if frac < 0 {
		frac = -frac
	}
	n := strconv.FormatInt(whole, 10)
	if frac != 0 {
		if whole == 0 && milli < 0 {
			n = "-0"
		}
		n += "," + strings.TrimRight(fmt.Sprintf("%03d", frac), "0")
	}
	one := milli == 1000 || milli == -1000
	names := map[string][2]string{
		"kg": {"kg", "kg"}, "l": {"L", "L"}, "un": {"unidade", "unidades"},
		"molho": {"molho", "molhos"}, "duzia": {"dúzia", "dúzias"},
	}
	name, ok := names[unit]
	if !ok {
		return n + " " + unit
	}
	if one {
		return n + " " + name[0]
	}
	return n + " " + name[1]
}

// Adjustment é um ajuste ou uma quebra pedida pelo gestor (RF-06).
type Adjustment struct {
	LotID uuid.UUID
	Type  MovementType
	// QuantityMilli tem sinal: soma-se ao lote. Numa quebra, é negativa.
	QuantityMilli int64
	Reason        string
}

// Normalized tira os espaços das pontas do motivo.
func (a Adjustment) Normalized() Adjustment {
	a.Reason = strings.TrimSpace(a.Reason)
	return a
}

// Validate verifica o que não depende do lote: o tipo, o sinal e o tamanho da
// quantidade, e o motivo. O motivo já vem normalizado.
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

// ValidateFor verifica a quantidade na unidade do produto do lote: un, molho
// e dúzia só aceitam inteiros.
func (a Adjustment) ValidateFor(p Product) error {
	if !p.Decimal && a.QuantityMilli%1000 != 0 {
		return &ValidationError{Field: "quantityMilli", Message: "Para unidades, molhos e dúzias, a quantidade tem de ser um número inteiro."}
	}
	return nil
}
