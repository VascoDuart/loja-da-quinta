package sales

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

var (
	tomato = Product{ID: uuid.New(), Name: "Tomate", Unit: "kg", Decimal: true, Active: true, PriceCents: 349, VatRate: 6}
	eggs   = Product{ID: uuid.New(), Name: "Ovos", Unit: "duzia", Active: true, PriceCents: 280, VatRate: 6}
	wine   = Product{ID: uuid.New(), Name: "Vinho", Unit: "l", Decimal: true, Active: true, PriceCents: 100, VatRate: 23}
	clerk  = identity.Principal{UserID: uuid.New(), Role: identity.RoleFuncionario}
)

func catalogOf(ps ...Product) map[uuid.UUID]Product {
	m := make(map[uuid.UUID]Product, len(ps))
	for _, p := range ps {
		m[p.ID] = p
	}
	return m
}

func TestLineTotalRoundsHalfUpPerLine(t *testing.T) {
	cases := []struct {
		price, milli, want int64
	}{
		{349, 2500, 873}, // 872,5 cents
		{349, 2499, 872}, // 872,151
		{1, 500, 1},      // exactly half a cent goes up
		{1, 499, 0},      // just under half goes down
		{280, 3000, 840}, // whole units
		{10_000_000, 100_000_000, 1_000_000_000_000}, // the largest line still fits
	}
	for _, c := range cases {
		if got := LineTotalCents(c.price, c.milli); got != c.want {
			t.Errorf("LineTotalCents(%d, %d) = %d, want %d", c.price, c.milli, got, c.want)
		}
	}
}

func TestVatIncludedInThePrice(t *testing.T) {
	cases := []struct {
		total int64
		rate  int
		want  int64
	}{
		{873, 6, 49},  // 49,42
		{100, 23, 19}, // 18,70
		{113, 13, 13}, // exactly 13
		{0, 23, 0},
		{1, 23, 0}, // 0,19
		{3, 23, 1}, // 0,56
	}
	for _, c := range cases {
		if got := VatIncludedCents(c.total, c.rate); got != c.want {
			t.Errorf("VatIncludedCents(%d, %d) = %d, want %d", c.total, c.rate, got, c.want)
		}
	}
}

func TestPriceSumsRoundedLines(t *testing.T) {
	in := NewSale{Payment: PaymentCash, Items: []ItemInput{
		{ProductID: tomato.ID, QuantityMilli: 2500},
		{ProductID: eggs.ID, QuantityMilli: 2000},
		{ProductID: wine.ID, QuantityMilli: 1},
	}}.Normalized()
	d, err := Price(in, catalogOf(tomato, eggs, wine))
	if err != nil {
		t.Fatal(err)
	}
	// 873 + 560 + 0 (0,1 cent rounds to 0): the total is the sum of the
	// rounded lines, never a rounding of the sum.
	if d.TotalCents != 1433 {
		t.Errorf("total %d, want 1433", d.TotalCents)
	}
	if d.VatCents != 49+32 {
		t.Errorf("VAT %d, want 81", d.VatCents)
	}
	if d.Lines[0].ProductName != "Ovos" || d.Lines[2].ProductName != "Vinho" {
		t.Errorf("lines not sorted by name: %+v", d.Lines)
	}
}

func TestPriceRejects(t *testing.T) {
	inactive := tomato
	inactive.Active = false
	cases := map[string]struct {
		item     ItemInput
		products map[uuid.UUID]Product
	}{
		"missing product":    {ItemInput{ProductID: uuid.New(), QuantityMilli: 1000}, catalogOf(tomato)},
		"inactive product":   {ItemInput{ProductID: tomato.ID, QuantityMilli: 1000}, catalogOf(inactive)},
		"decimals in dozens": {ItemInput{ProductID: eggs.ID, QuantityMilli: 1500}, catalogOf(eggs)},
	}
	for name, c := range cases {
		_, err := Price(NewSale{Payment: PaymentCash, Items: []ItemInput{c.item}}, c.products)
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: want ValidationError, got %v", name, err)
		}
	}
}

func TestNormalizedMergesProductsAndFingerprintIgnoresOrder(t *testing.T) {
	a := NewSale{Payment: PaymentMBWay, Items: []ItemInput{
		{ProductID: tomato.ID, QuantityMilli: 1000}, {ProductID: eggs.ID, QuantityMilli: 1000}, {ProductID: tomato.ID, QuantityMilli: 500},
	}}.Normalized()
	b := NewSale{Payment: PaymentMBWay, Items: []ItemInput{
		{ProductID: eggs.ID, QuantityMilli: 1000}, {ProductID: tomato.ID, QuantityMilli: 1500},
	}}.Normalized()
	if len(a.Items) != 2 {
		t.Fatalf("want 2 merged lines, got %+v", a.Items)
	}
	if !bytes.Equal(a.Fingerprint(), b.Fingerprint()) {
		t.Error("the same cart in another order has another fingerprint")
	}
	c := b
	c.Payment = PaymentCash
	if bytes.Equal(b.Fingerprint(), c.Fingerprint()) {
		t.Error("another payment has the same fingerprint")
	}
}

func TestValidate(t *testing.T) {
	bad := map[string]NewSale{
		"no payment":    {Items: []ItemInput{{ProductID: tomato.ID, QuantityMilli: 1}}},
		"no items":      {Payment: PaymentCash},
		"zero quantity": {Payment: PaymentCash, Items: []ItemInput{{ProductID: tomato.ID, QuantityMilli: 0}}},
		// Two lines within the limit that add up to more than it.
		"merged over the limit": {Payment: PaymentCash, Items: []ItemInput{
			{ProductID: tomato.ID, QuantityMilli: MaxQuantityMilli}, {ProductID: tomato.ID, QuantityMilli: 1},
		}},
	}
	for name, in := range bad {
		var ve *ValidationError
		if err := in.Normalized().Validate(); !errors.As(err, &ve) {
			t.Errorf("%s: want ValidationError, got %v", name, err)
		}
	}
}

// fakeRepo records sales in memory, keyed by user and idempotency key.
type fakeRepo struct {
	byKey   map[string]Sale
	hashes  map[string][]byte
	creates int
	// createErr, when set, is returned by Create instead of recording.
	createErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byKey: map[string]Sale{}, hashes: map[string][]byte{}}
}

func (f *fakeRepo) Create(_ context.Context, r NewRecord) (Sale, error) {
	f.creates++
	if f.createErr != nil {
		return Sale{}, f.createErr
	}
	k := r.UserID.String() + "/" + r.IdempotencyKey
	if _, ok := f.byKey[k]; ok {
		return Sale{}, errKeyTaken
	}
	s := Sale{ID: uuid.New(), Number: "V-000001", UserID: r.UserID, Payment: r.Draft.Payment, Status: StatusCompleted,
		TotalCents: r.Draft.TotalCents, VatCents: r.Draft.VatCents, Lines: r.Draft.Lines}
	f.byKey[k], f.hashes[k] = s, r.RequestHash
	return s, nil
}

func (f *fakeRepo) SaleByKey(_ context.Context, userID uuid.UUID, key string) (Sale, []byte, error) {
	k := userID.String() + "/" + key
	s, ok := f.byKey[k]
	if !ok {
		return Sale{}, nil, ErrSaleNotFound
	}
	return s, f.hashes[k], nil
}

type fakeProducts map[uuid.UUID]Product

func (f fakeProducts) ProductsByID(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]Product, error) {
	out := map[uuid.UUID]Product{}
	for _, id := range ids {
		if p, ok := f[id]; ok {
			out[id] = p
		}
	}
	return out, nil
}

const key = "kkkkkkkkkkkkkkkkkkkk"

func oneTomato() NewSale {
	return NewSale{Payment: PaymentCash, Items: []ItemInput{{ProductID: tomato.ID, QuantityMilli: 1000}}}
}

func TestRegisterRefusesWithoutRole(t *testing.T) {
	svc := NewService(newFakeRepo(), fakeProducts(catalogOf(tomato)))
	if _, _, err := svc.Register(context.Background(), identity.Principal{UserID: uuid.New()}, key, oneTomato()); !errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestRegisterReplaysTheSameCart(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, fakeProducts(catalogOf(tomato)))
	ctx := context.Background()
	first, created, err := svc.Register(ctx, clerk, key, oneTomato())
	if err != nil || !created {
		t.Fatalf("first: created=%v err=%v", created, err)
	}
	// The retry must not depend on the catalog of now (the product may have
	// been deactivated since).
	svc.products = fakeProducts{}
	again, created, err := svc.Register(ctx, clerk, key, oneTomato())
	if err != nil || created || again.ID != first.ID {
		t.Fatalf("retry: created=%v err=%v id=%v want %v", created, err, again.ID, first.ID)
	}
	if repo.creates != 1 {
		t.Errorf("Create called %d times, want 1", repo.creates)
	}
}

func TestRegisterRefusesAKeyReusedForAnotherCart(t *testing.T) {
	svc := NewService(newFakeRepo(), fakeProducts(catalogOf(tomato)))
	ctx := context.Background()
	if _, _, err := svc.Register(ctx, clerk, key, oneTomato()); err != nil {
		t.Fatal(err)
	}
	other := oneTomato()
	other.Items[0].QuantityMilli = 2000
	if _, _, err := svc.Register(ctx, clerk, key, other); !errors.Is(err, ErrKeyReused) {
		t.Fatalf("want ErrKeyReused, got %v", err)
	}
}

func TestRegisterNamesTheProductThatFallsShort(t *testing.T) {
	repo := newFakeRepo()
	repo.createErr = &InsufficientStockError{ProductID: tomato.ID, AvailableMilli: 500}
	svc := NewService(repo, fakeProducts(catalogOf(tomato)))
	_, _, err := svc.Register(context.Background(), clerk, key, oneTomato())
	var short *InsufficientStockError
	if !errors.As(err, &short) || short.ProductName != "Tomate" || short.Unit != "kg" {
		t.Fatalf("want the shortage of Tomate in kg, got %v %+v", err, short)
	}
}
