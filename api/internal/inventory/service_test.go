package inventory

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestDayIsTheDayInLisbon(t *testing.T) {
	cases := []struct {
		name string
		at   time.Time
		want time.Time
	}{
		// Hora de verão (UTC+1): às 23:30 UTC já é o dia seguinte em Lisboa.
		{"verão, antes da meia-noite UTC", time.Date(2026, 7, 1, 23, 30, 0, 0, time.UTC), day(2026, 7, 2)},
		{"verão, de manhã", time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC), day(2026, 7, 1)},
		// Hora de inverno (UTC+0): o dia é o mesmo do UTC.
		{"inverno, antes da meia-noite UTC", time.Date(2026, 12, 31, 23, 30, 0, 0, time.UTC), day(2026, 12, 31)},
		{"outro fuso", time.Date(2026, 11, 25, 20, 0, 0, 0, time.FixedZone("UTC-5", -5*3600)), day(2026, 11, 26)},
	}
	for _, c := range cases {
		if got := Day(c.at); !got.Equal(c.want) {
			t.Errorf("%s: queria %v, deu %v", c.name, c.want, got)
		}
	}
}

var (
	today  = day(2026, 11, 25)
	kg     = Product{ID: uuid.New(), Name: "Laranja", Unit: "kg", Decimal: true, Active: true}
	dozens = Product{ID: uuid.New(), Name: "Ovos", Unit: "duzia", Decimal: false, Active: true}
)

func validLot() NewLot {
	return NewLot{ProductID: kg.ID, QuantityMilli: 12_500, HarvestedOn: day(2026, 11, 24), ExpiresOn: day(2026, 12, 2)}
}

func TestLotValidation(t *testing.T) {
	if err := validLot().Validate(kg, today); err != nil {
		t.Fatalf("um lote válido foi recusado: %v", err)
	}
	ok := map[string]func(*NewLot, *Product){
		"colhido hoje":               func(l *NewLot, _ *Product) { l.HarvestedOn = today },
		"expira hoje":                func(l *NewLot, _ *Product) { l.HarvestedOn, l.ExpiresOn = today, today },
		"uma dúzia":                  func(l *NewLot, p *Product) { *p = dozens; l.QuantityMilli = 1000 },
		"1 g":                        func(l *NewLot, _ *Product) { l.QuantityMilli = 1 },
		"o máximo":                   func(l *NewLot, _ *Product) { l.QuantityMilli = MaxLotMilli },
		"colhido há meses (compota)": func(l *NewLot, _ *Product) { l.HarvestedOn = day(2026, 6, 1) },
	}
	for name, mutate := range ok {
		t.Run("aceita "+name, func(t *testing.T) {
			l, p := validLot(), kg
			mutate(&l, &p)
			if err := l.Validate(p, today); err != nil {
				t.Errorf("devia aceitar: %v", err)
			}
		})
	}
	bad := map[string]func(*NewLot, *Product){
		"quantidade 0":                func(l *NewLot, _ *Product) { l.QuantityMilli = 0 },
		"quantidade negativa":         func(l *NewLot, _ *Product) { l.QuantityMilli = -1000 },
		"quantidade absurda":          func(l *NewLot, _ *Product) { l.QuantityMilli = MaxLotMilli + 1 },
		"meia dúzia":                  func(l *NewLot, p *Product) { *p = dozens; l.QuantityMilli = 500 },
		"produto desativado":          func(_ *NewLot, p *Product) { p.Active = false },
		"colheita amanhã":             func(l *NewLot, _ *Product) { l.HarvestedOn = day(2026, 11, 26) },
		"validade antes da colheita":  func(l *NewLot, _ *Product) { l.ExpiresOn = day(2026, 11, 23) },
		"validade já passou (ontem)":  func(l *NewLot, _ *Product) { l.HarvestedOn, l.ExpiresOn = day(2026, 11, 20), day(2026, 11, 24) },
		"colheita e validade futuras": func(l *NewLot, _ *Product) { l.HarvestedOn, l.ExpiresOn = day(2026, 12, 1), day(2026, 12, 9) },
	}
	for name, mutate := range bad {
		t.Run("recusa "+name, func(t *testing.T) {
			l, p := validLot(), kg
			mutate(&l, &p)
			var ve *ValidationError
			if err := l.Validate(p, today); !errors.As(err, &ve) {
				t.Errorf("queria ValidationError, deu %v", err)
			}
		})
	}
}

func TestNormalizedKeepsTheCalendarDay(t *testing.T) {
	// "2026-11-24" pode chegar com hora ou fuso; o dia do calendário mantém-se.
	l := NewLot{HarvestedOn: time.Date(2026, 11, 24, 23, 0, 0, 0, time.FixedZone("x", 3600)), ExpiresOn: time.Date(2026, 12, 2, 0, 0, 0, 0, time.Local)}
	n := l.Normalized()
	if !n.HarvestedOn.Equal(day(2026, 11, 24)) || !n.ExpiresOn.Equal(day(2026, 12, 2)) {
		t.Errorf("datas: %v %v", n.HarvestedOn, n.ExpiresOn)
	}
}

type fakeRepo struct {
	created []NewLot
	units   []string
	by      []uuid.UUID
	// keyErr faz o CreateLot falhar como a chave (produto, unidade) da base
	// de dados, quando o produto mudou ou desapareceu entretanto.
	keyErr bool
	lotsOf map[uuid.UUID]bool
	// lots são os lotes gravados; Move aplica-lhes os movimentos como o
	// PostgresStore (Lot.LeftAfter), e moves guarda os que passaram.
	lots      map[uuid.UUID]Lot
	moves     []Movement
	listedFor uuid.UUID
	listedDay time.Time
	asked     []uuid.UUID
	today     time.Time
	levels    map[uuid.UUID]int64
	stocked   []Lot
}

func (f *fakeRepo) StockedLots(context.Context) ([]Lot, error) { return f.stocked, nil }

func (f *fakeRepo) CreateLot(_ context.Context, in NewLot, unit string, userID uuid.UUID) (Lot, error) {
	if f.keyErr {
		return Lot{}, errProductKey
	}
	f.created = append(f.created, in)
	f.units = append(f.units, unit)
	f.by = append(f.by, userID)
	return Lot{ID: uuid.New(), ProductID: in.ProductID, Unit: unit, QtyReceivedMilli: in.QuantityMilli, QtyLeftMilli: in.QuantityMilli}, nil
}

func (f *fakeRepo) HasLots(_ context.Context, id uuid.UUID) (bool, error) { return f.lotsOf[id], nil }

func (f *fakeRepo) Lot(_ context.Context, id uuid.UUID) (Lot, error) {
	l, ok := f.lots[id]
	if !ok {
		return Lot{}, ErrLotNotFound
	}
	return l, nil
}

func (f *fakeRepo) ListLots(_ context.Context, productID uuid.UUID, today time.Time) ([]Lot, error) {
	f.listedFor, f.listedDay = productID, today
	return nil, nil
}

func (f *fakeRepo) Move(_ context.Context, m Movement) (Lot, RecordedMovement, error) {
	l, ok := f.lots[m.LotID]
	if !ok {
		return Lot{}, RecordedMovement{}, ErrLotNotFound
	}
	left, err := l.LeftAfter(m.QuantityMilli)
	if err != nil {
		return Lot{}, RecordedMovement{}, err
	}
	l.QtyLeftMilli = left
	f.lots[m.LotID] = l
	f.moves = append(f.moves, m)
	return l, RecordedMovement{ID: int64(len(f.moves)), LotID: l.ID, Type: m.Type, QuantityMilli: m.QuantityMilli, Reason: m.Reason}, nil
}

func (f *fakeRepo) StockMilli(_ context.Context, ids []uuid.UUID, today time.Time) (map[uuid.UUID]int64, error) {
	f.asked, f.today = ids, today
	return f.levels, nil
}

type fakeProducts map[uuid.UUID]Product

func (f fakeProducts) Product(_ context.Context, id uuid.UUID) (Product, error) {
	p, ok := f[id]
	if !ok {
		return Product{}, ErrProductNotFound
	}
	return p, nil
}

func (f fakeProducts) Products(context.Context) ([]Product, error) {
	out := make([]Product, 0, len(f))
	for _, p := range f {
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b Product) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// vanishing devolve o produto na primeira pergunta e depois já não o
// encontra (foi apagado entre a validação e a gravação).
type vanishing struct{ p Product }

func (v *vanishing) Products(context.Context) ([]Product, error) { return nil, nil }

func (v *vanishing) Product(context.Context, uuid.UUID) (Product, error) {
	if v.p.ID == uuid.Nil {
		return Product{}, ErrProductNotFound
	}
	p := v.p
	v.p = Product{}
	return p, nil
}

// clock marca 25 de novembro de 2026, de manhã (hora de inverno em Lisboa).
var clock = func() time.Time { return time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC) }

func newSvc(repo *fakeRepo) *Service {
	return NewService(repo, fakeProducts{kg.ID: kg, dozens.ID: dozens}, clock)
}

func TestRegisterLot(t *testing.T) {
	repo := &fakeRepo{}
	svc := newSvc(repo)
	ctx := context.Background()
	funcionario := identity.Principal{UserID: uuid.New(), Role: identity.RoleFuncionario}
	gestor := identity.Principal{UserID: uuid.New(), Role: identity.RoleGestor}

	t.Run("funcionário e gestor registam; o movimento leva quem registou", func(t *testing.T) {
		for _, who := range []identity.Principal{funcionario, gestor} {
			lot, err := svc.RegisterLot(ctx, who, validLot())
			if err != nil {
				t.Fatal(err)
			}
			if lot.QtyLeftMilli != 12_500 || lot.QtyReceivedMilli != 12_500 {
				t.Errorf("lote: %+v", lot)
			}
		}
		if len(repo.by) != 2 || repo.by[0] != funcionario.UserID || repo.by[1] != gestor.UserID {
			t.Errorf("autores: %v", repo.by)
		}
		if repo.units[0] != "kg" {
			t.Errorf("o lote guarda a unidade em que a quantidade foi validada: %v", repo.units)
		}
	})

	t.Run("a base de dados recusa o par produto e unidade", func(t *testing.T) {
		// O produto ainda existe: mudou de unidade entretanto (409).
		changed := &fakeRepo{keyErr: true}
		if _, err := newSvc(changed).RegisterLot(ctx, funcionario, validLot()); !errors.Is(err, ErrUnitChanged) {
			t.Errorf("queria ErrUnitChanged, deu %v", err)
		}
		// O produto desapareceu entretanto: a pergunta ao catálogo diz qual.
		gone := NewService(&fakeRepo{keyErr: true}, &vanishing{kg}, clock)
		if _, err := gone.RegisterLot(ctx, funcionario, validLot()); !errors.Is(err, ErrProductNotFound) {
			t.Errorf("queria ErrProductNotFound, deu %v", err)
		}
	})

	t.Run("sem sessão ou com papel desconhecido: proibido, nada se grava", func(t *testing.T) {
		before := len(repo.created)
		for _, who := range []identity.Principal{{}, {UserID: uuid.New(), Role: "cliente"}, {Role: identity.RoleGestor}} {
			if _, err := svc.RegisterLot(ctx, who, validLot()); !errors.Is(err, identity.ErrForbidden) {
				t.Errorf("%+v: queria ErrForbidden, deu %v", who, err)
			}
		}
		if len(repo.created) != before {
			t.Error("gravou um lote sem permissão")
		}
	})

	t.Run("produto inexistente", func(t *testing.T) {
		in := validLot()
		in.ProductID = uuid.New()
		if _, err := svc.RegisterLot(ctx, funcionario, in); !errors.Is(err, ErrProductNotFound) {
			t.Errorf("queria ErrProductNotFound, deu %v", err)
		}
	})

	t.Run("hoje vem do relógio, na hora de Lisboa", func(t *testing.T) {
		in := validLot()
		in.HarvestedOn = day(2026, 11, 26) // amanhã, pelo relógio
		var ve *ValidationError
		if _, err := svc.RegisterLot(ctx, funcionario, in); !errors.As(err, &ve) || ve.Field != "harvestedOn" {
			t.Errorf("colheita amanhã: %v", err)
		}
	})

	t.Run("as datas chegam ao repositório como dias", func(t *testing.T) {
		in := validLot()
		in.HarvestedOn = in.HarvestedOn.Add(13 * time.Hour)
		if _, err := svc.RegisterLot(ctx, funcionario, in); err != nil {
			t.Fatal(err)
		}
		if got := repo.created[len(repo.created)-1].HarvestedOn; !got.Equal(day(2026, 11, 24)) {
			t.Errorf("colheita: %v", got)
		}
	})
}

func TestStockMilli(t *testing.T) {
	id := uuid.New()
	repo := &fakeRepo{levels: map[uuid.UUID]int64{id: 4500}}
	svc := newSvc(repo)

	got, err := svc.StockMilli(context.Background(), []uuid.UUID{id})
	if err != nil || got[id] != 4500 {
		t.Fatalf("stock: %v %v", got, err)
	}
	if !repo.today.Equal(day(2026, 11, 25)) {
		t.Errorf("os lotes contam a partir de hoje em Lisboa, deu %v", repo.today)
	}

	repo.asked = nil
	if got, err := svc.StockMilli(context.Background(), nil); err != nil || len(got) != 0 || repo.asked != nil {
		t.Errorf("sem produtos não se pergunta à base de dados: %v %v %v", got, err, repo.asked)
	}
}
