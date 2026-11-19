package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// AccessLog regista uma linha por pedido: método, caminho, status, tamanho e
// duração. Não regista query strings, cabeçalhos nem corpos, que podem conter
// tokens ou dados pessoais.
func AccessLog(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)

			level := slog.LevelInfo
			if rec.status >= 500 {
				level = slog.LevelError
			}
			log.Log(r.Context(), level, "pedido",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"bytes", rec.bytes,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}
