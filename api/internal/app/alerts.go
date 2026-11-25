package app

import (
	"context"
	"log/slog"

	"github.com/VascoDuart/loja-da-quinta/api/internal/inventory"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/config"
	"github.com/VascoDuart/loja-da-quinta/api/internal/users"
)

// newNotifier escolhe por onde saem os alertas: o Resend, se houver chave;
// senão o log, para o ambiente local e o staging funcionarem sem conta.
func newNotifier(cfg config.Config, log *slog.Logger) inventory.Notifier {
	if cfg.ResendAPIKey == "" {
		if cfg.Env != config.EnvDevelopment {
			log.Warn("sem RESEND_API_KEY: os alertas de stock ficam no log, não são enviados")
		}
		return inventory.NewLogNotifier(log)
	}
	return inventory.NewResendNotifier(cfg.ResendAPIKey, cfg.AlertsFrom)
}

// newRecipients diz a quem vão os alertas: a lista de ALERTS_TO, se houver;
// senão os gestores ativos, perguntando ao módulo users.
func newRecipients(cfg config.Config, userSvc *users.Service) inventory.Recipients {
	if len(cfg.AlertsTo) > 0 {
		return fixedRecipients(cfg.AlertsTo)
	}
	return activeManagers{userSvc}
}

type fixedRecipients []string

func (f fixedRecipients) AlertRecipients(context.Context) ([]string, error) { return f, nil }

type activeManagers struct{ svc *users.Service }

func (a activeManagers) AlertRecipients(ctx context.Context) ([]string, error) {
	return a.svc.ActiveManagerEmails(ctx)
}
