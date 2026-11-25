//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

func adjustment(lotID any, typ string, milli int64, reason string) map[string]any {
	return map[string]any{"lotId": lotID, "type": typ, "quantityMilli": milli, "reason": reason}
}

// TestStockAdjustments prova os ajustes e as quebras (#13, RF-06) contra um
// PostgreSQL real: só o gestor, com motivo, e o stock de um lote nunca fica
// negativo, também com pedidos ao mesmo tempo.
func TestStockAdjustments(t *testing.T) {
	e := newEnv(t)
	gestora := e.createUser(t, "gestor@quinta.pt", identity.RoleGestor, pw)
	e.createUser(t, "funcionario@quinta.pt", identity.RoleFuncionario, pw)
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
	newLot := func(productID string, milli int64, expiresIn int) string {
		t.Helper()
		r := call(t, srv, "POST", "/stock/lots", lot(productID, milli, lisbonDay(0), lisbonDay(expiresIn)), funcionario)
		if r.status != http.StatusCreated {
			t.Fatalf("registar: %d %v", r.status, r.body)
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
	movements := func(lotID string) int {
		t.Helper()
		var n int
		if err := e.pool.QueryRow(ctx, "SELECT count(*) FROM stock_movements WHERE lot_id = $1", lotID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	laranja := create("Laranja", nil)
	ovos := create("Ovos", func(in map[string]any) { in["unit"], in["lowStockMilli"] = "duzia", 0 })
	lotLaranja := newLot(laranja, 10_000, 7)
	lotOvos := newLot(ovos, 5000, 3)

	t.Run("o gestor regista uma quebra: lote, movimento, motivo e autor", func(t *testing.T) {
		r := call(t, srv, "POST", "/stock/adjustments", adjustment(lotLaranja, "quebra", -1500, "  Caixa caiu ao descarregar  "), gestor)
		if r.status != http.StatusCreated {
			t.Fatalf("quebra: %d %v", r.status, r.body)
		}
		mv, _ := r.body["movement"].(map[string]any)
		lt, _ := r.body["lot"].(map[string]any)
		if mv["type"] != "quebra" || mv["quantityMilli"] != float64(-1500) || mv["reason"] != "Caixa caiu ao descarregar" || mv["id"] == nil {
			t.Errorf("movimento: %v", mv)
		}
		if lt["quantityLeftMilli"] != float64(8500) || lt["quantityReceivedMilli"] != float64(10_000) || lt["unit"] != "kg" {
			t.Errorf("lote: %v", lt)
		}

		var typ, quantity, userID, reason string
		err := e.pool.QueryRow(ctx, `
			SELECT type, quantity::text, user_id::text, reason FROM stock_movements
			WHERE lot_id = $1 ORDER BY id DESC LIMIT 1`, lotLaranja).Scan(&typ, &quantity, &userID, &reason)
		if err != nil {
			t.Fatal(err)
		}
		if typ != "quebra" || quantity != "-1.500" || userID != gestora.ID.String() || reason != "Caixa caiu ao descarregar" {
			t.Errorf("na base de dados: %s %s por %s, %q", typ, quantity, userID, reason)
		}
	})

	t.Run("um ajuste corrige para mais e para menos", func(t *testing.T) {
		for _, milli := range []int64{750, -250} {
			if r := call(t, srv, "POST", "/stock/adjustments", adjustment(lotLaranja, "ajuste", milli, "Contagem de inventário"), gestor); r.status != http.StatusCreated {
				t.Errorf("ajuste %d: %d %v", milli, r.status, r.body)
			}
		}
		if got := left(lotLaranja); got != 9000 {
			t.Errorf("queria 9000, ficou %d", got)
		}
	})

	t.Run("um lote fora de validade ainda se pode dar como quebra", func(t *testing.T) {
		var expired string
		if err := e.pool.QueryRow(ctx, `
			INSERT INTO stock_lots (product_id, unit, qty_received, qty_left, harvested_on, expires_on)
			VALUES ($1, 'kg', 2, 2, current_date - 9, current_date - 2) RETURNING id`, laranja).Scan(&expired); err != nil {
			t.Fatal(err)
		}
		if r := call(t, srv, "POST", "/stock/adjustments", adjustment(expired, "quebra", -2000, "Passou da validade"), gestor); r.status != http.StatusCreated {
			t.Errorf("quebra: %d %v", r.status, r.body)
		}
	})

	t.Run("o stock não fica negativo (409, com o que resta), e nada se grava", func(t *testing.T) {
		before := movements(lotLaranja)
		r := call(t, srv, "POST", "/stock/adjustments", adjustment(lotLaranja, "quebra", -9001, "Tudo estragado"), gestor)
		if r.status != http.StatusConflict {
			t.Fatalf("queria 409, deu %d %v", r.status, r.body)
		}
		if detail, _ := r.body["detail"].(string); !strings.Contains(detail, "9 kg") {
			t.Errorf("a mensagem devia dizer o que resta (9 kg): %q", detail)
		}
		if left(lotLaranja) != 9000 || movements(lotLaranja) != before {
			t.Error("o lote ou os movimentos mudaram")
		}
	})

	t.Run("regras do servidor (422)", func(t *testing.T) {
		cases := map[string]map[string]any{
			"quebra positiva":        adjustment(lotLaranja, "quebra", 1000, "Engano"),
			"quantidade 0":           adjustment(lotLaranja, "ajuste", 0, "Nada"),
			"meia dúzia":             adjustment(lotOvos, "quebra", -500, "Partiu-se"),
			"motivo só com espaços":  adjustment(lotLaranja, "quebra", -1000, "   "),
			"motivo com duas linhas": adjustment(lotLaranja, "quebra", -1000, "linha 1\nlinha 2"),
		}
		for name, body := range cases {
			r := call(t, srv, "POST", "/stock/adjustments", body, gestor)
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
		extra := adjustment(lotLaranja, "quebra", -1000, "Caiu")
		extra["userId"] = gestora.ID.String() // o autor vem sempre da sessão
		cases := map[string]any{
			"sem motivo":         map[string]any{"lotId": lotLaranja, "type": "quebra", "quantityMilli": -1000},
			"motivo vazio":       adjustment(lotLaranja, "quebra", -1000, ""),
			"motivo com 501":     adjustment(lotLaranja, "quebra", -1000, strings.Repeat("a", 501)),
			"tipo venda":         adjustment(lotLaranja, "venda", -1000, "Vendi"),
			"tipo entrada":       adjustment(lotLaranja, "entrada", 1000, "Chegou"),
			"quantidade absurda": adjustment(lotLaranja, "ajuste", 100_000_001, "Zeros a mais"),
			"lote que não é id":  adjustment("lote-1", "quebra", -1000, "Caiu"),
			"campo a mais":       extra,
		}
		for name, body := range cases {
			if r := call(t, srv, "POST", "/stock/adjustments", body, gestor); r.status != http.StatusBadRequest {
				t.Errorf("%s: queria 400, deu %d %v", name, r.status, r.body)
			}
		}
	})

	t.Run("um motivo de 500 caracteres acentuados é aceite", func(t *testing.T) {
		r := call(t, srv, "POST", "/stock/adjustments", adjustment(lotOvos, "ajuste", 1000, strings.Repeat("é", 500)), gestor)
		if r.status != http.StatusCreated {
			t.Errorf("queria 201, deu %d %v", r.status, r.body)
		}
	})

	t.Run("lote inexistente (404)", func(t *testing.T) {
		r := call(t, srv, "POST", "/stock/adjustments", adjustment("00000000-0000-4000-8000-000000000000", "quebra", -1000, "Caiu"), gestor)
		if r.status != http.StatusNotFound {
			t.Errorf("queria 404, deu %d %v", r.status, r.body)
		}
	})

	t.Run("concorrência: uma saída que espera pelo bloqueio vê o que a outra deixou", func(t *testing.T) {
		// O lote tem 9 kg. Outra transação (como seria outro ajuste ou uma
		// venda) bloqueia-o e tira 6 kg; o pedido de tirar mais 6 kg espera
		// por ela e, quando continua, só há 3 kg: 409, nada se grava.
		tx, err := e.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		for _, stmt := range []string{
			"SELECT 1 FROM stock_lots WHERE id = $1 FOR UPDATE",
			"UPDATE stock_lots SET qty_left = qty_left - 6 WHERE id = $1",
		} {
			if _, err := tx.Exec(ctx, stmt, lotLaranja); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := tx.Exec(ctx, "INSERT INTO stock_movements (lot_id, type, quantity, user_id, reason) VALUES ($1, 'quebra', -6, $2, 'Outra quebra')", lotLaranja, gestora.ID); err != nil {
			t.Fatal(err)
		}

		pending := goCall(srv, "POST", "/stock/adjustments", adjustment(lotLaranja, "quebra", -6000, "Ao mesmo tempo"), gestor)
		waitForLockWait(t, e.pool)
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		r := receive(t, pending)
		if r.status != http.StatusConflict {
			t.Errorf("queria 409, deu %d %v", r.status, r.body)
		}
		// O CHECK (qty_left >= 0) também recusaria; é o bloqueio (o lote relido
		// depois da espera) que faz a mensagem dizer o valor atual, e não 9 kg.
		if detail, _ := r.body["detail"].(string); !strings.Contains(detail, "só tem 3 kg") {
			t.Errorf("a mensagem devia dizer o que resta agora (3 kg): %q", detail)
		}
		if got := left(lotLaranja); got != 3000 {
			t.Errorf("queria 3000, ficou %d", got)
		}
	})

	t.Run("concorrência: 10 quebras de 3 kg num lote de 10 kg, só 3 passam", func(t *testing.T) {
		lotID := newLot(laranja, 10_000, 5)
		const n = 10
		pending := make([]<-chan response, n)
		for i := range pending {
			pending[i] = goCall(srv, "POST", "/stock/adjustments", adjustment(lotID, "quebra", -3000, "Corrida"), gestor)
		}
		got := map[int]int{}
		for _, ch := range pending {
			got[receive(t, ch).status]++
		}
		if got[http.StatusCreated] != 3 || got[http.StatusConflict] != n-3 {
			t.Errorf("queria 3 × 201 e %d × 409, deu %v", n-3, got)
		}
		if l := left(lotID); l != 1000 {
			t.Errorf("queria 1000, ficou %d", l)
		}
		if m := movements(lotID); m != 1+3 {
			t.Errorf("queria a entrada e 3 quebras, há %d movimentos", m)
		}
	})

	t.Run("concorrência: 20 ajustes de +100 g ao mesmo tempo, nenhum se perde", func(t *testing.T) {
		before := left(lotLaranja)
		const n = 20
		pending := make([]<-chan response, n)
		for i := range pending {
			pending[i] = goCall(srv, "POST", "/stock/adjustments", adjustment(lotLaranja, "ajuste", 100, "Recontagem"), gestor)
		}
		for _, ch := range pending {
			if r := receive(t, ch); r.status != http.StatusCreated {
				t.Errorf("ajuste: %d %v", r.status, r.body)
			}
		}
		if got := left(lotLaranja); got != before+n*100 {
			t.Errorf("queria %d, ficou %d", before+n*100, got)
		}
	})

	t.Run("em todos os lotes, o que resta é a soma dos movimentos", func(t *testing.T) {
		rows, err := e.pool.Query(ctx, `
			SELECT l.id::text, l.qty_left::text, coalesce(sum(m.quantity), 0)::text
			FROM stock_lots l LEFT JOIN stock_movements m ON m.lot_id = l.id
			GROUP BY l.id HAVING l.qty_left <> coalesce(sum(m.quantity), 0)`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var id, qty, sum string
			_ = rows.Scan(&id, &qty, &sum)
			// O lote fora de validade foi criado à mão, sem movimento de entrada.
			if qty != "0.000" || sum != "-2.000" {
				t.Errorf("lote %s: resta %s, os movimentos somam %s", id, qty, sum)
			}
		}
	})

	t.Run("os lotes de um produto: com stock ou dentro da validade, por validade", func(t *testing.T) {
		// Um lote fora de validade e vazio já não aparece; um fora de
		// validade com stock aparece primeiro (é o que sai primeiro).
		for _, stmt := range []string{
			"INSERT INTO stock_lots (product_id, unit, qty_received, qty_left, harvested_on, expires_on) VALUES ($1, 'kg', 1, 0, current_date - 9, current_date - 3)",
			"INSERT INTO stock_lots (product_id, unit, qty_received, qty_left, harvested_on, expires_on) VALUES ($1, 'kg', 1, 1, current_date - 9, current_date - 4)",
		} {
			if _, err := e.pool.Exec(ctx, stmt, laranja); err != nil {
				t.Fatal(err)
			}
		}
		r, _, body := raw(t, srv, "GET", "/stock/lots?productId="+laranja, nil, funcionario)
		if r != http.StatusOK {
			t.Fatalf("listar: %d %s", r, body)
		}
		lots := decodeList(t, body)
		// O fora de validade com stock, o de 5 dias e o de 7 dias.
		if len(lots) != 3 {
			t.Fatalf("queria 3 lotes, deu %d: %v", len(lots), lots)
		}
		if lots[0]["quantityLeftMilli"] != float64(1000) || lots[1]["id"] == lotLaranja || lots[2]["id"] != lotLaranja {
			t.Errorf("ordem: %v", lots)
		}
		for _, l := range lots {
			if l["productId"] != laranja || l["unit"] != "kg" {
				t.Errorf("lote: %v", l)
			}
		}

		if r, _, _ := raw(t, srv, "GET", "/stock/lots", nil, funcionario); r != http.StatusBadRequest {
			t.Errorf("sem productId: queria 400, deu %d", r)
		}
		if r, _, body := raw(t, srv, "GET", "/stock/lots?productId=00000000-0000-4000-8000-000000000000", nil, gestor); r != http.StatusOK || strings.TrimSpace(string(body)) != "[]" {
			t.Errorf("produto sem lotes: %d %s", r, body)
		}
	})
}

func decodeList(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("resposta: %v (%s)", err, raw)
	}
	return out
}
