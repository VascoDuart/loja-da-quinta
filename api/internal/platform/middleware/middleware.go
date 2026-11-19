// Package middleware tem os middlewares HTTP transversais da API.
// A ordem está documentada em docs/architecture/backend.md.
package middleware

import "net/http"

// Middleware envolve um handler HTTP.
type Middleware func(http.Handler) http.Handler

// Chain aplica os middlewares pela ordem dada: o primeiro é o mais exterior,
// ou seja, o primeiro a ver o pedido.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// statusRecorder guarda o status e o tamanho da resposta para o log.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Unwrap permite ao http.ResponseController chegar ao writer original.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
