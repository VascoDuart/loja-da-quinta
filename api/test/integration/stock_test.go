//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/VascoDuart/loja-da-quinta/api/internal/inventory"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
	"github.com/VascoDuart/loja-da-quinta/api/migrations"
)

// lisbonDay devolve o dia de hoje em Lisboa mais n dias, como "2026-11-25".
func lisbonDay(n int) string {
	return inventory.Day(time.Now()).AddDate(0, 0, n).Format(time.DateOnly)
}

func lot(productID any, milli int64, harvested, expires string) map[string]any {
	return map[string]any{"productId": productID, "quantityMilli": milli, "harvestedOn": harvested, "expiresOn": expires}
}

// TestStockLots prova a entrada de stock por lotes (#12) contra um PostgreSQL
// real: o lote e o movimento ficam gravados, com o autor, e as regras de
// quantidade e de datas são as do servidor.
func TestStockLots(t *testing.T) {
	e := newEnv(t)
	e.createUser(t, "gestor@quinta.pt", identity.RoleGestor, pw)
	ana := e.createUser(t, "funcionario@quinta.pt", identity.RoleFuncionario, pw)
	srv := e.server(t)
	gestorToken, _ := login(t, srv, "gestor@quinta.pt")
	funcToken, _ := login(t, srv, "funcionario@quinta.pt")
	gestor, funcionario := bearer(gestorToken), bearer(funcToken)
	ctx := context.Background()

	fruta := call(t, srv, "POST", "/categories", map[string]string{"name": "Fruta"}, gestor).body["id"]
	create := func(name string, mutate func(map[string]any)) string {
		t.Helper()
		in := product(fruta, name)
		if mutate != nil {
			mutate(in)
		}
		r := call(t, srv, "POST", "/products", in, gestor)
		if r.status != http.StatusCreated {
			t.Fatalf("criar %s: %d %v", name, r.status, r.body)
		}
		return r.body["id"].(string)
	}
	laranja := create("Laranja", nil)
	ovos := create("Ovos", func(in map[string]any) { in["unit"], in["lowStockMilli"] = "duzia", 0 })
	marmelo := create("Marmelo", func(in map[string]any) { in["active"] = false })

	t.Run("o funcionário regista um lote: lote e movimento de entrada com o autor", func(t *testing.T) {
		r := call(t, srv, "POST", "/stock/lots", lot(laranja, 12_500, lisbonDay(-1), lisbonDay(7)), funcionario)
		if r.status != http.StatusCreated {
			t.Fatalf("registar: %d %v", r.status, r.body)
		}
		b := r.body
		if b["productId"] != laranja || b["quantityReceivedMilli"] != float64(12_500) || b["quantityLeftMilli"] != float64(12_500) {
			t.Errorf("lote: %v", b)
		}
		if b["harvestedOn"] != lisbonDay(-1) || b["expiresOn"] != lisbonDay(7) || b["createdAt"] == nil {
			t.Errorf("datas: %v", b)
		}

		var (
			typ      string
			quantity string
			userID   string
			left     string
		)
		err := e.pool.QueryRow(ctx, `
			SELECT m.type, m.quantity::text, m.user_id::text, l.qty_left::text
			FROM stock_movements m JOIN stock_lots l ON l.id = m.lot_id
			WHERE l.id = $1`, b["id"]).Scan(&typ, &quantity, &userID, &left)
		if err != nil {
			t.Fatal(err)
		}
		if typ != "entrada" || quantity != "12.500" || left != "12.500" || userID != ana.ID.String() {
			t.Errorf("movimento: %s %s por %s (lote com %s)", typ, quantity, userID, left)
		}
	})

	t.Run("o gestor também regista; uma dúzia inteira é aceite", func(t *testing.T) {
		if r := call(t, srv, "POST", "/stock/lots", lot(ovos, 3000, lisbonDay(0), lisbonDay(0)), gestor); r.status != http.StatusCreated {
			t.Errorf("registar: %d %v", r.status, r.body)
		}
	})

	t.Run("regras do servidor (422)", func(t *testing.T) {
		cases := map[string]map[string]any{
			"produto inexistente":        lot("00000000-0000-4000-8000-000000000000", 1000, lisbonDay(0), lisbonDay(3)),
			"produto desativado":         lot(marmelo, 1000, lisbonDay(0), lisbonDay(3)),
			"meia dúzia":                 lot(ovos, 500, lisbonDay(0), lisbonDay(3)),
			"colheita amanhã":            lot(laranja, 1000, lisbonDay(1), lisbonDay(3)),
			"validade antes da colheita": lot(laranja, 1000, lisbonDay(-1), lisbonDay(-2)),
			"validade já passou":         lot(laranja, 1000, lisbonDay(-5), lisbonDay(-1)),
		}
		for name, body := range cases {
			r := call(t, srv, "POST", "/stock/lots", body, funcionario)
			if r.status != http.StatusUnprocessableEntity {
				t.Errorf("%s: queria 422, deu %d %v", name, r.status, r.body)
				continue
			}
			if detail, _ := r.body["detail"].(string); detail == "" {
				t.Errorf("%s: o erro devia explicar o problema: %v", name, r.body)
			}
		}
	})

	t.Run("regras do contrato (400)", func(t *testing.T) {
		extra := lot(laranja, 1000, lisbonDay(0), lisbonDay(3))
		extra["userId"] = ana.ID.String() // o autor vem sempre da sessão
		cases := map[string]any{
			"quantidade 0":        lot(laranja, 0, lisbonDay(0), lisbonDay(3)),
			"quantidade absurda":  lot(laranja, 100_000_001, lisbonDay(0), lisbonDay(3)),
			"data noutro formato": lot(laranja, 1000, "25/11/2026", lisbonDay(3)),
			"campo a mais":        extra,
			"sem validade":        map[string]any{"productId": laranja, "quantityMilli": 1000, "harvestedOn": lisbonDay(0)},
		}
		for name, body := range cases {
			if r := call(t, srv, "POST", "/stock/lots", body, funcionario); r.status != http.StatusBadRequest {
				t.Errorf("%s: queria 400, deu %d %v", name, r.status, r.body)
			}
		}
	})

	t.Run("nada fica gravado pela metade: um lote por cada movimento", func(t *testing.T) {
		var lots, movements int
		_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM stock_lots").Scan(&lots)
		_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM stock_movements").Scan(&movements)
		if lots != 2 || movements != 2 {
			t.Errorf("queria 2 lotes e 2 movimentos, há %d e %d", lots, movements)
		}
	})

	t.Run("um produto com stock não se apaga (409)", func(t *testing.T) {
		if r := call(t, srv, "DELETE", "/products/"+laranja, nil, gestor); r.status != http.StatusConflict {
			t.Errorf("queria 409, deu %d %v", r.status, r.body)
		}
		if r := call(t, srv, "GET", "/products/"+laranja, nil, gestor); r.status != http.StatusOK {
			t.Errorf("o produto devia continuar lá: %d", r.status)
		}
	})

	t.Run("os movimentos são só de acrescento, mesmo para o dono das tabelas", func(t *testing.T) {
		for _, stmt := range []string{
			"UPDATE stock_movements SET quantity = 1",
			"DELETE FROM stock_movements",
			"TRUNCATE stock_movements",
		} {
			_, err := e.pool.Exec(ctx, stmt)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
				t.Errorf("%s: devia ser recusado (42501), deu %v", stmt, err)
			}
		}
	})

	t.Run("a base de dados recusa stock negativo e movimentos com o sinal errado", func(t *testing.T) {
		if _, err := e.pool.Exec(ctx, "UPDATE stock_lots SET qty_left = -1"); err == nil {
			t.Error("qty_left negativo devia ser recusado")
		}
		var lotID string
		_ = e.pool.QueryRow(ctx, "SELECT id FROM stock_lots LIMIT 1").Scan(&lotID)
		for _, typ := range []string{"entrada", "anulacao"} {
			if _, err := e.pool.Exec(ctx, "INSERT INTO stock_movements (lot_id, type, quantity, user_id) VALUES ($1, $2, -1, $3)", lotID, typ, ana.ID); err == nil {
				t.Errorf("%s negativa devia ser recusada", typ)
			}
		}
		for _, typ := range []string{"venda", "quebra"} {
			if _, err := e.pool.Exec(ctx, "INSERT INTO stock_movements (lot_id, type, quantity, user_id) VALUES ($1, $2, 1, $3)", lotID, typ, ana.ID); err == nil {
				t.Errorf("%s positiva devia ser recusada", typ)
			}
		}
	})
}

// TestStockMovementsPrivileges confirma que a migração deixa o papel app (o
// da API) só com INSERT e SELECT nos movimentos. O PostgreSQL dos testes não
// tem o papel: cria-se aqui com os privilégios por omissão de
// infra/postgres/init-roles.sh, antes de aplicar as migrações.
func TestStockMovementsPrivileges(t *testing.T) {
	url := startPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sqlDB.Close() }()

	for _, stmt := range []string{
		"CREATE ROLE app",
		"ALTER DEFAULT PRIVILEGES GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app",
	} {
		if _, err := sqlDB.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := migrations.Up(ctx, sqlDB); err != nil {
		t.Fatal(err)
	}

	want := map[string]map[string]bool{
		"stock_movements": {"SELECT": true, "INSERT": true, "UPDATE": false, "DELETE": false, "TRUNCATE": false},
		// Os lotes mudam (vendas, ajustes): o app precisa de UPDATE.
		"stock_lots": {"SELECT": true, "INSERT": true, "UPDATE": true},
	}
	for table, privs := range want {
		for priv, expected := range privs {
			var has bool
			if err := sqlDB.QueryRowContext(ctx, "SELECT has_table_privilege('app', $1, $2)", table, priv).Scan(&has); err != nil {
				t.Fatal(err)
			}
			if has != expected {
				t.Errorf("app em %s, %s: queria %v, deu %v", table, priv, expected, has)
			}
		}
	}
}
