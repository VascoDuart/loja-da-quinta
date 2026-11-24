package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/db/sqlcgen"
)

// PostgresStore implementa SessionStore com as queries geradas pelo sqlc
// (queries/auth.sql).
type PostgresStore struct{ pool *pgxpool.Pool }

// NewPostgresStore cria o store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

var _ SessionStore = (*PostgresStore)(nil)

func (s *PostgresStore) Create(ctx context.Context, userID, familyID uuid.UUID, hash []byte, expiresAt time.Time) error {
	return sqlcgen.New(s.pool).InsertRefreshToken(ctx, sqlcgen.InsertRefreshTokenParams{
		UserID: userID, TokenHash: hash, FamilyID: familyID, ExpiresAt: expiresAt,
	})
}

// Rotate corre numa transação com a linha do token bloqueada (FOR UPDATE):
// se dois pedidos chegarem com o mesmo token, o segundo espera e depois vê-o
// revogado, por isso conta como reutilização.
func (s *PostgresStore) Rotate(ctx context.Context, oldHash, newHash []byte, now, newExpiresAt time.Time) (userID, familyID uuid.UUID, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("iniciar a transação: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // não faz nada depois do Commit
	q := sqlcgen.New(tx)

	row, err := q.GetRefreshTokenForUpdate(ctx, oldHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, ErrSessionInvalid
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("ler o refresh token: %w", err)
	}

	if row.RevokedAt != nil {
		if _, err := q.RevokeRefreshFamily(ctx, row.FamilyID); err != nil {
			return uuid.Nil, uuid.Nil, fmt.Errorf("revogar a família: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return uuid.Nil, uuid.Nil, fmt.Errorf("revogar a família: %w", err)
		}
		return row.UserID, row.FamilyID, ErrRefreshReused
	}
	if !now.Before(row.ExpiresAt) {
		return uuid.Nil, uuid.Nil, ErrSessionInvalid
	}

	if err := q.RevokeRefreshToken(ctx, row.ID); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("revogar o refresh token: %w", err)
	}
	if err := q.InsertRefreshToken(ctx, sqlcgen.InsertRefreshTokenParams{
		UserID: row.UserID, TokenHash: newHash, FamilyID: row.FamilyID, ExpiresAt: newExpiresAt,
	}); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("gravar o refresh token novo: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("rodar o refresh token: %w", err)
	}
	return row.UserID, row.FamilyID, nil
}

func (s *PostgresStore) RevokeFamily(ctx context.Context, familyID uuid.UUID) error {
	_, err := sqlcgen.New(s.pool).RevokeRefreshFamily(ctx, familyID)
	return err
}

func (s *PostgresStore) RevokeFamilyOf(ctx context.Context, hash []byte) error {
	q := sqlcgen.New(s.pool)
	familyID, err := q.GetRefreshFamilyByHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ler o refresh token: %w", err)
	}
	_, err = q.RevokeRefreshFamily(ctx, familyID)
	return err
}
