package users

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/password"
)

// memRepo é um Repo em memória para os testes do serviço.
type memRepo struct{ byEmail map[string]User }

func newMemRepo() *memRepo { return &memRepo{byEmail: map[string]User{}} }

func (m *memRepo) FindByEmail(_ context.Context, email string) (User, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (m *memRepo) FindByID(_ context.Context, id uuid.UUID) (User, error) {
	for _, u := range m.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return User{}, ErrNotFound
}

func (m *memRepo) Create(_ context.Context, n NewRecord) (User, error) {
	if _, ok := m.byEmail[n.Email]; ok {
		return User{}, ErrEmailTaken
	}
	u := User{ID: uuid.New(), Email: n.Email, Name: n.Name, Role: n.Role, Active: true, PasswordHash: n.PasswordHash}
	m.byEmail[n.Email] = u
	return u, nil
}

func (m *memRepo) List(context.Context) ([]User, error) {
	out := make([]User, 0, len(m.byEmail))
	for _, u := range m.byEmail {
		out = append(out, u)
	}
	return out, nil
}

func (m *memRepo) Update(ctx context.Context, id uuid.UUID, c Changes) (User, error) {
	u, err := m.FindByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	if c.Name != nil {
		u.Name = *c.Name
	}
	if c.Role != nil {
		u.Role = *c.Role
	}
	if c.Active != nil {
		u.Active = *c.Active
	}
	if c.PasswordHash != nil {
		u.PasswordHash = *c.PasswordHash
	}
	m.byEmail[u.Email] = u
	return u, nil
}

// revoker regista para que utilizadores se terminaram as sessões.
type revoker struct{ revoked []uuid.UUID }

func (r *revoker) RevokeAllForUser(_ context.Context, id uuid.UUID) error {
	r.revoked = append(r.revoked, id)
	return nil
}

// countingHasher conta as verificações, para provar que um e-mail
// desconhecido também calcula um hash.
type countingHasher struct {
	*password.Hasher
	verifies int
}

func (c *countingHasher) Verify(ctx context.Context, plain, encoded string) (bool, error) {
	c.verifies++
	return c.Hasher.Verify(ctx, plain, encoded)
}

func newService(t *testing.T) (*Service, *memRepo, *countingHasher) {
	t.Helper()
	repo := newMemRepo()
	h := &countingHasher{Hasher: password.New(password.Params{Memory: 64, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}, 2)}
	svc, err := NewService(context.Background(), repo, h, &revoker{})
	if err != nil {
		t.Fatal(err)
	}
	return svc, repo, h
}

const goodPassword = "uma-palavra-passe-longa"

func TestCreateAndAuthenticate(t *testing.T) {
	svc, _, _ := newService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, NewUser{Email: "  Maria@Quinta.PT ", Name: " Maria ", Role: identity.RoleGestor, Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}
	if created.Email != "maria@quinta.pt" || created.Name != "Maria" {
		t.Errorf("e-mail e nome deviam vir normalizados: %+v", created)
	}
	if created.PasswordHash == goodPassword {
		t.Fatal("a palavra-passe nunca se guarda em claro")
	}

	u, err := svc.Authenticate(ctx, "MARIA@quinta.pt", goodPassword)
	if err != nil || u.ID != created.ID {
		t.Errorf("o login devia aceitar o e-mail com outras maiúsculas: %v", err)
	}
}

func TestAuthenticateFailuresAreIndistinguishable(t *testing.T) {
	svc, repo, h := newService(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, NewUser{Email: "ze@quinta.pt", Name: "Zé", Role: identity.RoleFuncionario, Password: goodPassword}); err != nil {
		t.Fatal(err)
	}
	inactive, _ := svc.Create(ctx, NewUser{Email: "antigo@quinta.pt", Name: "Antigo", Role: identity.RoleFuncionario, Password: goodPassword})
	inactive.Active = false
	repo.byEmail[inactive.Email] = inactive

	cases := map[string][2]string{
		"palavra-passe errada": {"ze@quinta.pt", "outra-palavra-passe"},
		"e-mail desconhecido":  {"ninguem@quinta.pt", goodPassword},
		"conta desativada":     {"antigo@quinta.pt", goodPassword},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			before := h.verifies
			if _, err := svc.Authenticate(ctx, c[0], c[1]); !errors.Is(err, ErrInvalidCredentials) {
				t.Errorf("queria ErrInvalidCredentials, deu %v", err)
			}
			if h.verifies != before+1 {
				t.Error("todos os casos têm de calcular exatamente um hash, para demorarem o mesmo")
			}
		})
	}
}

func TestCreateValidates(t *testing.T) {
	svc, _, _ := newService(t)
	ctx := context.Background()
	cases := map[string]NewUser{
		"e-mail sem @":              {Email: "maria", Name: "M", Role: identity.RoleGestor, Password: goodPassword},
		"nome vazio":                {Email: "m@q.pt", Name: "  ", Role: identity.RoleGestor, Password: goodPassword},
		"papel inexistente":         {Email: "m@q.pt", Name: "M", Role: "admin", Password: goodPassword},
		"palavra-passe curta":       {Email: "m@q.pt", Name: "M", Role: identity.RoleGestor, Password: "curta"},
		"palavra-passe enorme":      {Email: "m@q.pt", Name: "M", Role: identity.RoleGestor, Password: string(make([]byte, 129))},
		"11 caracteres com acentos": {Email: "m@q.pt", Name: "M", Role: identity.RoleGestor, Password: "ááááááááááá"},
	}
	for name, n := range cases {
		t.Run(name, func(t *testing.T) {
			var ve *ValidationError
			if _, err := svc.Create(ctx, n); !errors.As(err, &ve) {
				t.Errorf("queria ValidationError, deu %v", err)
			}
		})
	}

	if _, err := svc.Create(ctx, NewUser{Email: "m@q.pt", Name: "M", Role: identity.RoleGestor, Password: goodPassword}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, NewUser{Email: "M@Q.PT", Name: "M", Role: identity.RoleGestor, Password: goodPassword}); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("e-mail repetido devia dar ErrEmailTaken, deu %v", err)
	}
}

func TestGetActive(t *testing.T) {
	svc, repo, _ := newService(t)
	ctx := context.Background()
	u, _ := svc.Create(ctx, NewUser{Email: "m@q.pt", Name: "M", Role: identity.RoleGestor, Password: goodPassword})
	if _, err := svc.GetActive(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	u.Active = false
	repo.byEmail[u.Email] = u
	if _, err := svc.GetActive(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("utilizador desativado devia dar ErrNotFound, deu %v", err)
	}
}
