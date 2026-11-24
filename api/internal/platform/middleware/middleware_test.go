package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/httpx"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/logging"
)

var ok = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

func TestChainOrder(t *testing.T) {
	var order []string
	mark := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	Chain(ok, mark("a"), mark("b"), mark("c")).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if got := strings.Join(order, ","); got != "a,b,c" {
		t.Errorf("ordem = %s", got)
	}
}

func TestRequestID(t *testing.T) {
	var seen string
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = logging.RequestID(r.Context())
	}))

	t.Run("gera um ID quando não há", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		if len(seen) != 32 || w.Header().Get(HeaderRequestID) != seen {
			t.Errorf("id = %q, cabeçalho = %q", seen, w.Header().Get(HeaderRequestID))
		}
	})
	t.Run("aceita um ID válido do cliente", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set(HeaderRequestID, "abc-12345678")
		h.ServeHTTP(httptest.NewRecorder(), r)
		if seen != "abc-12345678" {
			t.Errorf("id = %q", seen)
		}
	})
	t.Run("recusa um ID que permitiria injeção nos logs", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set(HeaderRequestID, "x\n{\"level\":\"ERROR\"}")
		h.ServeHTTP(httptest.NewRecorder(), r)
		if strings.Contains(seen, "\n") || len(seen) != 32 {
			t.Errorf("id = %q", seen)
		}
	})
}

func TestRecoverReturnsProblemWithoutStack(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	h := Recover(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("rebentou") }))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "rebentou") || strings.Contains(w.Body.String(), "goroutine") {
		t.Errorf("a resposta expõe detalhes internos: %s", w.Body.String())
	}
	if !strings.Contains(logs.String(), "rebentou") {
		t.Errorf("o panic devia ficar nos logs: %s", logs.String())
	}
}

func TestAccessLogDoesNotLogQueryString(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	AccessLog(log)(ok).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x?token=segredo", nil))
	if strings.Contains(logs.String(), "segredo") {
		t.Errorf("o log contém a query string: %s", logs.String())
	}
	if !strings.Contains(logs.String(), `"status":204`) {
		t.Errorf("o log devia ter o status: %s", logs.String())
	}
}

func TestSecurityHeaders(t *testing.T) {
	for _, hsts := range []bool{false, true} {
		w := httptest.NewRecorder()
		SecurityHeaders(hsts)(ok).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy", "Cache-Control"} {
			if w.Header().Get(h) == "" {
				t.Errorf("falta o cabeçalho %s", h)
			}
		}
		if got := w.Header().Get("Strict-Transport-Security") != ""; got != hsts {
			t.Errorf("HSTS presente = %v, queria %v", got, hsts)
		}
	}
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		name, xff string
		hops      int
		want      string
	}{
		{"sem proxies usa a ligação TCP", "1.1.1.1", 0, "10.0.0.9"},
		{"dois proxies: cliente, vercel", "203.0.113.7, 76.76.21.1", 2, "203.0.113.7"},
		{"IP forjado à esquerda é ignorado", "6.6.6.6, 203.0.113.7, 76.76.21.1", 2, "203.0.113.7"},
		{"cabeçalho curto demais usa a ligação TCP", "76.76.21.1", 2, "10.0.0.9"},
		{"valor que não é IP usa a ligação TCP", "lixo, 76.76.21.1", 2, "10.0.0.9"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = "10.0.0.9:51234"
			r.Header.Set("X-Forwarded-For", c.xff)
			if got := ClientIP(r, c.hops); got != c.want {
				t.Errorf("ClientIP = %q, queria %q", got, c.want)
			}
		})
	}
}

func TestRateLimit(t *testing.T) {
	h := ClientIPContext(0)(RateLimit(1, 2)(ok))
	codes := make([]int, 0, 3)
	for range 3 {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		codes = append(codes, w.Code)
	}
	if codes[0] != 204 || codes[1] != 204 || codes[2] != 429 {
		t.Errorf("códigos = %v, queria [204 204 429]", codes)
	}

	other := httptest.NewRequest("GET", "/", nil)
	other.RemoteAddr = "192.0.2.50:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, other)
	if w.Code != 204 {
		t.Errorf("outro IP não devia ser limitado: %d", w.Code)
	}
}

func TestClientIPContext(t *testing.T) {
	var got string
	h := ClientIPContext(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = httpx.ClientIP(r.Context())
	}))
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if got != "203.0.113.9" {
		t.Errorf("IP no contexto = %q", got)
	}
}

func TestMaxBody(t *testing.T) {
	var readErr error
	h := MaxBody(4)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader("123456")))
	if readErr == nil {
		t.Error("ler para além do limite devia falhar")
	}
}

func TestTimeoutSetsDeadline(t *testing.T) {
	var hasDeadline bool
	Timeout(time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasDeadline = r.Context().Deadline()
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if !hasDeadline {
		t.Error("o contexto devia ter um prazo")
	}
}
