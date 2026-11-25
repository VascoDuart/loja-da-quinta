//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VascoDuart/loja-da-quinta/api/internal/app"
	"github.com/VascoDuart/loja-da-quinta/api/internal/inventory"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

var jobsToken = strings.Repeat("j", 40)

// TestStockOverview prova a consulta do stock (RF-11) contra um PostgreSQL
// real: o que conta para o stock vendável, o estado de cada lote e o stock
// baixo.
func TestStockOverview(t *testing.T) {
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
	register := func(productID string, milli int64, expiresIn int) string {
		t.Helper()
		r := call(t, srv, "POST", "/stock/lots", lot(productID, milli, lisbonDay(0), lisbonDay(expiresIn)), funcionario)
		if r.status != http.StatusCreated {
			t.Fatalf("registar: %d %v", r.status, r.body)
		}
		return r.body["id"].(string)
	}
	// A API não aceita um lote já fora de validade: envelhece-se na base.
	expire := func(lotID string) {
		t.Helper()
		if _, err := e.pool.Exec(ctx, "UPDATE stock_lots SET harvested_on = $2::date, expires_on = $3::date WHERE id = $1",
			lotID, lisbonDay(-10), lisbonDay(-1)); err != nil {
			t.Fatal(err)
		}
	}

	// Os nomes saem por ordem: Ameixa, Banana, Cereja (limiar 2,5 kg em todos).
	cereja := create("Cereja")
	ameixa := create("Ameixa")
	banana := create("Banana")
	expired := register(ameixa, 5000, 2)
	expire(expired)
	soon := register(ameixa, 1500, 3)
	later := register(ameixa, 500, 4)
	emptied := register(banana, 1000, 10)
	register(banana, 8000, 10)
	if r := call(t, srv, "POST", "/stock/adjustments", adjustment(emptied, "quebra", -1000, "Teste"), gestor); r.status != http.StatusCreated {
		t.Fatalf("quebra: %d %v", r.status, r.body)
	}

	for who, h := range map[string]map[string]string{"funcionário": funcionario, "gestor": gestor} {
		r := call(t, srv, "GET", "/stock", nil, h)
		if r.status != http.StatusOK {
			t.Fatalf("%s: %d %v", who, r.status, r.body)
		}
		if r.body["today"] != lisbonDay(0) || r.body["expiringUntil"] != lisbonDay(3) {
			t.Errorf("dias: %v %v", r.body["today"], r.body["expiringUntil"])
		}
		products := r.body["products"].([]any)
		if len(products) != 3 {
			t.Fatalf("produtos: %v", products)
		}
		byIndex := func(i int) map[string]any { return products[i].(map[string]any) }
		statuses := func(p map[string]any) []string {
			var out []string
			for _, l := range p["lots"].([]any) {
				m := l.(map[string]any)
				out = append(out, m["id"].(string)+" "+m["status"].(string))
			}
			return out
		}

		a := byIndex(0)
		// 1,5 + 0,5 kg: os 5 kg fora de validade não contam, e 2 < 2,5.
		if a["productId"] != ameixa || a["name"] != "Ameixa" || a["unit"] != "kg" || a["active"] != true ||
			a["sellableMilli"] != float64(2000) || a["lowStockMilli"] != float64(2500) || a["low"] != true {
			t.Errorf("ameixa: %v", a)
		}
		want := []string{expired + " fora_de_validade", soon + " expira_em_breve", later + " valido"}
		if got := statuses(a); !slices.Equal(got, want) {
			t.Errorf("lotes da ameixa: %v, queria %v", got, want)
		}
		first := a["lots"].([]any)[0].(map[string]any)
		if first["quantityLeftMilli"] != float64(5000) || first["expiresOn"] != lisbonDay(-1) || first["productId"] != ameixa {
			t.Errorf("lote: %v", first)
		}

		b := byIndex(1)
		// O lote posto a 0 pela quebra já não aparece.
		if b["productId"] != banana || b["sellableMilli"] != float64(8000) || b["low"] != false || len(statuses(b)) != 1 {
			t.Errorf("banana: %v", b)
		}
		c := byIndex(2)
		if c["productId"] != cereja || c["sellableMilli"] != float64(0) || c["low"] != true || len(statuses(c)) != 0 {
			t.Errorf("cereja: %v", c)
		}
	}
}

type recordingNotifier struct {
	mu   sync.Mutex
	sent []inventory.Email
	err  error
}

func (n *recordingNotifier) Send(_ context.Context, m inventory.Email) (inventory.Delivery, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.err != nil {
		return "", n.err
	}
	n.sent = append(n.sent, m)
	return inventory.DeliveryEmail, nil
}

func (n *recordingNotifier) emails() []inventory.Email {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.Clone(n.sent)
}

// TestStockAlerts prova o job diário (RF-12, ADR-0015) com um PostgreSQL
// real e um Notifier falso: a hora de Lisboa, os destinatários, uma vez por
// dia e dois disparos ao mesmo tempo.
func TestStockAlerts(t *testing.T) {
	e := newEnv(t)
	e.cfg.JobsToken = jobsToken
	e.createUser(t, "gestora@quinta.pt", identity.RoleGestor, pw)
	e.createUser(t, "rui@quinta.pt", identity.RoleGestor, pw)
	e.createUser(t, "funcionario@quinta.pt", identity.RoleFuncionario, pw)
	e.createUser(t, "antigo@quinta.pt", identity.RoleGestor, pw)
	ctx := context.Background()
	if _, err := e.pool.Exec(ctx, "UPDATE users SET active = false WHERE email = 'antigo@quinta.pt'"); err != nil {
		t.Fatal(err)
	}
	notifier := &recordingNotifier{}

	// day é o dia de hoje em Lisboa mais n; at dá um servidor com o relógio
	// parado nesse dia, à hora UTC pedida (08:00 UTC são 08 ou 09 em Lisboa;
	// 05:00 UTC são 05 ou 06).
	day := func(n int) time.Time { return inventory.Day(time.Now()).AddDate(0, 0, n) }
	at := func(n, hourUTC int) *httptest.Server {
		now := day(n).Add(time.Duration(hourUTC) * time.Hour)
		return e.server(t, app.WithClock(func() time.Time { return now }), app.WithNotifier(notifier))
	}
	headers := map[string]string{"X-Job-Token": jobsToken}
	trigger := func(srv *httptest.Server) response {
		t.Helper()
		return call(t, srv, "POST", "/jobs/stock-alerts", nil, headers)
	}
	expect := func(r response, status string, n int) {
		t.Helper()
		if r.status != http.StatusOK || r.body["status"] != status || r.body["day"] != day(n).Format(time.DateOnly) {
			t.Errorf("queria %s no dia %d, deu %d %v", status, n, r.status, r.body)
		}
	}
	recorded := func() []string {
		t.Helper()
		rows, err := e.pool.Query(ctx, "SELECT day::text FROM stock_alert_runs ORDER BY day")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var d string
			if err := rows.Scan(&d); err != nil {
				t.Fatal(err)
			}
			out = append(out, d)
		}
		return out
	}

	t.Run("sem nada a avisar não se envia nem se marca o dia", func(t *testing.T) {
		r := trigger(at(0, 8))
		expect(r, "sem_alertas", 0)
		if len(notifier.emails()) != 0 || len(recorded()) != 0 {
			t.Errorf("enviados %d, dias %v", len(notifier.emails()), recorded())
		}
	})

	// Um produto sem stock, abaixo do limiar: há sempre o que avisar.
	srv := e.server(t)
	gestorToken, _ := login(t, srv, "gestora@quinta.pt")
	fruta := call(t, srv, "POST", "/categories", map[string]string{"name": "Fruta"}, bearer(gestorToken)).body["id"]
	if r := call(t, srv, "POST", "/products", product(fruta, "Marmelo"), bearer(gestorToken)); r.status != http.StatusCreated {
		t.Fatalf("criar: %d %v", r.status, r.body)
	}

	t.Run("antes das 07:00 de Lisboa não envia", func(t *testing.T) {
		expect(trigger(at(0, 5)), "antes_da_hora", 0)
		if len(notifier.emails()) != 0 || len(recorded()) != 0 {
			t.Error("antes da hora não se envia nem se marca o dia")
		}
	})

	t.Run("se o envio falhar dá 502 e o dia fica livre", func(t *testing.T) {
		notifier.err = errors.New("o Resend recusou o e-mail: HTTP 500")
		defer func() { notifier.err = nil }()
		r := trigger(at(0, 8))
		if r.status != http.StatusBadGateway {
			t.Fatalf("queria 502, deu %d %v", r.status, r.body)
		}
		if detail, _ := r.body["detail"].(string); strings.Contains(detail, "Resend recusou") {
			t.Errorf("a resposta mostra o erro interno: %v", r.body)
		}
		if len(recorded()) != 0 {
			t.Errorf("o dia ficou marcado: %v", recorded())
		}
	})

	t.Run("às 07:00 envia aos gestores ativos, uma só vez", func(t *testing.T) {
		today := at(0, 8)
		r := trigger(today)
		expect(r, "enviado", 0)
		if r.body["delivery"] != "email" || r.body["lowStockCount"] != float64(1) {
			t.Errorf("resposta: %v", r.body)
		}
		expect(trigger(today), "ja_enviado", 0)
		expect(trigger(at(0, 15)), "ja_enviado", 0) // outro processo, o mesmo dia

		emails := notifier.emails()
		if len(emails) != 1 {
			t.Fatalf("enviados: %d", len(emails))
		}
		to := slices.Clone(emails[0].To)
		slices.Sort(to)
		// Nem o funcionário nem o gestor desativado.
		if !slices.Equal(to, []string{"gestora@quinta.pt", "rui@quinta.pt"}) {
			t.Errorf("destinatários: %v", to)
		}
		if !strings.Contains(emails[0].Text, "Marmelo") {
			t.Errorf("texto: %s", emails[0].Text)
		}
		if got := recorded(); !slices.Equal(got, []string{day(0).Format(time.DateOnly)}) {
			t.Errorf("dias marcados: %v", got)
		}
	})

	t.Run("vários disparos ao mesmo tempo: um só envia", func(t *testing.T) {
		before := len(notifier.emails())
		tomorrow := at(1, 8)
		const n = 8
		var pending []<-chan response
		for range n {
			pending = append(pending, goCall(tomorrow, "POST", "/jobs/stock-alerts", nil, headers))
		}
		counts := map[any]int{}
		for _, ch := range pending {
			r := receive(t, ch)
			if r.status != http.StatusOK {
				t.Fatalf("%d %v", r.status, r.body)
			}
			counts[r.body["status"]]++
		}
		if counts["enviado"] != 1 || counts["ja_enviado"] != n-1 {
			t.Errorf("resultados: %v", counts)
		}
		if got := len(notifier.emails()) - before; got != 1 {
			t.Errorf("enviados %d e-mails", got)
		}
	})

	// A corrida sem depender da sorte: o teste faz de "outro disparo", com o
	// dia inserido numa transação ainda por confirmar. A restrição única
	// obriga o pedido a esperar pelo desfecho.
	for name, c := range map[string]struct {
		offset int
		commit bool
		want   string
		emails int
	}{
		"o outro disparo confirma: não envia": {offset: 2, commit: true, want: "ja_enviado", emails: 0},
		"o outro disparo desiste: envia":      {offset: 3, commit: false, want: "enviado", emails: 1},
	} {
		t.Run("corrida, "+name, func(t *testing.T) {
			before := len(notifier.emails())
			srv := at(c.offset, 8)
			tx, err := e.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if _, err := tx.Exec(ctx, "INSERT INTO stock_alert_runs (day) VALUES ($1::date)", day(c.offset).Format(time.DateOnly)); err != nil {
				t.Fatal(err)
			}

			pending := goCall(srv, "POST", "/jobs/stock-alerts", nil, headers)
			waitForLockWait(t, e.pool)
			if c.commit {
				err = tx.Commit(ctx)
			} else {
				err = tx.Rollback(ctx)
			}
			if err != nil {
				t.Fatal(err)
			}

			expect(receive(t, pending), c.want, c.offset)
			if got := len(notifier.emails()) - before; got != c.emails {
				t.Errorf("enviados %d e-mails, queria %d", got, c.emails)
			}
		})
	}

	t.Run("ALERTS_TO substitui os gestores", func(t *testing.T) {
		e.cfg.AlertsTo = []string{"dono@exemplo.pt"}
		defer func() { e.cfg.AlertsTo = nil }()
		expect(trigger(at(4, 8)), "enviado", 4)
		emails := notifier.emails()
		if got := emails[len(emails)-1].To; !slices.Equal(got, []string{"dono@exemplo.pt"}) {
			t.Errorf("destinatários: %v", got)
		}
	})

	t.Run("sem chave do Resend o alerta fica no log", func(t *testing.T) {
		now := day(5).Add(8 * time.Hour)
		r := trigger(e.server(t, app.WithClock(func() time.Time { return now })))
		expect(r, "enviado", 5)
		if r.body["delivery"] != "log" {
			t.Errorf("entrega: %v", r.body)
		}
	})
}
