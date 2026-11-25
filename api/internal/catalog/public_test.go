package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestAvailabilityOf(t *testing.T) {
	cases := []struct {
		name         string
		stock, limit int64
		want         Availability
	}{
		{"sem stock", 0, 2500, SoldOut},
		{"stock negativo (erro de dados) conta como esgotado", -1, 2500, SoldOut},
		{"abaixo do limiar", 2499, 2500, Low},
		{"no limiar não é pouco", 2500, 2500, Available},
		{"acima do limiar", 9000, 2500, Available},
		{"limiar 0: nunca pouco", 1, 0, Available},
		{"limiar 0 e sem stock: esgotado", 0, 0, SoldOut},
	}
	for _, c := range cases {
		if got := AvailabilityOf(c.stock, c.limit); got != c.want {
			t.Errorf("%s: queria %q, deu %q", c.name, c.want, got)
		}
	}
}

type publicRepo struct {
	Repo
	products []Product
	err      error
}

func (r *publicRepo) ListPublicProducts(context.Context) ([]Product, error) { return r.products, r.err }

type fakeStock struct {
	levels map[uuid.UUID]int64
	err    error
	asked  []uuid.UUID
}

func (f *fakeStock) StockMilli(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	f.asked = ids
	return f.levels, f.err
}

func publicProduct(name string, low int64) Product {
	return Product{
		ProductInput: ProductInput{Name: name, Unit: UnitKg, PriceCents: 250, VatRate: 6, SeasonMonths: []int{1}, LowStockMilli: low, IsPublic: true, Active: true},
		ID:           uuid.New(), CategoryName: "Fruta", ImageVersion: "abc",
	}
}

// Sem o stock ligado (WithStock), nenhum produto tem stock: todos aparecem
// esgotados, nunca disponíveis por engano.
func TestPublicCatalogWithoutStockIsSoldOut(t *testing.T) {
	repo := &publicRepo{products: []Product{publicProduct("Maçã", 5000), publicProduct("Pera", 0)}}
	list, err := NewService(repo, &fakeImages{}).PublicCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("queria 2 produtos, deu %d", len(list))
	}
	for _, p := range list {
		if p.Availability != SoldOut {
			t.Errorf("%s: queria esgotado, deu %q", p.Name, p.Availability)
		}
	}
}

// O stock real decide esgotado, pouco e disponível; um produto que não vem
// no mapa não tem stock.
func TestPublicCatalogUsesTheStockSource(t *testing.T) {
	soldOut, low, fine, missing := publicProduct("A", 2000), publicProduct("B", 2000), publicProduct("C", 2000), publicProduct("D", 2000)
	repo := &publicRepo{products: []Product{soldOut, low, fine, missing}}
	stock := &fakeStock{levels: map[uuid.UUID]int64{soldOut.ID: 0, low.ID: 1999, fine.ID: 2000}}
	list, err := NewService(repo, &fakeImages{}).WithStock(stock).PublicCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []Availability{SoldOut, Low, Available, SoldOut}
	for i, p := range list {
		if p.Availability != want[i] {
			t.Errorf("%s: queria %q, deu %q", p.Name, want[i], p.Availability)
		}
	}
	if len(stock.asked) != 4 {
		t.Errorf("o stock devia ser pedido de uma vez para os 4 produtos, foi %d", len(stock.asked))
	}
	if list[0].ImageVersion != "abc" || list[0].PriceCents != 250 || list[0].CategoryName != "Fruta" {
		t.Errorf("o DTO perdeu campos: %+v", list[0])
	}
}

func TestPublicCatalogPropagatesErrors(t *testing.T) {
	boom := errors.New("falhou")
	if _, err := NewService(&publicRepo{err: boom}, &fakeImages{}).PublicCatalog(context.Background()); !errors.Is(err, boom) {
		t.Errorf("erro do repositório: %v", err)
	}
	repo := &publicRepo{products: []Product{publicProduct("A", 0)}}
	if _, err := NewService(repo, &fakeImages{}).WithStock(&fakeStock{err: boom}).PublicCatalog(context.Background()); !errors.Is(err, boom) {
		t.Errorf("erro do stock: %v", err)
	}
}
