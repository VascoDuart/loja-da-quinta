//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

func saleBody(payment string, items ...map[string]any) map[string]any {
	return map[string]any{"payment": payment, "items": items}
}

func item(productID string, milli int64) map[string]any {
	return map[string]any{"productId": productID, "quantityMilli": milli}
}

func withKey(h map[string]string, key string) map[string]string {
	return with(h, "Idempotency-Key", key)
}

// TestSales proves counter sales (#15, UC-06) against a real PostgreSQL:
// prices from the catalog, FEFO stock outflow in the sale's transaction,
// nothing recorded on a shortage, idempotency, and no negative stock or
// deadlocks with concurrent sales.
func TestSales(t *testing.T) {
	e := newEnv(t)
	e.createUser(t, "gestor@quinta.pt", identity.RoleGestor, pw)
	clerk := e.createUser(t, "funcionario@quinta.pt", identity.RoleFuncionario, pw)
	srv := e.server(t)
	gestorToken, _ := login(t, srv, "gestor@quinta.pt")
	clerkToken, _ := login(t, srv, "funcionario@quinta.pt")
	gestor, funcionario := bearer(gestorToken), bearer(clerkToken)
	ctx := context.Background()

	category := call(t, srv, "POST", "/categories", map[string]string{"name": "Horta"}, gestor).body["id"]
	create := func(name, unit string, priceCents int, vat int) string {
		t.Helper()
		in := product(category, name)
		in["unit"], in["priceCents"], in["vatRate"], in["lowStockMilli"] = unit, priceCents, vat, 0
		r := call(t, srv, "POST", "/products", in, gestor)
		if r.status != http.StatusCreated {
			t.Fatalf("create %s: %d %v", name, r.status, r.body)
		}
		return r.body["id"].(string)
	}
	newLot := func(productID string, milli int64, expiresIn int) string {
		t.Helper()
		r := call(t, srv, "POST", "/stock/lots", lot(productID, milli, lisbonDay(0), lisbonDay(expiresIn)), funcionario)
		if r.status != http.StatusCreated {
			t.Fatalf("lot: %d %v", r.status, r.body)
		}
		return r.body["id"].(string)
	}
	left := func(lotID string) int64 {
		t.Helper()
		var milli int64
		if err := e.pool.QueryRow(ctx, "SELECT (qty_left * 1000)::bigint FROM stock_lots WHERE id = $1", lotID).Scan(&milli); err != nil {
			t.Fatal(err)
		}
		return milli
	}
	count := func(sql string, args ...any) int {
		t.Helper()
		var n int
		if err := e.pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	// totals counts what a sale leaves behind: sales, lines and movements.
	totals := func() [3]int {
		t.Helper()
		return [3]int{
			count("SELECT count(*) FROM sales"),
			count("SELECT count(*) FROM sale_items"),
			count("SELECT count(*) FROM stock_movements WHERE type = 'venda'"),
		}
	}

	tomato := create("Tomate", "kg", 349, 6)
	eggs := create("Ovos", "duzia", 280, 6)
	soon := newLot(tomato, 1000, 2)
	later := newLot(tomato, 5000, 5)
	eggLot := newLot(eggs, 10_000, 4)
	// An expired lot (the API does not register those) is never sold, even
	// though it would come first in FEFO order.
	var expired string
	if err := e.pool.QueryRow(ctx, `
		INSERT INTO stock_lots (product_id, unit, qty_received, qty_left, harvested_on, expires_on)
		VALUES ($1, 'kg', 3, 3, $2::date - 5, $2::date - 1) RETURNING id`, tomato, lisbonDay(0)).Scan(&expired); err != nil {
		t.Fatal(err)
	}

	t.Run("a sale takes stock FEFO and records lines, movements and author", func(t *testing.T) {
		// Prices sent by the client are ignored: the contract does not even
		// accept them (additionalProperties: false), so none is sent here.
		r := call(t, srv, "POST", "/sales", saleBody("mbway", item(tomato, 2000), item(eggs, 1000), item(tomato, 500)),
			withKey(funcionario, "sale-fefo-000000001"))
		if r.status != http.StatusCreated {
			t.Fatalf("sale: %d %v", r.status, r.body)
		}
		// Tomate 2,5 kg × 3,49 € = 8,725 → 8,73 € (VAT 6 %: 0,49 €);
		// Ovos 1 dúzia = 2,80 € (VAT 0,16 €).
		if r.body["totalCents"] != float64(873+280) || r.body["vatCents"] != float64(49+16) {
			t.Errorf("totals: %v / %v", r.body["totalCents"], r.body["vatCents"])
		}
		if n, _ := r.body["number"].(string); !strings.HasPrefix(n, "V-") || r.body["status"] != "concluida" {
			t.Errorf("number %v, status %v", r.body["number"], r.body["status"])
		}
		if items, _ := r.body["items"].([]any); len(items) != 2 {
			t.Errorf("want 2 merged lines, got %v", r.body["items"])
		}
		if got := [3]int64{left(soon), left(later), left(expired)}; got != [3]int64{0, 3500, 3000} {
			t.Errorf("lots after the sale (soon, later, expired): %v", got)
		}
		if left(eggLot) != 9000 {
			t.Errorf("eggs left %d", left(eggLot))
		}
		id := r.body["id"].(string)
		if n := count("SELECT count(*) FROM sales WHERE id = $1 AND user_id = $2 AND payment = 'mbway'", id, clerk.ID); n != 1 {
			t.Error("the sale does not record who made it")
		}
		if n := count("SELECT count(*) FROM stock_movements WHERE sale_id = $1 AND type = 'venda' AND user_id = $2", id, clerk.ID); n != 3 {
			t.Errorf("want 3 sale movements (2 tomato lots, 1 egg lot), got %d", n)
		}
		if n := count("SELECT count(*) FROM sale_items WHERE sale_id = $1 AND unit_price_cents = 349 AND quantity = 2.5", id); n != 1 {
			t.Error("the tomato line does not keep the price and quantity of the sale")
		}
	})

	t.Run("the same key never records the sale twice", func(t *testing.T) {
		body := saleBody("numerario", item(eggs, 2000))
		first := call(t, srv, "POST", "/sales", body, withKey(funcionario, "sale-retry-00000001"))
		if first.status != http.StatusCreated {
			t.Fatalf("first: %d %v", first.status, first.body)
		}
		before := totals()
		again := call(t, srv, "POST", "/sales", body, withKey(funcionario, "sale-retry-00000001"))
		if again.status != http.StatusOK || again.body["id"] != first.body["id"] || again.body["number"] != first.body["number"] {
			t.Fatalf("retry: %d %v", again.status, again.body)
		}
		other := call(t, srv, "POST", "/sales", saleBody("numerario", item(eggs, 3000)), withKey(funcionario, "sale-retry-00000001"))
		if other.status != http.StatusUnprocessableEntity {
			t.Errorf("another cart with the same key: %d %v", other.status, other.body)
		}
		if totals() != before {
			t.Errorf("a retry recorded something: %v → %v", before, totals())
		}
		// The key belongs to the user: the manager may use the same one.
		if r := call(t, srv, "POST", "/sales", body, withKey(gestor, "sale-retry-00000001")); r.status != http.StatusCreated {
			t.Errorf("same key, another user: %d %v", r.status, r.body)
		}
	})

	t.Run("a double click at the same time records one sale", func(t *testing.T) {
		before := totals()
		body := saleBody("numerario", item(eggs, 1000))
		a := goCall(srv, "POST", "/sales", body, withKey(funcionario, "sale-double-click-01"))
		b := goCall(srv, "POST", "/sales", body, withKey(funcionario, "sale-double-click-01"))
		ra, rb := receive(t, a), receive(t, b)
		got := map[int]int{ra.status: 1}
		got[rb.status]++
		if got[http.StatusCreated] != 1 || got[http.StatusOK] != 1 || ra.body["id"] != rb.body["id"] {
			t.Fatalf("want one 201 and one 200 for the same sale: %d %v / %d %v", ra.status, ra.body, rb.status, rb.body)
		}
		if after := totals(); after[0] != before[0]+1 || after[2] != before[2]+1 {
			t.Errorf("want one more sale and movement: %v → %v", before, after)
		}
	})

	t.Run("insufficient stock: nothing is recorded and the product is named", func(t *testing.T) {
		before := totals()
		lots := [3]int64{left(later), left(eggLot), left(expired)}
		// The eggs line comes first in id order or not: either way nothing
		// stays recorded.
		r := call(t, srv, "POST", "/sales", saleBody("numerario", item(eggs, 1000), item(tomato, 4000)),
			withKey(funcionario, "sale-short-00000001"))
		if r.status != http.StatusConflict {
			t.Fatalf("want 409, got %d %v", r.status, r.body)
		}
		if d, _ := r.body["detail"].(string); !strings.Contains(d, "Tomate") || !strings.Contains(d, "3,5 kg") {
			t.Errorf("the message does not name the product and what is left: %q", d)
		}
		if totals() != before || [3]int64{left(later), left(eggLot), left(expired)} != lots {
			t.Errorf("something was recorded: %v → %v", before, totals())
		}
		// The key was not used up: the corrected cart goes through with it.
		r = call(t, srv, "POST", "/sales", saleBody("numerario", item(eggs, 1000), item(tomato, 3500)),
			withKey(funcionario, "sale-short-00000001"))
		if r.status != http.StatusCreated {
			t.Errorf("corrected cart: %d %v", r.status, r.body)
		}
	})

	t.Run("an inactive product is not sold", func(t *testing.T) {
		honey := create("Mel", "un", 500, 6)
		newLot(honey, 5000, 30)
		version := call(t, srv, "GET", "/products/"+honey, nil, gestor).header.Get("ETag")
		in := product(category, "Mel")
		in["unit"], in["priceCents"], in["lowStockMilli"], in["active"] = "un", 500, 0, false
		if r := call(t, srv, "PUT", "/products/"+honey, in, with(gestor, "If-Match", version)); r.status != http.StatusOK {
			t.Fatalf("deactivate: %d %v", r.status, r.body)
		}
		r := call(t, srv, "POST", "/sales", saleBody("numerario", item(honey, 1000)), withKey(funcionario, "sale-inactive-00001"))
		if r.status != http.StatusUnprocessableEntity {
			t.Errorf("want 422, got %d %v", r.status, r.body)
		}
	})

	t.Run("race: two sales of the last lot, one goes through", func(t *testing.T) {
		lettuce := create("Alface", "un", 90, 6)
		last := newLot(lettuce, 1000, 3)
		before := totals()
		// Hold the lot so both sales are surely waiting on it at once.
		tx, err := e.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := tx.Exec(ctx, "SELECT 1 FROM stock_lots WHERE id = $1 FOR UPDATE", last); err != nil {
			t.Fatal(err)
		}
		a := goCall(srv, "POST", "/sales", saleBody("numerario", item(lettuce, 1000)), withKey(funcionario, "sale-race-a-0000001"))
		b := goCall(srv, "POST", "/sales", saleBody("mbway", item(lettuce, 1000)), withKey(gestor, "sale-race-b-0000001"))
		waitForLockWaits(t, e.pool, 2)
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		ra, rb := receive(t, a), receive(t, b)
		got := map[int]int{ra.status: 1}
		got[rb.status]++
		if got[http.StatusCreated] != 1 || got[http.StatusConflict] != 1 {
			t.Fatalf("want one 201 and one 409: %d %v / %d %v", ra.status, ra.body, rb.status, rb.body)
		}
		if left(last) != 0 {
			t.Errorf("lot left %d, want 0", left(last))
		}
		if after := totals(); after[0] != before[0]+1 || after[2] != before[2]+1 {
			t.Errorf("want exactly one more sale and movement: %v → %v", before, after)
		}
	})

	t.Run("concurrency: sales of two products in both orders never deadlock", func(t *testing.T) {
		pear := create("Pera", "un", 150, 6)
		apple := create("Maçã", "un", 120, 6)
		newLot(pear, 6000, 3)
		newLot(apple, 6000, 3)
		const n = 10
		pending := make([]<-chan response, n)
		for i := range n {
			items := []map[string]any{item(pear, 1000), item(apple, 1000)}
			if i%2 == 1 {
				items[0], items[1] = items[1], items[0]
			}
			pending[i] = goCall(srv, "POST", "/sales", saleBody("numerario", items...),
				withKey(funcionario, "sale-order-"+strings.Repeat("x", 8)+string(rune('a'+i))))
		}
		got := map[int]int{}
		for _, ch := range pending {
			got[receive(t, ch).status]++
		}
		if got[http.StatusCreated] != 6 || got[http.StatusConflict] != 4 {
			t.Errorf("want 6 sales and 4 shortages, got %v", got)
		}
	})
}

// waitForLockWaits waits until at least n connections are waiting on a lock.
func waitForLockWaits(t *testing.T, pool *pgxpool.Pool, n int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := pool.QueryRow(context.Background(), `
			SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting >= n {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("fewer than %d requests ever waited on the lock", n)
}
