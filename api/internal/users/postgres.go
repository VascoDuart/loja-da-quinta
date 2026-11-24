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
	return toUser(row), nil
}

func (r *PostgresRepo) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return User{}, notFound(err)
	}
	return toUser(row), nil
}

func (r *PostgresRepo) List(ctx context.Context) ([]User, error) {
	rows, err := r.q.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar os utilizadores: %w", err)
	}
	out := make([]User, len(rows))
	for i, row := range rows {
		out[i] = toUser(row)
	}
	return out, nil
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
	return toUser(row), nil
}

func (r *PostgresRepo) Update(ctx context.Context, id uuid.UUID, c Changes) (User, error) {
	var role *string
	if c.Role != nil {
		s := string(*c.Role)
		role = &s
	}
	row, err := r.q.UpdateUser(ctx, sqlcgen.UpdateUserParams{
		ID: id, Name: c.Name, Role: role, Active: c.Active, PasswordHash: c.PasswordHash,
	})
	if err != nil {
		return User{}, notFound(err)
	}
	return toUser(row), nil
}

func toUser(row sqlcgen.User) User {
	return User{
		ID: row.ID, Email: row.Email, Name: row.Name, PasswordHash: row.PasswordHash,
		Role: identity.Role(row.Role), Active: row.Active, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("ler o utilizador: %w", err)
}
