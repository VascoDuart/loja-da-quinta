package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
	"github.com/VascoDuart/loja-da-quinta/api/internal/users"
)

// fakeUsers aceita uma única conta.
type fakeUsers struct {
	user     users.User
	password string
}

func (f *fakeUsers) Authenticate(_ context.Context, email, password string) (users.User, error) {
	if users.NormalizeEmail(email) == f.user.Email && password == f.password && f.user.Active {
		return f.user, nil
	}
	return users.User{}, users.ErrInvalidCredentials
}

func (f *fakeUsers) GetActive(_ context.Context, id uuid.UUID) (users.User, error) {
	if id == f.user.ID && f.user.Active {
		return f.user, nil
	}
	return users.User{}, users.ErrNotFound
}

// memStore implementa SessionStore em memória, com a mesma semântica do
// PostgresStore.
type memStore struct {
	mu     sync.Mutex
	tokens []*memToken
}

type memToken struct {
	userID, familyID uuid.UUID
	hash             []byte
	expiresAt        time.Time
	revoked          bool
}

func (m *memStore) find(hash []byte) *memToken {
	for _, t := range m.tokens {
		if bytes.Equal(t.hash, hash) {
			return t
		}
	}
	return nil
}

func (m *memStore) Create(_ context.Context, userID, familyID uuid.UUID, hash []byte, exp time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens = append(m.tokens, &memToken{userID: userID, familyID: familyID, hash: hash, expiresAt: exp})
	return nil
}

func (m *memStore) Rotate(_ context.Context, oldHash, newHash []byte, now, exp time.Time) (uuid.UUID, uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.find(oldHash)
	if t == nil {
		return uuid.Nil, uuid.Nil, ErrSessionInvalid
	}
	if t.revoked {
		m.revokeFamily(t.familyID)
		return t.userID, t.familyID, ErrRefreshReused
	}
	if !now.Before(t.expiresAt) {
		return uuid.Nil, uuid.Nil, ErrSessionInvalid
	}
	t.revoked = true
	m.tokens = append(m.tokens, &memToken{userID: t.userID, familyID: t.familyID, hash: newHash, expiresAt: exp})
	return t.userID, t.familyID, nil
}

func (m *memStore) revokeFamily(f uuid.UUID) {
	for _, t := range m.tokens {
		if t.familyID == f {
			t.revoked = true
		}
	}
}

func (m *memStore) RevokeFamily(_ context.Context, f uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revokeFamily(f)
	return nil
}

func (m *memStore) RevokeFamilyOf(_ context.Context, hash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t := m.find(hash); t != nil {
		m.revokeFamily(t.familyID)
	}
	return nil
}

type fixture struct {
	svc   *Service
	users *fakeUsers
	store *memStore
	now   time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		users: &fakeUsers{
			user:     users.User{ID: uuid.New(), Email: "maria@quinta.pt", Name: "Maria", Role: identity.RoleGestor, Active: true},
			password: "palavra-passe-certa",
		},
		store: &memStore{},
		now:   time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC),
	}
	f.svc = NewService(f.users, f.store, NewEphemeralJWT(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	f.svc.now = func() time.Time { return f.now }
	return f
}

func (f *fixture) login(t *testing.T) Session {
	t.Helper()
	s, err := f.svc.Login(context.Background(), "Maria@Quinta.pt", "palavra-passe-certa", "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLoginIssuesTokens(t *testing.T) {
	f := newFixture(t)
	s := f.login(t)

	if s.ExpiresIn != 15*time.Minute || s.User.ID != f.users.user.ID {
		t.Errorf("sessão inesperada: %+v", s)
	}
	p, err := f.svc.Verify(s.AccessToken)
	if err != nil || p.UserID != f.users.user.ID || p.Role != identity.RoleGestor {
		t.Errorf("o access token devia identificar a Maria como gestora: %+v, %v", p, err)
	}
	if len(f.store.tokens) != 1 || !bytes.Equal(f.store.tokens[0].hash, HashRefresh(s.Refresh.Plain)) {
		t.Fatal("o store devia ter só o hash do refresh token")
	}
	if got := f.store.tokens[0].expiresAt.Sub(f.now); got != 7*24*time.Hour {
		t.Errorf("o refresh devia durar 7 dias, dura %v", got)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Login(context.Background(), "maria@quinta.pt", "errada", "203.0.113.1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("queria ErrInvalidCredentials, deu %v", err)
	}
	if len(f.store.tokens) != 0 {
		t.Error("um login falhado não cria sessão")
	}
}

func TestAccountLockoutAfterFiveFailures(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for i := range failuresPerAccount {
		// IPs diferentes: o bloqueio é por conta.
		if _, err := f.svc.Login(ctx, "maria@quinta.pt", "errada", "198.51.100."+string(rune('1'+i))); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("tentativa %d: %v", i+1, err)
		}
	}
	if _, err := f.svc.Login(ctx, "MARIA@quinta.pt", "palavra-passe-certa", "203.0.113.50"); !errors.Is(err, ErrTooManyAttempts) {
		t.Errorf("depois de 5 falhas, até a palavra-passe certa devia ser recusada; deu %v", err)
	}
	if _, err := f.svc.Login(ctx, "outra@quinta.pt", "x", "203.0.113.50"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("outra conta não devia ser afetada; deu %v", err)
	}
}

func TestLockoutAppliesToUnknownAccounts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for range failuresPerAccount {
		_, _ = f.svc.Login(ctx, "ninguem@quinta.pt", "x", "")
	}
	if _, err := f.svc.Login(ctx, "ninguem@quinta.pt", "x", ""); !errors.Is(err, ErrTooManyAttempts) {
		t.Errorf("um e-mail inexistente também é bloqueado, para o bloqueio não revelar que contas existem; deu %v", err)
	}
}

func TestIPLimitCountsOnlyFailures(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	ip := "192.0.2.7"
	for range 30 {
		if _, err := f.svc.Login(ctx, "maria@quinta.pt", "palavra-passe-certa", ip); err != nil {
			t.Fatalf("logins certos nunca são travados: %v", err)
		}
	}
	for i := range failuresPerIP {
		email := "conta" + string(rune('a'+i)) + "@quinta.pt"
		_, _ = f.svc.Login(ctx, email, "x", ip)
	}
	if _, err := f.svc.Login(ctx, "maria@quinta.pt", "palavra-passe-certa", ip); !errors.Is(err, ErrTooManyAttempts) {
		t.Errorf("depois de 20 falhas no mesmo IP devia dar ErrTooManyAttempts; deu %v", err)
	}
}

func TestSuccessfulLoginResetsAccountFailures(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for range failuresPerAccount - 1 {
		_, _ = f.svc.Login(ctx, "maria@quinta.pt", "errada", "")
	}
	f.login(t)
	for range failuresPerAccount - 1 {
		_, _ = f.svc.Login(ctx, "maria@quinta.pt", "errada", "")
	}
	if _, err := f.svc.Login(ctx, "maria@quinta.pt", "palavra-passe-certa", ""); err != nil {
		t.Errorf("as falhas antes de um login certo não deviam contar: %v", err)
	}
}

func TestRefreshRotates(t *testing.T) {
	f := newFixture(t)
	first := f.login(t)
	f.now = f.now.Add(20 * time.Minute)

	second, err := f.svc.Refresh(context.Background(), first.Refresh.Plain)
	if err != nil {
		t.Fatal(err)
	}
	if second.Refresh.Plain == first.Refresh.Plain {
		t.Error("cada renovação tem de dar um refresh token novo")
	}
	if _, err := f.svc.Verify(second.AccessToken); err != nil {
		t.Errorf("o access token novo devia ser válido: %v", err)
	}
	if _, err := f.svc.Refresh(context.Background(), second.Refresh.Plain); err != nil {
		t.Errorf("o token novo devia renovar: %v", err)
	}
}

func TestRefreshReuseRevokesFamily(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	stolen := f.login(t)

	legit, err := f.svc.Refresh(ctx, stolen.Refresh.Plain) // o utilizador renova
	if err != nil {
		t.Fatal(err)
	}
	// O atacante usa o token antigo, que já foi rodado.
	if _, err := f.svc.Refresh(ctx, stolen.Refresh.Plain); !errors.Is(err, ErrRefreshReused) {
		t.Fatalf("queria ErrRefreshReused, deu %v", err)
	}
	// A sessão inteira acabou, também para o utilizador legítimo.
	if _, err := f.svc.Refresh(ctx, legit.Refresh.Plain); !errors.Is(err, ErrRefreshReused) {
		t.Errorf("o token legítimo também devia estar revogado; deu %v", err)
	}
}

func TestRefreshOtherSessionsSurviveReuse(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	tablet := f.login(t)
	phone := f.login(t)

	_, _ = f.svc.Refresh(ctx, tablet.Refresh.Plain)
	_, _ = f.svc.Refresh(ctx, tablet.Refresh.Plain) // reutilização na sessão do tablet

	if _, err := f.svc.Refresh(ctx, phone.Refresh.Plain); err != nil {
		t.Errorf("a sessão do telemóvel é outra família e não devia ser afetada: %v", err)
	}
}

func TestRefreshInvalidCases(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	s := f.login(t)

	if _, err := f.svc.Refresh(ctx, ""); !errors.Is(err, ErrSessionInvalid) {
		t.Errorf("sem cookie: %v", err)
	}
	if _, err := f.svc.Refresh(ctx, "token-inventado"); !errors.Is(err, ErrSessionInvalid) {
		t.Errorf("token desconhecido: %v", err)
	}

	f.now = f.now.Add(RefreshTokenTTL)
	if _, err := f.svc.Refresh(ctx, s.Refresh.Plain); !errors.Is(err, ErrSessionInvalid) {
		t.Errorf("ao fim de 7 dias sem renovar a sessão devia ter acabado: %v", err)
	}
}

func TestRefreshEndsSessionOfDeactivatedUser(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	s := f.login(t)
	f.users.user.Active = false

	if _, err := f.svc.Refresh(ctx, s.Refresh.Plain); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("utilizador desativado não renova: %v", err)
	}
	for _, tok := range f.store.tokens {
		if !tok.revoked {
			t.Error("a família devia ter ficado revogada")
		}
	}
}

func TestLogoutRevokesFamily(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	s := f.login(t)
	rotated, _ := f.svc.Refresh(ctx, s.Refresh.Plain)

	if err := f.svc.Logout(ctx, rotated.Refresh.Plain); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Refresh(ctx, rotated.Refresh.Plain); err == nil {
		t.Error("depois do logout o refresh não pode renovar")
	}
	if err := f.svc.Logout(ctx, ""); err != nil {
		t.Errorf("logout sem sessão não é erro: %v", err)
	}
}

func TestMe(t *testing.T) {
	f := newFixture(t)
	p := identity.Principal{UserID: f.users.user.ID, Role: identity.RoleGestor}
	if u, err := f.svc.Me(context.Background(), p); err != nil || u.Email != "maria@quinta.pt" {
		t.Errorf("Me = %+v, %v", u, err)
	}
	f.users.user.Active = false
	if _, err := f.svc.Me(context.Background(), p); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Errorf("utilizador desativado: %v", err)
	}
}
