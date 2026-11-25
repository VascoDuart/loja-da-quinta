package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var sample = Email{To: []string{"gestora@quinta.pt", "ze@quinta.pt"}, Subject: "Alertas", Text: "Stock baixo (1)\n- Laranja"}

var apiKey = "re_" + strings.Repeat("x", 12)

func TestResendNotifierCallsTheHTTPAPI(t *testing.T) {
	var (
		method, path, auth, contentType string
		body                            map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		auth, contentType = r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write([]byte(`{"id":"49a3999c"}`))
	}))
	defer srv.Close()

	n := NewResendNotifier(apiKey, "Loja <onboarding@resend.dev>")
	if n.endpoint != "https://api.resend.com/emails" {
		t.Errorf("endpoint: %s", n.endpoint)
	}
	n.endpoint = srv.URL + "/emails"

	d, err := n.Send(context.Background(), sample)
	if err != nil || d != DeliveryEmail {
		t.Fatal(d, err)
	}
	if method != "POST" || path != "/emails" || auth != "Bearer "+apiKey || contentType != "application/json" {
		t.Errorf("pedido: %s %s %q %q", method, path, auth, contentType)
	}
	to, _ := body["to"].([]any)
	if body["from"] != "Loja <onboarding@resend.dev>" || body["subject"] != "Alertas" || body["text"] != sample.Text || len(to) != 2 {
		t.Errorf("corpo: %v", body)
	}
}

func TestResendNotifierErrorsRevealNeitherKeyNorAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"statusCode":403,"name":"validation_error","message":"You can only send to dono@exemplo.pt"}`))
	}))
	defer srv.Close()
	n := NewResendNotifier(apiKey, "x@resend.dev")
	n.endpoint = srv.URL

	_, err := n.Send(context.Background(), sample)
	if err == nil {
		t.Fatal("um 403 é um erro")
	}
	if !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "validation_error") {
		t.Errorf("o erro devia dizer o estado e o tipo: %v", err)
	}
	for _, secret := range []string{apiKey, "dono@exemplo.pt", "gestora@quinta.pt"} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("o erro revela %q: %v", secret, err)
		}
	}
}

func TestResendNotifierDoesNotFollowRedirects(t *testing.T) {
	var followed bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed = true }))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	n := NewResendNotifier(apiKey, "x@resend.dev")
	n.endpoint = srv.URL

	if _, err := n.Send(context.Background(), sample); err == nil || followed {
		t.Errorf("um redirecionamento é um erro e não se segue: %v %v", err, followed)
	}
}

func TestLogNotifierWritesTheEmailWithoutAddresses(t *testing.T) {
	var out bytes.Buffer
	n := NewLogNotifier(slog.New(slog.NewJSONHandler(&out, nil)))
	d, err := n.Send(context.Background(), sample)
	if err != nil || d != DeliveryLog {
		t.Fatal(d, err)
	}
	logged := out.String()
	for _, want := range []string{`"assunto":"Alertas"`, `"destinatarios":2`, "Laranja"} {
		if !strings.Contains(logged, want) {
			t.Errorf("o log devia ter %q: %s", want, logged)
		}
	}
	if strings.Contains(logged, "@") {
		t.Errorf("o log não leva endereços de e-mail: %s", logged)
	}
}
