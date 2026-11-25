// Package inventory gere o stock por lotes (UC-04, RF-05, ADR-0010). É o
// único módulo que lê ou escreve as tabelas stock_lots e stock_movements.
package inventory

import (
	"time"
	_ "time/tzdata" // a hora de Lisboa não pode depender do sistema da imagem

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
	QtyReceivedMilli int64
	QtyLeftMilli     int64
	HarvestedOn      time.Time
	ExpiresOn        time.Time
	CreatedAt        time.Time
}

// MovementType é o tipo de um movimento de stock (ADR-0010).
type MovementType string

// Só a entrada existe hoje; venda e anulação chegam com as vendas (#15) e
// ajuste e quebra com os ajustes (#13).
const MovementEntry MovementType = "entrada"
