//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// TestPublicCatalog prova o catálogo público de ponta a ponta (#11): sem
// sessão, só produtos ativos e públicos, só os campos permitidos, com 60 s
// de cache e a disponibilidade calculada a partir do stock real (#12).
func TestPublicCatalog(t *testing.T) {
	e := newEnv(t)
	e.createUser(t, "gestor@quinta.pt", identity.RoleGestor, pw)
	srv := e.server(t)
	token, _ := login(t, srv, "gestor@quinta.pt")
	gestor := bearer(token)

	fruta := call(t, srv, "POST", "/categories", map[string]string{"name": "Fruta"}, gestor).body["id"]
	horta := call(t, srv, "POST", "/categories", map[string]string{"name": "Hortícolas"}, gestor).body["id"]

	create := func(category any, name string, mutate func(map[string]any)) string {
		t.Helper()
		in := product(category, name)
		if mutate != nil {
			mutate(in)
		}
		r := call(t, srv, "POST", "/products", in, gestor)
		if r.status != http.StatusCreated {
			t.Fatalf("criar %s: %d %v", name, r.status, r.body)
		}
		return r.body["id"].(string)
	}
	tomate := create(horta, "Tomate", nil)
	laranja := create(fruta, "Laranja", nil)
	create(fruta, "Banana", func(in map[string]any) { in["active"] = false })
	create(fruta, "Marmelo", func(in map[string]any) { in["isPublic"] = false })
	abacate := create(fruta, "Abacate", func(in map[string]any) { in["seasonMonths"] = []int{} })

	// Stock (o limiar de stock baixo é 2,5 kg): a laranja tem 10 kg em dois
	// lotes; o tomate tem 1 kg num lote que expira hoje (ainda conta).
	for _, l := range []map[string]any{
		lot(laranja, 6000, lisbonDay(-1), lisbonDay(5)),
		lot(laranja, 4000, lisbonDay(0), lisbonDay(9)),
		lot(tomate, 1000, lisbonDay(0), lisbonDay(0)),
	} {
		if r := call(t, srv, "POST", "/stock/lots", l, gestor); r.status != http.StatusCreated {
			t.Fatalf("registar stock: %d %v", r.status, r.body)
		}
	}
	// O abacate só tem um lote fora de validade e um lote vazio: não contam.
	// A API nunca aceita um lote já fora de validade, por isso vão direto à
	// base de dados.
	for _, stmt := range []string{
		"INSERT INTO stock_lots (product_id, qty_received, qty_left, harvested_on, expires_on) VALUES ($1, 50, 50, $2::date - 10, $2::date - 1)",
		"INSERT INTO stock_lots (product_id, qty_received, qty_left, harvested_on, expires_on) VALUES ($1, 50, 0, $2::date, $2::date + 30)",
	} {
		if _, err := e.pool.Exec(context.Background(), stmt, abacate, lisbonDay(0)); err != nil {
			t.Fatal(err)
		}
	}

	// Imagem na laranja: a versão tem de aparecer no catálogo.
	if status, _, _ := raw(t, srv, "PUT", "/products/"+laranja+"/image", photo(t, 600, 400), gestor); status != http.StatusOK {
		t.Fatalf("imagem: %d", status)
	}

	status, header, body := raw(t, srv, "GET", "/public/catalog", nil, nil) // sem sessão
	if status != http.StatusOK {
		t.Fatalf("catálogo público: %d %s", status, body)
	}

	t.Run("cache de 60 s, em vez do no-store por omissão", func(t *testing.T) {
		if got := header.Get("Cache-Control"); got != "public, max-age=60" {
			t.Errorf("Cache-Control: %q", got)
		}
		if !strings.HasPrefix(header.Get("Content-Type"), "application/json") {
			t.Errorf("Content-Type: %q", header.Get("Content-Type"))
		}
	})

	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		t.Fatal(err)
	}

	t.Run("só produtos ativos e públicos, por categoria e nome", func(t *testing.T) {
		var names []string
		for _, it := range items {
			names = append(names, it["name"].(string))
		}
		if want := []string{"Abacate", "Laranja", "Tomate"}; !slices.Equal(names, want) {
			t.Errorf("produtos: %v, queria %v", names, want)
		}
	})

	t.Run("só os campos permitidos, nunca quantidades nem campos internos", func(t *testing.T) {
		allowed := []string{"id", "name", "categoryName", "unit", "priceCents", "seasonMonths", "image", "availability"}
		for _, it := range items {
			for k := range it {
				if !slices.Contains(allowed, k) {
					t.Errorf("%s: campo %q não devia sair no catálogo público", it["name"], k)
				}
			}
		}
		// Reforço: o texto cru não leva nada que cheire a stock ou a dados internos.
		for _, forbidden := range []string{"lowStock", "vatRate", "isPublic", "createdAt", "updatedAt", "quantity", "stock\""} {
			if strings.Contains(string(body), forbidden) {
				t.Errorf("a resposta contém %q", forbidden)
			}
		}
	})

	t.Run("campos de um produto: preço, época, imagem e disponibilidade", func(t *testing.T) {
		byName := map[string]map[string]any{}
		for _, it := range items {
			byName[it["name"].(string)] = it
		}
		l := byName["Laranja"]
		if l["id"] != laranja || l["categoryName"] != "Fruta" || l["unit"] != "kg" || l["priceCents"] != float64(120) {
			t.Errorf("laranja: %v", l)
		}
		if got := l["seasonMonths"].([]any); len(got) != 3 {
			t.Errorf("época: %v", got)
		}
		img, _ := l["image"].(map[string]any)
		if v, _ := img["version"].(string); len(v) != 64 {
			t.Errorf("versão da imagem: %v", l["image"])
		}
		if _, ok := byName["Tomate"]["image"]; ok {
			t.Errorf("um produto sem imagem não devia ter o campo image")
		}
		// Disponibilidade a partir dos lotes dentro da validade.
		for name, want := range map[string]string{"Laranja": "disponivel", "Tomate": "pouco", "Abacate": "esgotado"} {
			if got := byName[name]["availability"]; got != want {
				t.Errorf("%s: disponibilidade %v, queria %s", name, got, want)
			}
		}
		if got := byName["Abacate"]["seasonMonths"].([]any); len(got) != 0 {
			t.Errorf("todo o ano devia ser lista vazia: %v", got)
		}
	})

	t.Run("a imagem do catálogo abre sem sessão", func(t *testing.T) {
		v := items[1]["image"].(map[string]any)["version"].(string)
		if status, _, _ := raw(t, srv, "GET", "/public/products/"+laranja+"/image?size=thumb&v="+v, nil, nil); status != http.StatusOK {
			t.Errorf("miniatura: %d", status)
		}
	})

	t.Run("esconder um produto tira-o do catálogo", func(t *testing.T) {
		got := call(t, srv, "GET", "/products/"+laranja, nil, gestor)
		in := product(fruta, "Laranja")
		in["isPublic"] = false
		if r := call(t, srv, "PUT", "/products/"+laranja, in, with(gestor, "If-Match", got.header.Get("ETag"))); r.status != http.StatusOK {
			t.Fatalf("esconder: %d %v", r.status, r.body)
		}
		_, _, body := raw(t, srv, "GET", "/public/catalog", nil, nil)
		if strings.Contains(string(body), "Laranja") {
			t.Errorf("a laranja escondida continua no catálogo: %s", body)
		}
	})
}
