//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// raw faz um pedido com um corpo binário (upload de imagem) ou lê um corpo
// binário (imagem pública).
func raw(t *testing.T, srv *httptest.Server, method, path string, body []byte, headers map[string]string) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, srv.URL+"/api/v1"+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header, b
}

func photo(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 80, 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func product(categoryID any, name string) map[string]any {
	return map[string]any{
		"categoryId": categoryID, "name": name, "unit": "kg", "priceCents": 120, "vatRate": 6,
		"seasonMonths": []int{12, 1, 2}, "lowStockMilli": 2500, "isPublic": true, "active": true,
	}
}

func with(h map[string]string, k, v string) map[string]string {
	out := map[string]string{k: v}
	for a, b := range h {
		out[a] = b
	}
	return out
}

func TestCatalog(t *testing.T) {
	e := newEnv(t)
	e.createUser(t, "gestor@quinta.pt", identity.RoleGestor, pw)
	e.createUser(t, "funcionario@quinta.pt", identity.RoleFuncionario, pw)
	srv := e.server(t)
	gestorToken, _ := login(t, srv, "gestor@quinta.pt")
	funcToken, _ := login(t, srv, "funcionario@quinta.pt")
	gestor, funcionario := bearer(gestorToken), bearer(funcToken)

	fruta := call(t, srv, "POST", "/categories", map[string]string{"name": "Fruta"}, gestor)
	if fruta.status != http.StatusCreated {
		t.Fatalf("criar categoria: %d %v", fruta.status, fruta.body)
	}
	frutaID := fruta.body["id"]

	t.Run("categorias: nome repetido sem olhar a maiúsculas, renomear, apagar", func(t *testing.T) {
		if r := call(t, srv, "POST", "/categories", map[string]string{"name": "  FRUTA "}, gestor); r.status != http.StatusConflict {
			t.Errorf("repetida: queria 409, deu %d", r.status)
		}
		legumes := call(t, srv, "POST", "/categories", map[string]string{"name": "Legumes"}, gestor)
		id := legumes.body["id"].(string)
		if r := call(t, srv, "PATCH", "/categories/"+id, map[string]string{"name": "Hortícolas"}, gestor); r.status != http.StatusOK || r.body["name"] != "Hortícolas" {
			t.Errorf("renomear: %d %v", r.status, r.body)
		}
		if r := call(t, srv, "DELETE", "/categories/"+id, nil, gestor); r.status != http.StatusNoContent {
			t.Errorf("apagar vazia: %d", r.status)
		}
		if r := call(t, srv, "PATCH", "/categories/"+id, map[string]string{"name": "X"}, gestor); r.status != http.StatusNotFound {
			t.Errorf("renomear apagada: queria 404, deu %d", r.status)
		}
	})

	var created response
	t.Run("criar produto: ETag, milésimos, meses por ordem", func(t *testing.T) {
		created = call(t, srv, "POST", "/products", product(frutaID, "  Laranja   do Algarve "), gestor)
		if created.status != http.StatusCreated {
			t.Fatalf("criar: %d %v", created.status, created.body)
		}
		b := created.body
		if b["name"] != "Laranja do Algarve" || b["categoryName"] != "Fruta" || b["lowStockMilli"] != float64(2500) {
			t.Errorf("produto: %v", b)
		}
		if fmt.Sprint(b["seasonMonths"]) != "[1 2 12]" {
			t.Errorf("meses: %v", b["seasonMonths"])
		}
		if created.header.Get("ETag") != `"`+b["version"].(string)+`"` {
			t.Errorf("o ETag devia ser a versão: %q vs %v", created.header.Get("ETag"), b["version"])
		}
		if _, ok := b["image"]; ok {
			t.Error("sem imagem, o campo image não aparece")
		}
	})
	id := created.body["id"].(string)

	t.Run("criar produto: erros", func(t *testing.T) {
		if r := call(t, srv, "POST", "/products", product(frutaID, "laranja do algarve"), gestor); r.status != http.StatusConflict {
			t.Errorf("nome repetido: queria 409, deu %d", r.status)
		}
		if r := call(t, srv, "POST", "/products", product("00000000-0000-4000-8000-000000000000", "Pera"), gestor); r.status != http.StatusUnprocessableEntity {
			t.Errorf("categoria inexistente: queria 422, deu %d %v", r.status, r.body)
		}
		ovos := product(frutaID, "Ovos")
		ovos["unit"], ovos["lowStockMilli"] = "duzia", 500
		if r := call(t, srv, "POST", "/products", ovos, gestor); r.status != http.StatusUnprocessableEntity {
			t.Errorf("meia dúzia de limiar: queria 422, deu %d", r.status)
		}
		iva := product(frutaID, "Kiwi")
		iva["vatRate"] = 10
		if r := call(t, srv, "POST", "/products", iva, gestor); r.status != http.StatusBadRequest {
			t.Errorf("IVA de 10 %% (contrato): queria 400, deu %d", r.status)
		}
	})

	t.Run("editar com If-Match: a versão certa grava, a antiga dá 412", func(t *testing.T) {
		v1 := created.header.Get("ETag")
		change := product(frutaID, "Laranja do Algarve")
		change["priceCents"] = 150
		r := call(t, srv, "PUT", "/products/"+id, change, with(gestor, "If-Match", v1))
		if r.status != http.StatusOK || r.body["priceCents"] != float64(150) {
			t.Fatalf("editar: %d %v", r.status, r.body)
		}
		if r.header.Get("ETag") == v1 {
			t.Error("a versão tinha de mudar")
		}
		// Outro gestor, com a versão antiga, não apaga a alteração do primeiro.
		change["priceCents"] = 99
		if stale := call(t, srv, "PUT", "/products/"+id, change, with(gestor, "If-Match", v1)); stale.status != http.StatusPreconditionFailed {
			t.Errorf("versão antiga: queria 412, deu %d", stale.status)
		}
		if r := call(t, srv, "PUT", "/products/"+id, change, gestor); r.status != http.StatusBadRequest {
			t.Errorf("sem If-Match (contrato): queria 400, deu %d", r.status)
		}
		got := call(t, srv, "GET", "/products/"+id, nil, funcionario)
		if got.body["priceCents"] != float64(150) {
			t.Errorf("o preço devia continuar 150: %v", got.body["priceCents"])
		}
	})

	t.Run("a categoria com produtos não se apaga", func(t *testing.T) {
		if r := call(t, srv, "DELETE", "/categories/"+frutaID.(string), nil, gestor); r.status != http.StatusConflict {
			t.Errorf("queria 409, deu %d", r.status)
		}
		list := call(t, srv, "GET", "/categories", nil, funcionario)
		if list.status != http.StatusOK {
			t.Fatalf("listar: %d", list.status)
		}
	})

	t.Run("imagem: upload, URL público imutável e versões antigas", func(t *testing.T) {
		status, _, body := raw(t, srv, "PUT", "/products/"+id+"/image", photo(t, 1200, 900), gestor)
		if status != http.StatusOK {
			t.Fatalf("upload: %d %s", status, body)
		}
		var img struct{ Version string }
		_ = json.Unmarshal(body, &img)
		if len(img.Version) != 64 {
			t.Fatalf("versão: %q", img.Version)
		}
		if p := call(t, srv, "GET", "/products/"+id, nil, gestor); fmt.Sprint(p.body["image"]) != fmt.Sprintf("map[version:%s]", img.Version) {
			t.Errorf("o produto devia mostrar a versão da imagem: %v", p.body["image"])
		}

		status, h, jpg := raw(t, srv, "GET", "/public/products/"+id+"/image?size=large&v="+img.Version, nil, nil)
		if status != http.StatusOK || h.Get("Content-Type") != "image/jpeg" {
			t.Fatalf("pública: %d %s", status, h.Get("Content-Type"))
		}
		if h.Get("Cache-Control") != "public, max-age=31536000, immutable" || h.Get("ETag") == "" || h.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("cabeçalhos: %v", h)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(jpg))
		if err != nil || cfg.Width != 800 || cfg.Height != 600 {
			t.Errorf("a versão large tem 800x600: %+v %v", cfg, err)
		}
		if status, _, _ := raw(t, srv, "GET", "/public/products/"+id+"/image?size=thumb&v="+img.Version, nil, nil); status != http.StatusOK {
			t.Errorf("thumb: %d", status)
		}

		// Trocar a imagem: o URL antigo deixa de servir.
		status, _, body = raw(t, srv, "PUT", "/products/"+id+"/image", photo(t, 300, 300), gestor)
		var img2 struct{ Version string }
		_ = json.Unmarshal(body, &img2)
		if status != http.StatusOK || img2.Version == img.Version {
			t.Fatalf("segundo upload: %d, versão %q", status, img2.Version)
		}
		if status, _, _ := raw(t, srv, "GET", "/public/products/"+id+"/image?size=large&v="+img.Version, nil, nil); status != http.StatusNotFound {
			t.Errorf("versão antiga: queria 404, deu %d", status)
		}
		if status, _, _ := raw(t, srv, "GET", "/public/products/"+id+"/image?size=enorme&v="+img2.Version, nil, nil); status != http.StatusBadRequest {
			t.Errorf("tamanho inválido: queria 400, deu %d", status)
		}

		if r := call(t, srv, "DELETE", "/products/"+id+"/image", nil, gestor); r.status != http.StatusNoContent {
			t.Fatalf("apagar imagem: %d", r.status)
		}
		if status, _, _ := raw(t, srv, "GET", "/public/products/"+id+"/image?size=large&v="+img2.Version, nil, nil); status != http.StatusNotFound {
			t.Errorf("depois de apagar: queria 404, deu %d", status)
		}
	})

	t.Run("imagem: ficheiros recusados", func(t *testing.T) {
		svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
		if status, _, _ := raw(t, srv, "PUT", "/products/"+id+"/image", svg, gestor); status != http.StatusUnsupportedMediaType {
			t.Errorf("SVG: queria 415, deu %d", status)
		}
		// Content-Type de imagem não engana: conta o conteúdo.
		fake := with(gestor, "Content-Type", "application/octet-stream")
		if status, _, _ := raw(t, srv, "PUT", "/products/"+id+"/image", []byte("GIF89a..."), fake); status != http.StatusUnsupportedMediaType {
			t.Errorf("GIF: queria 415, deu %d", status)
		}
		big := append(photo(t, 10, 10), bytes.Repeat([]byte{0}, 6<<20)...)
		if status, _, _ := raw(t, srv, "PUT", "/products/"+id+"/image", big, gestor); status != http.StatusRequestEntityTooLarge {
			t.Errorf("6 MB: queria 413, deu %d", status)
		}
		if status, _, _ := raw(t, srv, "PUT", "/products/"+id+"/image", []byte{0xFF, 0xD8, 0xFF, 0x00}, gestor); status != http.StatusUnprocessableEntity {
			t.Errorf("JPEG estragado: queria 422, deu %d", status)
		}
		if status, _, _ := raw(t, srv, "PUT", "/products/00000000-0000-4000-8000-000000000000/image", photo(t, 10, 10), gestor); status != http.StatusNotFound {
			t.Errorf("produto inexistente: queria 404, deu %d", status)
		}
	})

	t.Run("matriz de autorização do catálogo", func(t *testing.T) {
		pid := id
		rows := []struct {
			method, path string
			body         any
			want         map[string]int
		}{
			{"GET", "/categories", nil, map[string]int{"visitante": 401, "funcionario": 200, "gestor": 200}},
			{"POST", "/categories", map[string]string{"name": "Matriz"}, map[string]int{"visitante": 401, "funcionario": 403, "gestor": 201}},
			{"GET", "/products", nil, map[string]int{"visitante": 401, "funcionario": 200, "gestor": 200}},
			{"GET", "/products/" + pid, nil, map[string]int{"visitante": 401, "funcionario": 200, "gestor": 200}},
			{"POST", "/products", product(frutaID, "Matriz"), map[string]int{"visitante": 401, "funcionario": 403, "gestor": 201}},
			{"DELETE", "/products/" + pid + "/image", nil, map[string]int{"visitante": 401, "funcionario": 403, "gestor": 204}},
		}
		tokens := map[string]map[string]string{"visitante": nil, "funcionario": funcionario, "gestor": gestor}
		for _, row := range rows {
			for _, who := range []string{"visitante", "funcionario", "gestor"} {
				if r := call(t, srv, row.method, row.path, row.body, tokens[who]); r.status != row.want[who] {
					t.Errorf("%s %s como %s: queria %d, deu %d", row.method, row.path, who, row.want[who], r.status)
				}
			}
		}
		if status, _, _ := raw(t, srv, "PUT", "/products/"+pid+"/image", photo(t, 10, 10), funcionario); status != http.StatusForbidden {
			t.Errorf("upload como funcionário: queria 403, deu %d", status)
		}
	})

	t.Run("apagar produto sem histórico", func(t *testing.T) {
		if r := call(t, srv, "DELETE", "/products/"+id, nil, gestor); r.status != http.StatusNoContent {
			t.Fatalf("apagar: %d", r.status)
		}
		if r := call(t, srv, "GET", "/products/"+id, nil, gestor); r.status != http.StatusNotFound {
			t.Errorf("depois de apagar: queria 404, deu %d", r.status)
		}
		var images int
		_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM product_images WHERE product_id = $1", id).Scan(&images)
		if images != 0 {
			t.Error("as imagens vão com o produto (ON DELETE CASCADE)")
		}
	})

}
