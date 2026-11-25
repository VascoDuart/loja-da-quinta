package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"time"
)

// resendURL é o endpoint de envio da API HTTP do Resend.
const resendURL = "https://api.resend.com/emails"

// ResendNotifier envia pelo Resend, com a API HTTP e a biblioteca padrão
// (ADR-0015).
type ResendNotifier struct {
	apiKey   string
	from     string
	endpoint string
	client   *http.Client
}

var _ Notifier = (*ResendNotifier)(nil)

// NewResendNotifier cria o adaptador. from é o remetente ("Nome <e-mail>").
func NewResendNotifier(apiKey, from string) *ResendNotifier {
	return &ResendNotifier{
		apiKey: apiKey, from: from, endpoint: resendURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
			// A API não redireciona: um redirecionamento é um erro, e não se
			// leva o e-mail (nem a chave) para outro sítio.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// errorName é o que se aceita como nome de um erro do Resend: um identificador
// curto, nunca uma frase (as mensagens podem trazer endereços de e-mail).
var errorName = regexp.MustCompile(`^[a-z_]{1,64}$`)

// Send envia um e-mail de texto. Os erros dizem o estado HTTP e o tipo do
// erro, nunca a chave nem endereços.
func (n *ResendNotifier) Send(ctx context.Context, m Email) (Delivery, error) {
	body, err := json.Marshal(map[string]any{"from": n.from, "to": m.To, "subject": m.Subject, "text": m.Text})
	if err != nil {
		return "", fmt.Errorf("preparar o e-mail: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", errors.New("preparar o pedido ao Resend")
	}
	req.Header.Set("Authorization", "Bearer "+n.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := n.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("contactar o Resend: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return DeliveryEmail, nil
	}
	var problem struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(io.LimitReader(res.Body, 4<<10)).Decode(&problem)
	if !errorName.MatchString(problem.Name) {
		problem.Name = "sem tipo"
	}
	return "", fmt.Errorf("o Resend recusou o e-mail: HTTP %d (%s)", res.StatusCode, problem.Name)
}

// LogNotifier é o que se usa sem chave do Resend: escreve o e-mail no log em
// vez de o enviar, para o staging funcionar sem conta. Os endereços não vão
// para o log, só quantos são.
type LogNotifier struct{ log *slog.Logger }

var _ Notifier = (*LogNotifier)(nil)

// NewLogNotifier cria o adaptador.
func NewLogNotifier(log *slog.Logger) *LogNotifier { return &LogNotifier{log: log} }

// Send escreve o e-mail no log.
func (n *LogNotifier) Send(ctx context.Context, m Email) (Delivery, error) {
	n.log.InfoContext(ctx, "alerta de stock não enviado (sem RESEND_API_KEY): fica no log",
		"destinatarios", len(m.To), "assunto", m.Subject, "texto", m.Text)
	return DeliveryLog, nil
}
