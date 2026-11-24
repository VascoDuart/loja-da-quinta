package users

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

type managementFixture struct {
	svc     *Service
	repo    *memRepo
	revoked *revoker
	gestor  identity.Principal
	ze      User
}

func newManagement(t *testing.T) *managementFixture {
	t.Helper()
	svc, repo, _ := newService(t)
	ctx := context.Background()
	maria, err := svc.Create(ctx, NewUser{Email: "maria@quinta.pt", Name: "Maria", Role: identity.RoleGestor, Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}
	ze, err := svc.Create(ctx, NewUser{Email: "ze@quinta.pt", Name: "Zé", Role: identity.RoleFuncionario, Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}
	return &managementFixture{
		svc: svc, repo: repo, revoked: svc.sessions.(*revoker),
		gestor: identity.Principal{UserID: maria.ID, Role: identity.RoleGestor},
		ze:     ze,
	}
}

func ptr[T any](v T) *T { return &v }

func TestOnlyGestorManagesUsers(t *testing.T) {
	f := newManagement(t)
	ctx := context.Background()
	funcionario := identity.Principal{UserID: f.ze.ID, Role: identity.RoleFuncionario}

	if _, err := f.svc.List(ctx, funcionario); !errors.Is(err, identity.ErrForbidden) {
		t.Errorf("List: %v", err)
	}
	if _, err := f.svc.CreateBy(ctx, funcionario, NewUser{Email: "x@q.pt", Name: "X", Role: identity.RoleGestor, Password: goodPassword}); !errors.Is(err, identity.ErrForbidden) {
		t.Errorf("CreateBy: %v", err)
	}
	// Um funcionário não se promove a gestor, nem com o próprio ID.
	if _, err := f.svc.Update(ctx, funcionario, f.ze.ID, Update{Role: ptr(identity.RoleGestor)}); !errors.Is(err, identity.ErrForbidden) {
		t.Errorf("Update: %v", err)
	}
	if _, err := f.svc.List(ctx, identity.Principal{}); !errors.Is(err, identity.ErrForbidden) {
		t.Errorf("sem principal também é recusado: %v", err)
	}
}

func TestGestorListsAndCreates(t *testing.T) {
	f := newManagement(t)
	ctx := context.Background()
	u, err := f.svc.CreateBy(ctx, f.gestor, NewUser{Email: "Ana@Quinta.pt", Name: "Ana", Role: identity.RoleFuncionario, Password: goodPassword})
	if err != nil || u.Email != "ana@quinta.pt" {
		t.Fatalf("CreateBy = %+v, %v", u, err)
	}
	list, err := f.svc.List(ctx, f.gestor)
	if err != nil || len(list) != 3 {
		t.Errorf("List = %d utilizadores, %v", len(list), err)
	}
}

func TestUpdateEndsSessionsOnlyWhenNeeded(t *testing.T) {
	cases := []struct {
		name string
		upd  Update
		ends bool
	}{
		{"mudar só o nome", Update{Name: ptr("José")}, false},
		{"reativar", Update{Active: ptr(true)}, false},
		{"manter o mesmo papel", Update{Role: ptr(identity.RoleFuncionario)}, false},
		{"desativar", Update{Active: ptr(false)}, true},
		{"mudar o papel", Update{Role: ptr(identity.RoleGestor)}, true},
		{"palavra-passe nova", Update{Password: ptr("outra-palavra-passe")}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newManagement(t)
			if _, err := f.svc.Update(context.Background(), f.gestor, f.ze.ID, c.upd); err != nil {
				t.Fatal(err)
			}
			if got := slices.Contains(f.revoked.revoked, f.ze.ID); got != c.ends {
				t.Errorf("terminou as sessões = %v, queria %v", got, c.ends)
			}
		})
	}
}

func TestUpdatePasswordReallyChangesIt(t *testing.T) {
	f := newManagement(t)
	ctx := context.Background()
	if _, err := f.svc.Update(ctx, f.gestor, f.ze.ID, Update{Password: ptr("palavra-passe-nova-123")}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(ctx, "ze@quinta.pt", goodPassword); !errors.Is(err, ErrInvalidCredentials) {
		t.Error("a palavra-passe antiga não devia funcionar")
	}
	if _, err := f.svc.Authenticate(ctx, "ze@quinta.pt", "palavra-passe-nova-123"); err != nil {
		t.Errorf("a nova devia funcionar: %v", err)
	}
}

func TestDeactivatedUserCannotLogIn(t *testing.T) {
	f := newManagement(t)
	ctx := context.Background()
	if _, err := f.svc.Update(ctx, f.gestor, f.ze.ID, Update{Active: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(ctx, "ze@quinta.pt", goodPassword); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("conta desativada: %v", err)
	}
}

func TestGestorCannotLockThemselvesOut(t *testing.T) {
	f := newManagement(t)
	ctx := context.Background()
	for name, upd := range map[string]Update{
		"desativar-se":       {Active: ptr(false)},
		"tirar-se o gestor":  {Role: ptr(identity.RoleFuncionario)},
		"as duas de uma vez": {Active: ptr(false), Name: ptr("Maria")},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.svc.Update(ctx, f.gestor, f.gestor.UserID, upd); !errors.Is(err, ErrSelfLockout) {
				t.Errorf("queria ErrSelfLockout, deu %v", err)
			}
		})
	}
	// Mudar o próprio nome ou a própria palavra-passe pode.
	if _, err := f.svc.Update(ctx, f.gestor, f.gestor.UserID, Update{Name: ptr("Maria Silva")}); err != nil {
		t.Errorf("mudar o próprio nome devia poder: %v", err)
	}
}

func TestUpdateValidation(t *testing.T) {
	f := newManagement(t)
	ctx := context.Background()
	for name, upd := range map[string]Update{
		"vazio":               {},
		"nome em branco":      {Name: ptr("   ")},
		"papel inexistente":   {Role: ptr(identity.Role("admin"))},
		"palavra-passe curta": {Password: ptr("curta")},
	} {
		t.Run(name, func(t *testing.T) {
			var ve *ValidationError
			if _, err := f.svc.Update(ctx, f.gestor, f.ze.ID, upd); !errors.As(err, &ve) {
				t.Errorf("queria ValidationError, deu %v", err)
			}
		})
	}
	if _, err := f.svc.Update(ctx, f.gestor, uuid.New(), Update{Name: ptr("X")}); !errors.Is(err, ErrNotFound) {
		t.Errorf("utilizador inexistente: %v", err)
	}
}
