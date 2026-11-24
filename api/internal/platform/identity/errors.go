package identity

import "errors"

// Erros de autenticação e autorização. Ficam aqui, e não no módulo auth,
// para o servidor HTTP os traduzir em 401 e 403 sem importar módulos.
var (
	// ErrUnauthenticated: não há sessão, ou o token é inválido (401).
	ErrUnauthenticated = errors.New("sem sessão")
	// ErrTokenExpired: o access token expirou; o cliente deve renovar (401).
	ErrTokenExpired = errors.New("o access token expirou")
	// ErrForbidden: há sessão, mas o papel não chega (403).
	ErrForbidden = errors.New("sem permissão")
)
