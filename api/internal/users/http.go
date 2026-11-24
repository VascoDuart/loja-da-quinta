package users

import (
	"context"
	"errors"
	"net/http"

	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/problems"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// Endpoints implementa as operações da tag users da OpenAPI.
type Endpoints struct{ svc *Service }

// NewEndpoints cria os handlers.
func NewEndpoints(svc *Service) *Endpoints { return &Endpoints{svc: svc} }

const (
	msgForbidden = "Só o gestor pode gerir utilizadores."
	msgNotFound  = "Este utilizador não existe."
	msgTaken     = "Já existe um utilizador com este e-mail."
	msgSelf      = "Não podes desativar a tua própria conta nem tirar-lhe o papel de gestor."
)

func (h *Endpoints) ListUsers(ctx context.Context, _ gen.ListUsersRequestObject) (gen.ListUsersResponseObject, error) {
	list, err := h.svc.List(ctx, actor(ctx))
	if errors.Is(err, identity.ErrForbidden) {
		return gen.ListUsers403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbidden)), nil
	}
	if err != nil {
		return nil, err
	}
	out := make(gen.ListUsers200JSONResponse, len(list))
	for i, u := range list {
		out[i] = toManaged(u)
	}
	return out, nil
}

func (h *Endpoints) CreateUser(ctx context.Context, req gen.CreateUserRequestObject) (gen.CreateUserResponseObject, error) {
	b := req.Body
	u, err := h.svc.CreateBy(ctx, actor(ctx), NewUser{Email: b.Email, Name: b.Name, Role: identity.Role(b.Role), Password: b.Password})
	var ve *ValidationError
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.CreateUser403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbidden)), nil
	case errors.Is(err, ErrEmailTaken):
		return gen.CreateUser409ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusConflict, msgTaken)), nil
	case errors.As(err, &ve):
		return gen.CreateUser422ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnprocessableEntity, ve.Message)), nil
	case err != nil:
		return nil, err
	}
	return gen.CreateUser201JSONResponse(toManaged(u)), nil
}

func (h *Endpoints) UpdateUser(ctx context.Context, req gen.UpdateUserRequestObject) (gen.UpdateUserResponseObject, error) {
	b := req.Body
	upd := Update{Name: b.Name, Active: b.Active, Password: b.Password}
	if b.Role != nil {
		r := identity.Role(*b.Role)
		upd.Role = &r
	}
	u, err := h.svc.Update(ctx, actor(ctx), req.Id, upd)
	var ve *ValidationError
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.UpdateUser403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbidden)), nil
	case errors.Is(err, ErrNotFound):
		return gen.UpdateUser404ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusNotFound, msgNotFound)), nil
	case errors.Is(err, ErrSelfLockout):
		return gen.UpdateUser409ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusConflict, msgSelf)), nil
	case errors.As(err, &ve):
		return gen.UpdateUser422ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnprocessableEntity, ve.Message)), nil
	case err != nil:
		return nil, err
	}
	return gen.UpdateUser200JSONResponse(toManaged(u)), nil
}

// actor devolve quem faz o pedido. Sem sessão devolve um principal vazio,
// que o serviço recusa (o RBAC da API nunca deixa chegar aqui sem sessão).
func actor(ctx context.Context) identity.Principal {
	p, _ := identity.From(ctx)
	return p
}

func toManaged(u User) gen.ManagedUser {
	return gen.ManagedUser{
		Id: u.ID, Email: u.Email, Name: u.Name, Role: gen.Role(u.Role), Active: u.Active,
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}
