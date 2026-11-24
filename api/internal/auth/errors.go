package auth

import "errors"

var (
	// ErrInvalidCredentials: e-mail ou palavra-passe errados (mensagem genérica).
	ErrInvalidCredentials = errors.New("credenciais inválidas")
	// ErrTooManyAttempts: demasiados logins falhados para o IP ou a conta.
	ErrTooManyAttempts = errors.New("demasiadas tentativas")
	// ErrSessionInvalid: o refresh token não existe, expirou ou foi revogado.
	ErrSessionInvalid = errors.New("sessão inválida")
	// ErrRefreshReused: apareceu um refresh token já usado. A família inteira
	// foi revogada: alguém pode ter roubado o token.
	ErrRefreshReused = errors.New("refresh token reutilizado")
)
