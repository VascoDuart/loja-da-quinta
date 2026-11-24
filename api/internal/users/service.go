package users

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// Service tem os casos de uso dos utilizadores.
//
// Os métodos de gestão recebem quem os pede (actor) e voltam a verificar o
// papel: o RBAC da API já o fez, mas se outro módulo ou um bug chamar o
// serviço por outro caminho, a regra continua a valer.
type Service struct {
	repo     Repo
	hasher   Hasher
	sessions SessionRevoker
	// dummyHash serve para um e-mail desconhecido demorar o mesmo que uma
	// palavra-passe errada: sem isso, o tempo de resposta revelaria que
	// contas existem.
	dummyHash string
}

// NewService cria o serviço. Calcula logo o hash de referência. sessions
// pode ser nil só em comandos que não editam utilizadores (createuser).
func NewService(ctx context.Context, repo Repo, hasher Hasher, sessions SessionRevoker) (*Service, error) {
	dummy, err := hasher.Hash(ctx, "palavra-passe-que-nunca-coincide")
	if err != nil {
		return nil, fmt.Errorf("calcular o hash de referência: %w", err)
	}
	return &Service{repo: repo, hasher: hasher, sessions: sessions, dummyHash: dummy}, nil
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

// Create cria um utilizador sem verificar quem pede: só para o comando
// createuser, que corre com acesso direto à base de dados.
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

// List devolve todos os utilizadores (só o gestor).
func (s *Service) List(ctx context.Context, actor identity.Principal) ([]User, error) {
	if err := requireGestor(actor); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

// CreateBy cria um utilizador a pedido de um gestor.
func (s *Service) CreateBy(ctx context.Context, actor identity.Principal, n NewUser) (User, error) {
	if err := requireGestor(actor); err != nil {
		return User{}, err
	}
	return s.Create(ctx, n)
}

// Update edita um utilizador a pedido de um gestor. Desativar, mudar o papel
// ou a palavra-passe termina as sessões dele.
func (s *Service) Update(ctx context.Context, actor identity.Principal, id uuid.UUID, u Update) (User, error) {
	if err := requireGestor(actor); err != nil {
		return User{}, err
	}
	if u.Name != nil {
		name := strings.TrimSpace(*u.Name)
		u.Name = &name
	}
	if err := u.Validate(); err != nil {
		return User{}, err
	}
	if id == actor.UserID && ((u.Active != nil && !*u.Active) || (u.Role != nil && *u.Role != identity.RoleGestor)) {
		return User{}, ErrSelfLockout
	}

	before, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	changes := Changes{Name: u.Name, Role: u.Role, Active: u.Active}
	if u.Password != nil {
		hash, err := s.hasher.Hash(ctx, *u.Password)
		if err != nil {
			return User{}, fmt.Errorf("calcular o hash: %w", err)
		}
		changes.PasswordHash = &hash
	}

	after, err := s.repo.Update(ctx, id, changes)
	if err != nil {
		return User{}, err
	}
	// Depois de gravar: se a revogação falhar, a renovação já recusa contas
	// desativadas, e o erro chega ao gestor para tentar de novo.
	if u.EndsSessions(before) {
		if err := s.sessions.RevokeAllForUser(ctx, id); err != nil {
			return User{}, fmt.Errorf("terminar as sessões: %w", err)
		}
	}
	return after, nil
}

func requireGestor(actor identity.Principal) error {
	if actor.Role != identity.RoleGestor {
		return identity.ErrForbidden
	}
	return nil
}
