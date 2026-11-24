// Package identity tem quem faz o pedido (o principal autenticado) e os
// papéis. Fica na plataforma para qualquer módulo o poder ler do contexto
// sem importar o módulo auth.
package identity

import (
	"context"

	"github.com/google/uuid"
)

// Role é o papel de um utilizador interno (docs/security/authz-matrix.md).
type Role string

const (
	RoleFuncionario Role = "funcionario"
	RoleGestor      Role = "gestor"
)

// Valid indica se o papel existe.
func (r Role) Valid() bool { return r == RoleFuncionario || r == RoleGestor }

// Principal é o utilizador autenticado de um pedido.
type Principal struct {
	UserID uuid.UUID
	Role   Role
}

type ctxKey struct{}

// With devolve um contexto com o principal.
func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// From devolve o principal do pedido, se houver sessão.
func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
