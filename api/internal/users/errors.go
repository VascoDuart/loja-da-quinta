package users

import (
	"errors"
)

var (
	// ErrInvalidCredentials é o único erro do login: e-mail desconhecido,
	// palavra-passe errada e conta desativada dão todos o mesmo, para não
	// revelar que contas existem.
	ErrInvalidCredentials = errors.New("credenciais inválidas")

	// ErrNotFound indica que o utilizador não existe (ou, em GetActive, que
	// está desativado).
	ErrNotFound = errors.New("utilizador não encontrado")

	// ErrEmailTaken indica que já existe um utilizador com o e-mail.
	ErrEmailTaken = errors.New("já existe um utilizador com este e-mail")

	// ErrSelfLockout: o gestor tentou desativar a própria conta ou tirar-lhe
	// o papel de gestor. Assim há sempre pelo menos um gestor ativo.
	ErrSelfLockout = errors.New("não podes desativar nem despromover a tua própria conta")
)

// ValidationError indica dados inválidos num campo. A mensagem é para
// mostrar a quem usa a app.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }
