package users

import (
	"context"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// Repo guarda os utilizadores.
type Repo interface {
	// FindByEmail devolve ErrNotFound se não existir.
	FindByEmail(ctx context.Context, email string) (User, error)
	// FindByID devolve ErrNotFound se não existir.
	FindByID(ctx context.Context, id uuid.UUID) (User, error)
	// Create devolve ErrEmailTaken se o e-mail já existir.
	Create(ctx context.Context, u NewRecord) (User, error)
}

// NewRecord é o que se grava ao criar um utilizador: já com o hash.
type NewRecord struct {
	Email        string
	Name         string
	Role         identity.Role
	PasswordHash string
}

// Hasher calcula e verifica hashes de palavras-passe (argon2id).
type Hasher interface {
	Hash(ctx context.Context, plain string) (string, error)
	Verify(ctx context.Context, plain, encoded string) (bool, error)
}
