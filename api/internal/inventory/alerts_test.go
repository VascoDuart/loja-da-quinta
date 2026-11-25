package inventory

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type fakeDays struct {
	claimed  map[string]bool
	released int
}

func (f *fakeDays) ClaimDay(_ context.Context, d time.Time) (bool, error) {
	key := d.Format(time.DateOnly)
	if f.claimed[key] {
		return false, nil
	}
	if f.claimed == nil {
		f.claimed = map[string]bool{}
	}
	f.claimed[key] = true
	return true, nil
}

func (f *fakeDays) ReleaseDay(_ context.Context, d time.Time) error {
	f.released++
	delete(f.claimed, d.Format(time.DateOnly))
	return nil
}

type fakeNotifier struct {
	sent []Email
	err  error
	// ctxErr é o estado do contexto com que o envio foi chamado.
	ctxErr error
}

func (f *fakeNotifier) Send(ctx context.Context, m Email) (Delivery, error) {
	f.ctxErr = ctx.Err()
	if f.err != nil {
		return "", f.err
	}
	f.sent = append(f.sent, m)
	return DeliveryEmail, nil
}

type fakeRecipients []string

func (f fakeRecipients) AlertRecipients(context.Context) ([]string, error) { return f, nil }

type alertsFixture struct {
	alerts   *Alerts
	repo     *fakeRepo
	days     *fakeDays
	notifier *fakeNotifier
	now      time.Time
}

// newAlerts: um produto com stock baixo, às 07:00 de Lisboa de 25/11/2026
// (hora de inverno: 07:00 UTC).
func newAlerts(recipients ...string) *alertsFixture {
	laranja := product("Laranja", "kg", 5000, true)
	f := &alertsFixture{
		repo:     &fakeRepo{stocked: []Lot{lotOf(laranja, 2000, 20)}},
		days:     &fakeDays{},
		notifier: &fakeNotifier{},
		now:      time.Date(2026, 11, 25, 7, 0, 0, 0, time.UTC),
	}
	stock := NewService(f.repo, fakeProducts{laranja.ID: laranja}, func() time.Time { return f.now })
	f.alerts = NewAlerts(stock, f.days, fakeRecipients(recipients), f.notifier, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return f
}

func (f *alertsFixture) run(t *testing.T) AlertRun {
	t.Helper()
	r, err := f.alerts.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAlertsOnlyFromSevenInLisbon(t *testing.T) {
	cases := []struct {
		name string
		at   time.Time
		want AlertStatus
	}{
		{"inverno, 06:59", time.Date(2026, 11, 25, 6, 59, 59, 0, time.UTC), AlertTooEarly},
		{"inverno, 07:00", time.Date(2026, 11, 25, 7, 0, 0, 0, time.UTC), AlertSent},
		// Hora de verão (UTC+1): o cron das 06:00 UTC são as 07:00 em Lisboa.
		{"verão, 05:59 UTC", time.Date(2026, 7, 1, 5, 59, 0, 0, time.UTC), AlertTooEarly},
		{"verão, 06:00 UTC", time.Date(2026, 7, 1, 6, 0, 0, 0, time.UTC), AlertSent},
		// Um disparo atrasado ainda envia.
		{"à tarde", time.Date(2026, 11, 25, 15, 0, 0, 0, time.UTC), AlertSent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newAlerts("gestor@loja.test")
			// O lote do fixture é de novembro: em julho conta como válido.
			f.now = c.at
			r := f.run(t)
			if r.Status != c.want || !r.Day.Equal(Day(c.at)) {
				t.Errorf("queria %s, deu %s (%v)", c.want, r.Status, r.Day)
			}
			if c.want == AlertTooEarly && (len(f.notifier.sent) != 0 || len(f.days.claimed) != 0) {
				t.Error("antes da hora não se envia nem se marca o dia")
			}
		})
	}
}

func TestAlertsOncePerDay(t *testing.T) {
	f := newAlerts("gestor@loja.test", "outra@loja.test")

	first := f.run(t)
	if first.Status != AlertSent || first.Delivery != DeliveryEmail || first.Low != 1 {
		t.Fatalf("primeiro: %+v", first)
	}
	second := f.run(t)
	if second.Status != AlertAlreadySent || second.Delivery != "" {
		t.Errorf("segundo: %+v", second)
	}
	if len(f.notifier.sent) != 1 {
		t.Fatalf("enviados: %d", len(f.notifier.sent))
	}
	m := f.notifier.sent[0]
	if len(m.To) != 2 || m.To[0] != "gestor@loja.test" || m.Subject == "" || m.Text == "" {
		t.Errorf("e-mail: %+v", m)
	}

	f.now = f.now.AddDate(0, 0, 1)
	if r := f.run(t); r.Status != AlertSent || len(f.notifier.sent) != 2 {
		t.Errorf("no dia seguinte volta a enviar: %+v", r)
	}
}

func TestAlertsWithNothingToReport(t *testing.T) {
	f := newAlerts("gestor@loja.test")
	f.repo.stocked[0].QtyLeftMilli = 6000
	if r := f.run(t); r.Status != AlertNothing {
		t.Fatalf("queria sem_alertas, deu %+v", r)
	}
	if len(f.notifier.sent) != 0 || len(f.days.claimed) != 0 {
		t.Error("sem alertas não se envia nem se marca o dia")
	}
}

func TestAlertsFailedSendCanBeRetried(t *testing.T) {
	f := newAlerts("gestor@loja.test")
	f.notifier.err = errors.New("o serviço de e-mail está em baixo")

	if _, err := f.alerts.Run(context.Background()); !errors.Is(err, ErrAlertNotSent) {
		t.Fatalf("queria ErrAlertNotSent, deu %v", err)
	}
	if f.days.released != 1 || len(f.days.claimed) != 0 {
		t.Error("o dia tem de ficar livre para voltar a tentar")
	}
	f.notifier.err = nil
	if r := f.run(t); r.Status != AlertSent {
		t.Errorf("segunda tentativa: %+v", r)
	}
}

func TestAlertsWithoutRecipients(t *testing.T) {
	f := newAlerts()
	if _, err := f.alerts.Run(context.Background()); !errors.Is(err, ErrNoRecipients) {
		t.Fatalf("queria ErrNoRecipients, deu %v", err)
	}
	if len(f.days.claimed) != 0 {
		t.Error("sem destinatários não se marca o dia")
	}
}

func TestAlertsSendSurvivesACancelledRequest(t *testing.T) {
	// Quem disparou desistiu depois de o dia ficar marcado: o envio não pode
	// ser interrompido a meio (não se saberia se o e-mail saiu).
	f := newAlerts("gestor@loja.test")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.alerts.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if f.notifier.ctxErr != nil {
		t.Errorf("o envio recebeu um contexto cancelado: %v", f.notifier.ctxErr)
	}
}
