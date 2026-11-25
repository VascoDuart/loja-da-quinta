package inventory

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

func validAdjustment() Adjustment {
	return Adjustment{LotID: uuid.New(), Type: MovementBreakage, QuantityMilli: -1500, Reason: "Caixa caiu ao descarregar"}
}

func TestAdjustmentValidation(t *testing.T) {
	ok := map[string]func(*Adjustment){
		"quebra de 1,5 kg":             func(*Adjustment) {},
		"ajuste para mais":             func(a *Adjustment) { a.Type, a.QuantityMilli = MovementAdjust, 2000 },
		"ajuste para menos":            func(a *Adjustment) { a.Type, a.QuantityMilli = MovementAdjust, -1 },
		"o máximo, para os dois lados": func(a *Adjustment) { a.Type, a.QuantityMilli = MovementAdjust, -MaxLotMilli },
		"motivo com 500 caracteres":    func(a *Adjustment) { a.Reason = strings.Repeat("é", MaxReasonRunes) },
		"motivo com espaços nas pontas": func(a *Adjustment) {
			a.Reason = "  contagem de inventário  "
		},
	}
	for name, mutate := range ok {
		t.Run("aceita "+name, func(t *testing.T) {
			a := validAdjustment()
			mutate(&a)
			if err := a.Normalized().Validate(); err != nil {
				t.Errorf("devia aceitar: %v", err)
			}
		})
	}

	bad := map[string]struct {
		mutate func(*Adjustment)
		field  string
	}{
		"quantidade 0":                {func(a *Adjustment) { a.QuantityMilli = 0 }, "quantityMilli"},
		"quebra positiva":             {func(a *Adjustment) { a.QuantityMilli = 1500 }, "quantityMilli"},
		"quantidade absurda":          {func(a *Adjustment) { a.QuantityMilli = -MaxLotMilli - 1 }, "quantityMilli"},
		"ajuste absurdo":              {func(a *Adjustment) { a.Type, a.QuantityMilli = MovementAdjust, MaxLotMilli+1 }, "quantityMilli"},
		"tipo entrada":                {func(a *Adjustment) { a.Type, a.QuantityMilli = MovementEntry, 1000 }, "type"},
		"tipo venda":                  {func(a *Adjustment) { a.Type = "venda" }, "type"},
		"sem motivo":                  {func(a *Adjustment) { a.Reason = "" }, "reason"},
		"motivo só com espaços":       {func(a *Adjustment) { a.Reason = " \t " }, "reason"},
		"motivo com 501":              {func(a *Adjustment) { a.Reason = strings.Repeat("a", MaxReasonRunes+1) }, "reason"},
		"motivo com mudança de linha": {func(a *Adjustment) { a.Reason = "linha 1\nlinha 2" }, "reason"},
	}
	for name, c := range bad {
		t.Run("recusa "+name, func(t *testing.T) {
			a := validAdjustment()
			c.mutate(&a)
			var ve *ValidationError
			if err := a.Normalized().Validate(); !errors.As(err, &ve) || ve.Field != c.field {
				t.Errorf("queria ValidationError em %s, deu %v", c.field, err)
			}
		})
	}

	t.Run("unidades, molhos e dúzias só inteiros", func(t *testing.T) {
		a := validAdjustment()
		if err := a.ValidateFor(dozens); err == nil {
			t.Error("-1,5 dúzias devia ser recusado")
		}
		a.QuantityMilli = -2000
		if err := a.ValidateFor(dozens); err != nil {
			t.Errorf("-2 dúzias devia passar: %v", err)
		}
		a.QuantityMilli = -1
		if err := a.ValidateFor(kg); err != nil {
			t.Errorf("-1 g devia passar: %v", err)
		}
	})
}

func TestLotLeftAfter(t *testing.T) {
	l := Lot{QtyLeftMilli: 4500, Unit: "kg"}
	cases := []struct {
		delta   int64
		want    int64
		failure bool
	}{
		{-4500, 0, false}, // tirar tudo
		{-1, 4499, false},
		{1000, 5500, false},
		{-4501, 0, true}, // ficaria negativo
		{MaxLotMilli - 4500, MaxLotMilli, false},
		{MaxLotMilli - 4499, 0, true}, // passaria do máximo
	}
	for _, c := range cases {
		got, err := l.LeftAfter(c.delta)
		if (err != nil) != c.failure || got != c.want {
			t.Errorf("%d: queria %d (falha %v), deu %d %v", c.delta, c.want, c.failure, got, err)
		}
	}
	var short *InsufficientStockError
	if _, err := l.LeftAfter(-5000); !errors.As(err, &short) || short.LeftMilli != 4500 || short.Unit != "kg" {
		t.Errorf("o erro diz o que resta: %v", err)
	}
}

func TestFormatQuantity(t *testing.T) {
	cases := map[string]struct {
		milli int64
		unit  string
	}{
		"12,5 kg":   {12_500, "kg"},
		"0,001 kg":  {1, "kg"},
		"-0,25 L":   {-250, "l"},
		"2 L":       {2000, "l"},
		"1 dúzia":   {1000, "duzia"},
		"3 dúzias":  {3000, "duzia"},
		"1 unidade": {1000, "un"},
		"0 molhos":  {0, "molho"},
		"-1 molho":  {-1000, "molho"},
	}
	for want, c := range cases {
		if got := FormatQuantity(c.milli, c.unit); got != want {
			t.Errorf("%d %s: queria %q, deu %q", c.milli, c.unit, want, got)
		}
	}
}

func TestAdjust(t *testing.T) {
	ctx := context.Background()
	gestor := identity.Principal{UserID: uuid.New(), Role: identity.RoleGestor}
	funcionario := identity.Principal{UserID: uuid.New(), Role: identity.RoleFuncionario}
	lotKg := Lot{ID: uuid.New(), ProductID: kg.ID, Unit: "kg", QtyReceivedMilli: 10_000, QtyLeftMilli: 4500}
	lotDozens := Lot{ID: uuid.New(), ProductID: dozens.ID, Unit: "duzia", QtyReceivedMilli: 5000, QtyLeftMilli: 5000}
	setup := func() *fakeRepo {
		return &fakeRepo{lots: map[uuid.UUID]Lot{lotKg.ID: lotKg, lotDozens.ID: lotDozens}}
	}
	adjust := func(lot Lot, typ MovementType, milli int64) Adjustment {
		return Adjustment{LotID: lot.ID, Type: typ, QuantityMilli: milli, Reason: "  Caixa caiu  "}
	}

	t.Run("o gestor regista uma quebra: o lote desce e o movimento leva o motivo e o autor", func(t *testing.T) {
		repo := setup()
		lot, mv, err := newSvc(repo).Adjust(ctx, gestor, adjust(lotKg, MovementBreakage, -1500))
		if err != nil {
			t.Fatal(err)
		}
		if lot.QtyLeftMilli != 3000 || mv.QuantityMilli != -1500 || mv.Type != MovementBreakage {
			t.Errorf("lote %+v, movimento %+v", lot, mv)
		}
		if m := repo.moves[0]; m.UserID != gestor.UserID || m.Reason != "Caixa caiu" {
			t.Errorf("movimento gravado: %+v", m)
		}
	})

	t.Run("só o gestor; sem sessão também não", func(t *testing.T) {
		repo := setup()
		for _, who := range []identity.Principal{funcionario, {}, {Role: identity.RoleGestor}} {
			if _, _, err := newSvc(repo).Adjust(ctx, who, adjust(lotKg, MovementBreakage, -1000)); !errors.Is(err, identity.ErrForbidden) {
				t.Errorf("%+v: queria ErrForbidden, deu %v", who, err)
			}
		}
		if len(repo.moves) != 0 {
			t.Error("gravou um movimento sem permissão")
		}
	})

	t.Run("o stock nunca fica negativo", func(t *testing.T) {
		repo := setup()
		var short *InsufficientStockError
		if _, _, err := newSvc(repo).Adjust(ctx, gestor, adjust(lotKg, MovementAdjust, -4501)); !errors.As(err, &short) {
			t.Errorf("queria InsufficientStockError, deu %v", err)
		}
		if len(repo.moves) != 0 || repo.lots[lotKg.ID].QtyLeftMilli != 4500 {
			t.Error("o lote mudou")
		}
	})

	t.Run("meia dúzia é recusada antes de chegar ao lote", func(t *testing.T) {
		repo := setup()
		var ve *ValidationError
		if _, _, err := newSvc(repo).Adjust(ctx, gestor, adjust(lotDozens, MovementBreakage, -500)); !errors.As(err, &ve) {
			t.Errorf("queria ValidationError, deu %v", err)
		}
		if len(repo.moves) != 0 {
			t.Error("gravou o movimento")
		}
	})

	t.Run("lote inexistente", func(t *testing.T) {
		if _, _, err := newSvc(setup()).Adjust(ctx, gestor, adjust(Lot{ID: uuid.New()}, MovementBreakage, -1000)); !errors.Is(err, ErrLotNotFound) {
			t.Errorf("queria ErrLotNotFound, deu %v", err)
		}
	})

	t.Run("os lotes do produto: funcionário e gestor, a partir de hoje em Lisboa", func(t *testing.T) {
		repo := setup()
		for _, who := range []identity.Principal{funcionario, gestor} {
			if _, err := newSvc(repo).ListLots(ctx, who, kg.ID); err != nil {
				t.Fatal(err)
			}
		}
		if repo.listedFor != kg.ID || !repo.listedDay.Equal(today) {
			t.Errorf("perguntou por %v em %v", repo.listedFor, repo.listedDay)
		}
		if _, err := newSvc(repo).ListLots(ctx, identity.Principal{}, kg.ID); !errors.Is(err, identity.ErrForbidden) {
			t.Errorf("sem sessão: %v", err)
		}
	})
}
