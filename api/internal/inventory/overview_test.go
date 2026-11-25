package inventory

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// lotOf é um lote de p que expira daqui a n dias (hoje é 25/11/2026).
func lotOf(p Product, leftMilli int64, n int) Lot {
	return Lot{
		ID: uuid.New(), ProductID: p.ID, Unit: p.Unit,
		QtyReceivedMilli: leftMilli, QtyLeftMilli: leftMilli,
		HarvestedOn: today.AddDate(0, 0, -10), ExpiresOn: today.AddDate(0, 0, n),
	}
}

func product(name, unit string, lowMilli int64, active bool) Product {
	return Product{ID: uuid.New(), Name: name, Unit: unit, Decimal: unit == "kg" || unit == "l", Active: active, LowStockMilli: lowMilli}
}

func TestLotStatus(t *testing.T) {
	cases := map[int]LotStatus{
		-30: LotExpired, -1: LotExpired,
		0: LotExpiringSoon, 3: LotExpiringSoon, // hoje ainda se vende, e é urgente
		4: LotValid, 90: LotValid,
	}
	for days, want := range cases {
		if got := StatusOf(today.AddDate(0, 0, days), today); got != want {
			t.Errorf("validade a %d dias: queria %s, deu %s", days, want, got)
		}
	}
}

func TestBuildOverview(t *testing.T) {
	laranja := product("Laranja", "kg", 5000, true)
	ovos := product("Ovos", "duzia", 2000, true)
	salsa := product("Salsa", "molho", 3000, true)
	mel := product("Mel", "un", 0, true)
	figo := product("Figo", "kg", 5000, false)
	noz := product("Noz", "kg", 5000, false)

	o := BuildOverview(
		[]Product{figo, laranja, mel, noz, ovos, salsa},
		[]Lot{
			lotOf(laranja, 4000, -1), lotOf(laranja, 2000, 2), lotOf(laranja, 2500, 20),
			lotOf(ovos, 2000, 4),
			lotOf(figo, 1000, -3),
		},
		today,
	)

	if !o.Today.Equal(today) || !o.ExpiringUntil.Equal(day(2026, 11, 28)) {
		t.Errorf("dias: %v %v", o.Today, o.ExpiringUntil)
	}
	var names []string
	got := map[string]ProductStock{}
	for _, p := range o.Products {
		names = append(names, p.Product.Name)
		got[p.Product.Name] = p
	}
	// Os desativados sem lotes (Noz) não interessam; com lotes (Figo), sim:
	// o stock está lá e é preciso tratá-lo.
	if want := "Figo,Laranja,Mel,Ovos,Salsa"; strings.Join(names, ",") != want {
		t.Fatalf("produtos: %v, queria %s", names, want)
	}

	check := func(name string, sellable int64, low bool, statuses ...LotStatus) {
		t.Helper()
		p := got[name]
		if p.SellableMilli != sellable || p.Low != low {
			t.Errorf("%s: vendável %d (queria %d), baixo %v (queria %v)", name, p.SellableMilli, sellable, p.Low, low)
		}
		if len(p.Lots) != len(statuses) {
			t.Fatalf("%s: %d lotes, queria %d", name, len(p.Lots), len(statuses))
		}
		for i, s := range statuses {
			if p.Lots[i].Status != s {
				t.Errorf("%s, lote %d: %s, queria %s", name, i, p.Lots[i].Status, s)
			}
		}
	}
	// O lote fora de validade não conta: 2 + 2,5 = 4,5 kg, abaixo de 5 kg.
	check("Laranja", 4500, true, LotExpired, LotExpiringSoon, LotValid)
	check("Ovos", 2000, false, LotValid) // no limiar não é "abaixo"
	check("Salsa", 0, true)              // ativo sem lotes
	check("Mel", 0, false)               // limiar 0 nunca dá stock baixo
	check("Figo", 0, false, LotExpired)  // desativado nunca dá stock baixo
}

func TestReportAndMessage(t *testing.T) {
	laranja := product("Laranja", "kg", 5000, true)
	ovos := product("Ovos", "duzia", 0, true)
	abobora := product("Abóbora", "un", 0, true)
	r := BuildOverview(
		[]Product{abobora, laranja, ovos},
		[]Lot{lotOf(laranja, 1500, -1), lotOf(laranja, 2000, 20), lotOf(ovos, 3000, 2), lotOf(abobora, 1000, 0)},
		today,
	).Report()

	if len(r.Low) != 1 || len(r.Expiring) != 2 || len(r.Expired) != 1 || r.Empty() {
		t.Fatalf("relatório: %+v", r)
	}
	// Primeiro o que expira primeiro.
	if r.Expiring[0].Product.Name != "Abóbora" || r.Expiring[1].Product.Name != "Ovos" {
		t.Errorf("ordem dos lotes a expirar: %+v", r.Expiring)
	}

	subject, text := r.Message()
	if want := "Loja da Quinta: alertas de stock de 25/11/2026"; subject != want {
		t.Errorf("assunto: %q", subject)
	}
	for _, want := range []string{
		"Stock baixo (1)\n- Laranja: 2 kg para vender (limiar: 5 kg)",
		"A expirar até 28/11/2026 (2)\n- Abóbora: 1 unidade, validade 25/11/2026\n- Ovos: 3 dúzias, validade 27/11/2026",
		"Fora de validade, para dar como quebra (1)\n- Laranja: 1,5 kg, validade 24/11/2026",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("o texto devia ter %q:\n%s", want, text)
		}
	}

	t.Run("sem nada a avisar", func(t *testing.T) {
		empty := BuildOverview([]Product{ovos}, []Lot{lotOf(ovos, 3000, 20)}, today).Report()
		if !empty.Empty() {
			t.Errorf("devia estar vazio: %+v", empty)
		}
	})

	t.Run("só as secções com alguma coisa", func(t *testing.T) {
		_, text := BuildOverview([]Product{ovos}, []Lot{lotOf(ovos, 3000, 1)}, today).Report().Message()
		if strings.Contains(text, "Stock baixo") || strings.Contains(text, "Fora de validade") {
			t.Errorf("secções vazias:\n%s", text)
		}
	})
}

func TestOverviewIsForStaffOnly(t *testing.T) {
	laranja := product("Laranja", "kg", 5000, true)
	repo := &fakeRepo{stocked: []Lot{lotOf(laranja, 2000, 1)}}
	svc := NewService(repo, fakeProducts{laranja.ID: laranja}, clock)
	ctx := context.Background()

	for _, role := range []identity.Role{identity.RoleFuncionario, identity.RoleGestor} {
		o, err := svc.Overview(ctx, identity.Principal{UserID: uuid.New(), Role: role})
		if err != nil || len(o.Products) != 1 || o.Products[0].SellableMilli != 2000 {
			t.Errorf("%s: %+v %v", role, o, err)
		}
	}
	for _, who := range []identity.Principal{{}, {UserID: uuid.New(), Role: "cliente"}} {
		if _, err := svc.Overview(ctx, who); !errors.Is(err, identity.ErrForbidden) {
			t.Errorf("%+v: queria ErrForbidden, deu %v", who, err)
		}
	}
}
