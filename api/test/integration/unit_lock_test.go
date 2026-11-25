//go:build integration

package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
	"github.com/VascoDuart/loja-da-quinta/api/migrations"
)

// goCall faz um pedido numa goroutine e entrega a resposta no canal. Não
// recebe o *testing.T (t.Fatal só pode ser chamado na goroutine do teste): um
// erro de rede chega como status 0.
func goCall(srv *httptest.Server, method, path string, body any, headers map[string]string) <-chan response {
	out := make(chan response, 1)
	go func() {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req, err := http.NewRequestWithContext(context.Background(), method, srv.URL+"/api/v1"+path, &buf)
		if err != nil {
			out <- response{}
			return
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			out <- response{}
			return
		}
		defer func() { _ = res.Body.Close() }()
		var b map[string]any
		_ = json.NewDecoder(res.Body).Decode(&b)
		out <- response{status: res.StatusCode, header: res.Header, body: b}
	}()
	return out
}

// waitForLockWait espera até haver uma ligação parada à espera de um
// bloqueio de linha: é a prova de que o pedido concorrente chegou à base de
// dados e está à espera da transação do teste.
func waitForLockWait(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		err := pool.QueryRow(context.Background(), `
			SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("o pedido concorrente nunca ficou à espera do bloqueio")
}

func receive(t *testing.T, ch <-chan response) response {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(15 * time.Second):
		t.Fatal("o pedido não respondeu")
		return response{}
	}
}

// TestUnitOfAProductWithLots prova que a unidade de um produto com lotes não
// muda (#13): as quantidades dos lotes estão nessa unidade. A verificação do
// serviço dá a mensagem; a chave estrangeira (product_id, unit) da migração
// 00005 garante a regra quando um lote e a mudança de unidade chegam ao
// mesmo tempo, em qualquer das ordens.
func TestUnitOfAProductWithLots(t *testing.T) {
	e := newEnv(t)
	e.createUser(t, "gestor@quinta.pt", identity.RoleGestor, pw)
	e.createUser(t, "funcionario@quinta.pt", identity.RoleFuncionario, pw)
	srv := e.server(t)
	gestorToken, _ := login(t, srv, "gestor@quinta.pt")
	funcToken, _ := login(t, srv, "funcionario@quinta.pt")
	gestor, funcionario := bearer(gestorToken), bearer(funcToken)
	ctx := context.Background()

	fruta := call(t, srv, "POST", "/categories", map[string]string{"name": "Fruta"}, gestor).body["id"]
	create := func(name string) string {
		t.Helper()
		r := call(t, srv, "POST", "/products", product(fruta, name), gestor)
		if r.status != http.StatusCreated {
			t.Fatalf("criar %s: %d %v", name, r.status, r.body)
		}
		return r.body["id"].(string)
	}
	// toUnits é o pedido de edição que passa o produto de kg para unidades,
	// com a versão atual (If-Match).
	toUnits := func(id, name string) (map[string]any, map[string]string) {
		t.Helper()
		cur := call(t, srv, "GET", "/products/"+id, nil, gestor)
		in := product(fruta, name)
		in["unit"], in["lowStockMilli"] = "un", 2000
		return in, with(gestor, "If-Match", cur.header.Get("ETag"))
	}
	unitOf := func(id string) string {
		t.Helper()
		return call(t, srv, "GET", "/products/"+id, nil, gestor).body["unit"].(string)
	}
	lotsOf := func(id string) int {
		t.Helper()
		var n int
		if err := e.pool.QueryRow(ctx, "SELECT count(*) FROM stock_lots WHERE product_id = $1", id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	t.Run("com lotes: a unidade não muda (409), o resto muda", func(t *testing.T) {
		laranja := create("Laranja")
		if r := call(t, srv, "POST", "/stock/lots", lot(laranja, 12_500, lisbonDay(0), lisbonDay(3)), funcionario); r.status != http.StatusCreated {
			t.Fatalf("registar: %d %v", r.status, r.body)
		}
		in, h := toUnits(laranja, "Laranja")
		r := call(t, srv, "PUT", "/products/"+laranja, in, h)
		if r.status != http.StatusConflict {
			t.Fatalf("queria 409, deu %d %v", r.status, r.body)
		}
		if detail, _ := r.body["detail"].(string); detail == "" {
			t.Errorf("o gestor devia ter a explicação: %v", r.body)
		}
		if got := unitOf(laranja); got != "kg" {
			t.Errorf("a unidade mudou para %s", got)
		}

		// Mudar o preço, com a mesma unidade, continua a ser possível.
		cur := call(t, srv, "GET", "/products/"+laranja, nil, gestor)
		price := product(fruta, "Laranja")
		price["priceCents"] = 150
		if r := call(t, srv, "PUT", "/products/"+laranja, price, with(gestor, "If-Match", cur.header.Get("ETag"))); r.status != http.StatusOK {
			t.Errorf("mudar o preço: %d %v", r.status, r.body)
		}
	})

	t.Run("sem lotes: a unidade muda", func(t *testing.T) {
		pera := create("Pera")
		in, h := toUnits(pera, "Pera")
		if r := call(t, srv, "PUT", "/products/"+pera, in, h); r.status != http.StatusOK {
			t.Fatalf("queria 200, deu %d %v", r.status, r.body)
		}
		if got := unitOf(pera); got != "un" {
			t.Errorf("a unidade devia ser un, é %s", got)
		}
	})

	t.Run("corrida: o lote entra primeiro, a mudança de unidade espera e é recusada", func(t *testing.T) {
		kiwi := create("Kiwi")
		// O lote ainda por confirmar: o serviço do catálogo não o vê, e a
		// verificação dele deixa passar. Quem trava é a base de dados.
		tx, err := e.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_lots (product_id, unit, qty_received, qty_left, harvested_on, expires_on)
			VALUES ($1, 'kg', 12.5, 12.5, current_date, current_date + 3)`, kiwi); err != nil {
			t.Fatal(err)
		}

		in, h := toUnits(kiwi, "Kiwi")
		pending := goCall(srv, "PUT", "/products/"+kiwi, in, h)
		waitForLockWait(t, e.pool)
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}

		if r := receive(t, pending); r.status != http.StatusConflict {
			t.Errorf("queria 409, deu %d %v", r.status, r.body)
		}
		if got := unitOf(kiwi); got != "kg" {
			t.Errorf("a unidade mudou para %s com um lote em kg", got)
		}
	})

	t.Run("corrida: a unidade muda primeiro, o lote validado em kg espera e é recusado", func(t *testing.T) {
		ameixa := create("Ameixa")
		tx, err := e.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := tx.Exec(ctx, "UPDATE products SET unit = 'un', low_stock = 2 WHERE id = $1", ameixa); err != nil {
			t.Fatal(err)
		}

		// O serviço lê o produto ainda em kg e aceita 12,5; a gravação espera.
		pending := goCall(srv, "POST", "/stock/lots", lot(ameixa, 12_500, lisbonDay(0), lisbonDay(3)), funcionario)
		waitForLockWait(t, e.pool)
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}

		r := receive(t, pending)
		if r.status != http.StatusConflict {
			t.Fatalf("queria 409, deu %d %v", r.status, r.body)
		}
		if detail, _ := r.body["detail"].(string); detail == "" {
			t.Errorf("o funcionário devia ter a explicação: %v", r.body)
		}
		if n := lotsOf(ameixa); n != 0 {
			t.Errorf("entrou um lote de 12,5 numa unidade inteira (%d lotes)", n)
		}
	})

	t.Run("os lotes guardam a unidade do produto", func(t *testing.T) {
		var mismatched int
		if err := e.pool.QueryRow(ctx, `
			SELECT count(*) FROM stock_lots l JOIN products p ON p.id = l.product_id
			WHERE l.unit <> p.unit`).Scan(&mismatched); err != nil {
			t.Fatal(err)
		}
		if mismatched != 0 {
			t.Errorf("%d lotes numa unidade diferente da do produto", mismatched)
		}
	})
}

// TestLotUnitMigrationFillsExistingLots confirma que a migração 00005 dá aos
// lotes que já existiam (staging) a unidade do produto deles.
func TestLotUnitMigrationFillsExistingLots(t *testing.T) {
	url := startPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := migrations.UpTo(ctx, sqlDB, 4); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		"INSERT INTO categories (id, name) VALUES ('10000000-0000-4000-8000-000000000001', 'Ovos')",
		`INSERT INTO products (id, category_id, name, unit, price_cents, vat_rate, season_months, low_stock, is_public, active)
		 VALUES ('20000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000001', 'Ovos', 'duzia', 300, 6, '{}', 2, true, true)`,
		`INSERT INTO stock_lots (product_id, qty_received, qty_left, harvested_on, expires_on)
		 VALUES ('20000000-0000-4000-8000-000000000001', 3, 3, current_date, current_date + 3)`,
	} {
		if _, err := sqlDB.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := migrations.Up(ctx, sqlDB); err != nil {
		t.Fatal(err)
	}
	var unit string
	if err := sqlDB.QueryRowContext(ctx, "SELECT unit FROM stock_lots").Scan(&unit); err != nil {
		t.Fatal(err)
	}
	if unit != "duzia" {
		t.Errorf("o lote antigo devia ficar em duzia, ficou em %q", unit)
	}
	if _, err := sqlDB.ExecContext(ctx, "UPDATE products SET unit = 'un'"); err == nil {
		t.Error("a base de dados devia recusar mudar a unidade de um produto com lotes")
	}
}
