// Package logging cria o logger estruturado (slog) da API e guarda o
// request ID no contexto, para todas as linhas de um pedido o incluírem.
package logging

import (
	"context"
	"io"
	"log/slog"
)

// New cria um logger em JSON (staging e produção) ou texto (desenvolvimento).
func New(w io.Writer, level string, json bool) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	var h slog.Handler
	if json {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}
	return slog.New(contextHandler{h})
}

func parseLevel(s string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(s)); err != nil {
		return slog.LevelInfo
	}
	return l
}

type ctxKey struct{}

// WithRequestID devolve um contexto com o request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// RequestID devolve o request ID do contexto, ou "" se não existir.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// contextHandler acrescenta o request_id a cada linha registada com
// slog.*Context(ctx, ...).
type contextHandler struct{ slog.Handler }

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}
