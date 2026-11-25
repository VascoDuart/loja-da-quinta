package inventory

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// AlertHour é a hora de Lisboa a partir da qual o alerta do dia pode sair
// (RF-12).
const AlertHour = 7

// sendTimeout limita o envio depois de o dia estar marcado, já sem o prazo do
// pedido (ver Run).
const sendTimeout = 15 * time.Second

// AlertStatus diz o que um disparo do job fez.
type AlertStatus string

const (
	AlertSent        AlertStatus = "enviado"
	AlertAlreadySent AlertStatus = "ja_enviado"
	AlertTooEarly    AlertStatus = "antes_da_hora"
	AlertNothing     AlertStatus = "sem_alertas"
)

// AlertRun é o resultado de um disparo.
type AlertRun struct {
	Status AlertStatus
	// Day é o dia de Lisboa a que se refere.
	Day time.Time
	// Delivery só vem preenchido quando Status é AlertSent.
	Delivery Delivery
	Low      int
	Expiring int
	Expired  int
}

// Alerts envia o alerta diário de stock. Não há agendamento dentro da API,
// que adormece no plano gratuito: é o GitHub Actions que a chama (ADR-0015).
//
// Não recebe quem pede: só o endpoint dos jobs o chama, e esse só aceita o
// token dos jobs.
type Alerts struct {
	stock      *Service
	days       AlertDays
	recipients Recipients
	notifier   Notifier
	log        *slog.Logger
}

// NewAlerts cria o serviço dos alertas. O relógio é o do serviço do stock.
func NewAlerts(stock *Service, days AlertDays, recipients Recipients, notifier Notifier, log *slog.Logger) *Alerts {
	return &Alerts{stock: stock, days: days, recipients: recipients, notifier: notifier, log: log}
}

// Run envia o alerta de hoje, se for caso disso: a partir das 07:00 de
// Lisboa, no máximo uma vez por dia, e só se houver o que avisar. Pode
// chamar-se as vezes que se quiser.
//
// O dia é marcado antes do envio: entre dois disparos ao mesmo tempo, só um
// o consegue marcar. Se o envio falhar, o dia fica outra vez livre. Se o
// processo morrer a meio do envio, o dia fica marcado e o alerta desse dia
// perde-se: prefere-se isso a enviá-lo duas vezes.
func (a *Alerts) Run(ctx context.Context) (AlertRun, error) {
	now := a.stock.now()
	run := AlertRun{Day: Day(now)}
	if now.In(lisbon).Hour() < AlertHour {
		run.Status = AlertTooEarly
		return run, nil
	}

	overview, err := a.stock.overview(ctx, run.Day)
	if err != nil {
		return AlertRun{}, err
	}
	report := overview.Report()
	run.Low, run.Expiring, run.Expired = len(report.Low), len(report.Expiring), len(report.Expired)
	if report.Empty() {
		run.Status = AlertNothing
		return run, nil
	}

	to, err := a.recipients.AlertRecipients(ctx)
	if err != nil {
		return AlertRun{}, fmt.Errorf("ler os destinatários: %w", err)
	}
	if len(to) == 0 {
		return AlertRun{}, ErrNoRecipients
	}

	claimed, err := a.days.ClaimDay(ctx, run.Day)
	if err != nil {
		return AlertRun{}, err
	}
	if !claimed {
		run.Status = AlertAlreadySent
		return run, nil
	}

	// A partir daqui, quem disparou já não interrompe o envio: se o pedido
	// for cancelado a meio, não se saberia se o e-mail saiu.
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sendTimeout)
	defer cancel()

	subject, text := report.Message()
	delivery, err := a.notifier.Send(sendCtx, Email{To: to, Subject: subject, Text: text})
	if err != nil {
		releaseCtx, cancelRelease := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancelRelease()
		if rerr := a.days.ReleaseDay(releaseCtx, run.Day); rerr != nil {
			a.log.ErrorContext(ctx, "o dia do alerta ficou marcado sem o e-mail ter saído", "dia", run.Day.Format(time.DateOnly), "err", rerr)
		}
		return AlertRun{}, fmt.Errorf("%w: %w", ErrAlertNotSent, err)
	}
	run.Status, run.Delivery = AlertSent, delivery
	return run, nil
}
