package users

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Service tem os casos de uso dos utilizadores. A gestão pelo gestor
// (editar, desativar) chega com o #9.
type Service struct {
	repo   Repo
	hasher Hasher
	// dummyHash serve para um e-mail desconhecido demorar o mesmo que uma
	// palavra-passe errada: sem isso, o tempo de resposta revelaria que
	// contas existem.
	dummyHash string
}

// NewService cria o serviço. Calcula logo o hash de referência.
func NewService(ctx context.Context, repo Repo, hasher Hasher) (*Service, error) {
	dummy, err := hasher.Hash(ctx, "palavra-passe-que-nunca-coincide")
	if err != nil {
		return nil, fmt.Errorf("calcular o hash de referência: %w", err)
	}
	return &Service{repo: repo, hasher: hasher, dummyHash: dummy}, nil
}

// Authenticate verifica o e-mail e a palavra-passe. Devolve sempre
// ErrInvalidCredentials quando falha, seja qual for o motivo.
func (s *Service) Authenticate(ctx context.Context, email, password string) (User, error) {
	u, err := s.repo.FindByEmail(ctx, NormalizeEmail(email))
	if errors.Is(err, ErrNotFound) {
		_, _ = s.hasher.Verify(ctx, password, s.dummyHash)
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	ok, err := s.hasher.Verify(ctx, password, u.PasswordHash)
	if err != nil {
		return User{}, fmt.Errorf("verificar a palavra-passe: %w", err)
	}
	// A conta desativada só se verifica depois do hash, para demorar o mesmo.
	if !ok || !u.Active {
		return User{}, ErrInvalidCredentials
	}
	return u, nil
}

// GetActive devolve um utilizador ativo, ou ErrNotFound.
func (s *Service) GetActive(ctx context.Context, id uuid.UUID) (User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	if !u.Active {
		return User{}, ErrNotFound
	}
	return u, nil
}

// Create cria um utilizador. Hoje só é usado pelo comando createuser; o
// endpoint do gestor chega com o #9.
func (s *Service) Create(ctx context.Context, n NewUser) (User, error) {
	n.Email = NormalizeEmail(n.Email)
	n.Name = strings.TrimSpace(n.Name)
	if err := n.Validate(); err != nil {
		return User{}, err
	}
	hash, err := s.hasher.Hash(ctx, n.Password)
	if err != nil {
		return User{}, fmt.Errorf("calcular o hash: %w", err)
	}
	return s.repo.Create(ctx, NewRecord{Email: n.Email, Name: n.Name, Role: n.Role, PasswordHash: hash})
}
