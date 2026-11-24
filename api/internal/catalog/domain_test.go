package catalog

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

func validInput() ProductInput {
	return ProductInput{
		CategoryID: uuid.New(), Name: "Laranja do Algarve", Unit: UnitKg, PriceCents: 120, VatRate: 6,
		SeasonMonths: []int{12, 1, 2}, LowStockMilli: 2500, IsPublic: true, Active: true,
	}
}

func TestProductValidation(t *testing.T) {
	if err := validInput().Normalized().Validate(); err != nil {
		t.Fatalf("um produto válido foi recusado: %v", err)
	}
	cases := map[string]func(*ProductInput){
		"nome vazio":                  func(p *ProductInput) { p.Name = "" },
		"nome com 101 caracteres":     func(p *ProductInput) { p.Name = string(make([]rune, 101)) },
		"unidade inexistente":         func(p *ProductInput) { p.Unit = "caixa" },
		"preço negativo":              func(p *ProductInput) { p.PriceCents = -1 },
		"preço absurdo":               func(p *ProductInput) { p.PriceCents = MaxPriceCents + 1 },
		"IVA de 10 %":                 func(p *ProductInput) { p.VatRate = 10 },
		"mês 13":                      func(p *ProductInput) { p.SeasonMonths = []int{13} },
		"mês 0":                       func(p *ProductInput) { p.SeasonMonths = []int{0} },
		"mês repetido":                func(p *ProductInput) { p.SeasonMonths = []int{3, 3} },
		"limiar negativo":             func(p *ProductInput) { p.LowStockMilli = -1 },
		"meia dúzia como limiar":      func(p *ProductInput) { p.Unit = UnitDozen; p.LowStockMilli = 500 },
		"unidades com casas decimais": func(p *ProductInput) { p.Unit = UnitUnit; p.LowStockMilli = 1500 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := validInput()
			mutate(&in)
			var ve *ValidationError
			if err := in.Normalized().Validate(); !errors.As(err, &ve) {
				t.Errorf("queria ValidationError, deu %v", err)
			}
		})
	}
}

func TestNormalized(t *testing.T) {
	in := validInput()
	in.Name = "  Laranja   do\tAlgarve "
	in.SeasonMonths = nil
	n := in.Normalized()
	if n.Name != "Laranja do Algarve" {
		t.Errorf("nome = %q", n.Name)
	}
	if n.SeasonMonths == nil || len(n.SeasonMonths) != 0 {
		t.Error("sem meses fica uma lista vazia (todo o ano), nunca nil")
	}
	in.SeasonMonths = []int{12, 1, 2}
	if got := in.Normalized().SeasonMonths; got[0] != 1 || got[2] != 12 {
		t.Errorf("os meses ficam por ordem: %v", got)
	}
}

func TestVersionRoundTrip(t *testing.T) {
	p := Product{UpdatedAt: time.Date(2026, 11, 24, 10, 30, 0, 123456000, time.UTC)}
	for _, header := range []string{p.Version(), `"` + p.Version() + `"`, `W/"` + p.Version() + `"`} {
		got, ok := ParseVersion(header)
		if !ok || !got.Equal(p.UpdatedAt) {
			t.Errorf("ParseVersion(%q) = %v, %v", header, got, ok)
		}
	}
	for _, bad := range []string{"", "*", "abc", `"-1"`} {
		if _, ok := ParseVersion(bad); ok {
			t.Errorf("ParseVersion(%q) devia falhar", bad)
		}
	}
}

// fakeRepo regista as chamadas; só o necessário para testar o serviço.
type fakeRepo struct {
	Repo
	created []ProductInput
	updated time.Time
}

func (f *fakeRepo) CreateProduct(_ context.Context, in ProductInput) (Product, error) {
	f.created = append(f.created, in)
	return Product{ProductInput: in, ID: uuid.New()}, nil
}

func (f *fakeRepo) UpdateProduct(_ context.Context, _ uuid.UUID, expected time.Time, in ProductInput) (Product, error) {
	f.updated = expected
	return Product{ProductInput: in}, nil
}

type fakeImages struct {
	ImageStore
	version []byte
}

func (f *fakeImages) Get(context.Context, uuid.UUID, string) ([]byte, []byte, []byte, error) {
	return []byte("jpeg"), make([]byte, 32), f.version, nil
}

func TestWritesRequireGestor(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeImages{})
	ctx := context.Background()
	funcionario := identity.Principal{UserID: uuid.New(), Role: identity.RoleFuncionario}

	checks := map[string]error{}
	_, checks["criar categoria"] = svc.CreateCategory(ctx, funcionario, "Fruta")
	_, checks["renomear categoria"] = svc.RenameCategory(ctx, funcionario, uuid.New(), "Fruta")
	checks["apagar categoria"] = svc.DeleteCategory(ctx, funcionario, uuid.New())
	_, checks["criar produto"] = svc.CreateProduct(ctx, funcionario, validInput())
	_, checks["editar produto"] = svc.UpdateProduct(ctx, funcionario, uuid.New(), "1", validInput())
	checks["apagar produto"] = svc.DeleteProduct(ctx, funcionario, uuid.New())
	_, checks["trocar imagem"] = svc.ReplaceImage(ctx, funcionario, uuid.New(), nil)
	checks["apagar imagem"] = svc.DeleteImage(ctx, funcionario, uuid.New())
	for name, err := range checks {
		if !errors.Is(err, identity.ErrForbidden) {
			t.Errorf("%s: queria ErrForbidden, deu %v", name, err)
		}
	}
}

func TestUpdateRequiresAValidVersion(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, &fakeImages{})
	gestor := identity.Principal{UserID: uuid.New(), Role: identity.RoleGestor}
	if _, err := svc.UpdateProduct(context.Background(), gestor, uuid.New(), "*", validInput()); !errors.Is(err, ErrVersionMismatch) {
		t.Errorf("If-Match sem versão válida: %v", err)
	}
	if _, err := svc.UpdateProduct(context.Background(), gestor, uuid.New(), `"1764000000000000"`, validInput()); err != nil {
		t.Fatal(err)
	}
	if repo.updated.UnixMicro() != 1764000000000000 {
		t.Errorf("a versão chega ao repositório: %v", repo.updated)
	}
}

func TestPublicImageNeedsCurrentVersion(t *testing.T) {
	current := make([]byte, 32)
	current[0] = 0xAB
	svc := NewService(&fakeRepo{}, &fakeImages{version: current})
	ctx := context.Background()

	if _, _, err := svc.PublicImage(ctx, uuid.New(), "thumb", hex.EncodeToString(current)); err != nil {
		t.Errorf("a versão atual devia servir a imagem: %v", err)
	}
	old := make([]byte, 32)
	for name, v := range map[string]string{"versão antiga": hex.EncodeToString(old), "não é hex": "zz", "curta": "ab"} {
		if _, _, err := svc.PublicImage(ctx, uuid.New(), "thumb", v); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: queria ErrNotFound, deu %v", name, err)
		}
	}
}
