package auth

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
	"github.com/VascoDuart/loja-da-quinta/api/internal/users"
)

// Users é o que o auth precisa do módulo users (implementado por
// users.Service).
type Users interface {
	Authenticate(ctx context.Context, email, password string) (users.User, error)
	GetActive(ctx context.Context, id uuid.UUID) (users.User, error)
}

// SessionStore guarda os refresh tokens.
type SessionStore interface {
	// Create grava o primeiro token de uma família (login).
	Create(ctx context.Context, userID, familyID uuid.UUID, hash []byte, expiresAt time.Time) error

	// Rotate troca o token oldHash por newHash, na mesma família, numa
	// transação. Devolve ErrSessionInvalid se oldHash não existir ou tiver
	// expirado, e ErrRefreshReused (depois de revogar a família) se já
	// tiver sido usado.
	Rotate(ctx context.Context, oldHash, newHash []byte, now, newExpiresAt time.Time) (userID, familyID uuid.UUID, err error)

	// RevokeFamily revoga todos os tokens ativos de uma família.
	RevokeFamily(ctx context.Context, familyID uuid.UUID) error

	// RevokeFamilyOf revoga a família do token com este hash (logout). Não
	// faz nada se o token não existir.
	RevokeFamilyOf(ctx context.Context, hash []byte) error
}

// Tokens emite e verifica access tokens.
type Tokens interface {
	Issue(p identity.Principal, now time.Time) (string, error)
	// Verify devolve identity.ErrTokenExpired ou identity.ErrUnauthenticated.
	Verify(token string, now time.Time) (identity.Principal, error)
}
