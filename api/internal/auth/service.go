package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/ratelimit"
	"github.com/VascoDuart/loja-da-quinta/api/internal/users"
)

// Limites de logins falhados (docs/security/controls.md). Só contam as
// falhas: quem acerta na palavra-passe nunca é travado por quem partilha o
// mesmo IP. A chave da conta é o e-mail normalizado, exista ou não, para o
// bloqueio não revelar que contas existem.
const (
	failuresPerAccount = 5
	accountWindow      = 15 * time.Minute
	failuresPerIP      = 20
	ipWindow           = time.Minute
)

// Service tem os casos de uso da sessão.
type Service struct {
	users  Users
	store  SessionStore
	tokens Tokens
	log    *slog.Logger
	now    func() time.Time

	accountFailures *ratelimit.Keyed
	ipFailures      *ratelimit.Keyed
}

// NewService cria o serviço.
func NewService(u Users, store SessionStore, tokens Tokens, log *slog.Logger) *Service {
	return &Service{
		users:           u,
		store:           store,
		tokens:          tokens,
		log:             log,
		now:             time.Now,
		accountFailures: ratelimit.New(rate.Every(accountWindow/failuresPerAccount), failuresPerAccount, accountWindow),
		ipFailures:      ratelimit.New(rate.Every(ipWindow/failuresPerIP), failuresPerIP, 15*time.Minute),
	}
}

// Login verifica as credenciais e abre uma sessão nova (uma família nova de
// refresh tokens).
func (s *Service) Login(ctx context.Context, email, password, ip string) (Session, error) {
	account := users.NormalizeEmail(email)
	if s.accountFailures.Exhausted(account) || s.ipFailures.Exhausted(ip) {
		s.log.WarnContext(ctx, "login recusado: demasiadas tentativas falhadas", "ip", ip)
		return Session{}, ErrTooManyAttempts
	}

	u, err := s.users.Authenticate(ctx, email, password)
	if errors.Is(err, users.ErrInvalidCredentials) {
		s.accountFailures.Allow(account)
		s.ipFailures.Allow(ip)
		// Não se regista o e-mail: pode ser uma palavra-passe escrita no campo errado.
		s.log.WarnContext(ctx, "login falhado", "ip", ip)
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	s.accountFailures.Reset(account)

	now := s.now()
	refresh := NewRefreshToken()
	if err := s.store.Create(ctx, u.ID, uuid.New(), refresh.Hash, now.Add(RefreshTokenTTL)); err != nil {
		return Session{}, fmt.Errorf("gravar a sessão: %w", err)
	}
	s.log.InfoContext(ctx, "login", "user_id", u.ID)
	return s.session(u, refresh, now)
}

// Refresh roda o refresh token e emite um access token novo. Um token já
// usado revoga a sessão inteira.
func (s *Service) Refresh(ctx context.Context, plain string) (Session, error) {
	if plain == "" {
		return Session{}, ErrSessionInvalid
	}
	now := s.now()
	next := NewRefreshToken()
	userID, familyID, err := s.store.Rotate(ctx, HashRefresh(plain), next.Hash, now, now.Add(RefreshTokenTTL))
	if errors.Is(err, ErrRefreshReused) {
		s.log.WarnContext(ctx, "refresh token reutilizado: sessão revogada", "user_id", userID, "family_id", familyID)
		return Session{}, ErrRefreshReused
	}
	if err != nil {
		return Session{}, err
	}

	u, err := s.users.GetActive(ctx, userID)
	if errors.Is(err, users.ErrNotFound) {
		// Desativado (ou apagado) depois do login: a sessão acaba aqui.
		if err := s.store.RevokeFamily(ctx, familyID); err != nil {
			return Session{}, err
		}
		return Session{}, ErrSessionInvalid
	}
	if err != nil {
		return Session{}, err
	}
	return s.session(u, next, now)
}

// Logout revoga a sessão do refresh token. Sem token, não faz nada.
func (s *Service) Logout(ctx context.Context, plain string) error {
	if plain == "" {
		return nil
	}
	return s.store.RevokeFamilyOf(ctx, HashRefresh(plain))
}

// Me devolve o utilizador da sessão atual.
func (s *Service) Me(ctx context.Context, p identity.Principal) (users.User, error) {
	u, err := s.users.GetActive(ctx, p.UserID)
	if errors.Is(err, users.ErrNotFound) {
		return users.User{}, identity.ErrUnauthenticated
	}
	return u, err
}

// Verify verifica um access token (usado pelo middleware).
func (s *Service) Verify(token string) (identity.Principal, error) {
	return s.tokens.Verify(token, s.now())
}

func (s *Service) session(u users.User, refresh RefreshToken, now time.Time) (Session, error) {
	access, err := s.tokens.Issue(identity.Principal{UserID: u.ID, Role: u.Role}, now)
	if err != nil {
		return Session{}, fmt.Errorf("emitir o access token: %w", err)
	}
	return Session{AccessToken: access, ExpiresIn: AccessTokenTTL, User: u, Refresh: refresh}, nil
}
