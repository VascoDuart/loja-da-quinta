package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/db/sqlcgen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// PostgresRepo implementa Repo com as queries geradas pelo sqlc
// (queries/users.sql).
type PostgresRepo struct{ q *sqlcgen.Queries }

// NewPostgresRepo cria o repositório sobre um pool ou uma transação.
func NewPostgresRepo(db sqlcgen.DBTX) *PostgresRepo { return &PostgresRepo{q: sqlcgen.New(db)} }

var _ Repo = (*PostgresRepo)(nil)

func (r *PostgresRepo) FindByEmail(ctx context.Context, email string) (User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, notFound(err)
	}
	return toUser(row.ID, row.Email, row.Name, row.PasswordHash, row.Role, row.Active), nil
}

func (r *PostgresRepo) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return User{}, notFound(err)
	}
	return toUser(row.ID, row.Email, row.Name, row.PasswordHash, row.Role, row.Active), nil
}

func (r *PostgresRepo) Create(ctx context.Context, n NewRecord) (User, error) {
	row, err := r.q.CreateUser(ctx, sqlcgen.CreateUserParams{
		Email:        n.Email,
		Name:         n.Name,
		PasswordHash: n.PasswordHash,
		Role:         string(n.Role),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return User{}, ErrEmailTaken
		}
		return User{}, fmt.Errorf("criar o utilizador: %w", err)
	}
	return toUser(row.ID, row.Email, row.Name, row.PasswordHash, row.Role, row.Active), nil
}

func toUser(id uuid.UUID, email, name, hash, role string, active bool) User {
	return User{ID: id, Email: email, Name: name, PasswordHash: hash, Role: identity.Role(role), Active: active}
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("ler o utilizador: %w", err)
}
