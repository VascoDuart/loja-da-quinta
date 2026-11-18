// Spike #3: mede o arranque a frio da Render através do rewrite da Vercel.
// Código descartável: não segue a arquitetura da API real.
package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)

var startedAt = time.Now()

func main() {
	mux := http.NewServeMux()

	// Estado do processo: um uptime baixo indica que houve arranque a frio.
	mux.HandleFunc("GET /api/v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"status":        "ok",
			"startedAt":     startedAt.UTC().Format(time.RFC3339),
			"uptimeSeconds": int(time.Since(startedAt).Seconds()),
		})
	})

	// Atraso artificial, para descobrir o tempo limite do proxy da Vercel.
	mux.HandleFunc("GET /api/v1/slow", func(w http.ResponseWriter, r *http.Request) {
		ms, _ := strconv.Atoi(r.URL.Query().Get("ms"))
		ms = min(max(ms, 0), 120_000)
		time.Sleep(time.Duration(ms) * time.Millisecond)
		writeJSON(w, map[string]any{"sleptMs": ms})
	})

	// Define um cookie com as mesmas propriedades do refresh token real.
	mux.HandleFunc("POST /api/v1/cookie", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "spike_refresh",
			Value:    strconv.FormatInt(time.Now().UnixNano(), 36),
			Path:     "/api/v1",
			MaxAge:   600,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})
		writeJSON(w, map[string]any{"set": true})
	})

	// Confirma se o browser enviou o cookie através do rewrite.
	mux.HandleFunc("GET /api/v1/cookie", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("spike_refresh")
		writeJSON(w, map[string]any{"received": err == nil, "value": cookieValue(c)})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	slog.Info("a arrancar", "port", port)
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      150 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("servidor parou", "err", err)
		os.Exit(1)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func cookieValue(c *http.Cookie) string {
	if c == nil {
		return ""
	}
	return c.Value
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("pedido", "method", r.Method, "path", r.URL.Path, "ms", time.Since(t).Milliseconds())
	})
}
