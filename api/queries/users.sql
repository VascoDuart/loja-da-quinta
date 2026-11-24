-- Queries do módulo users. Só este módulo lê ou escreve a tabela users.

-- name: GetUserByEmail :one
SELECT id, email, name, password_hash, role, active, created_at, updated_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, name, password_hash, role, active, created_at, updated_at
FROM users
WHERE id = $1;

-- name: ListUsers :many
SELECT id, email, name, password_hash, role, active, created_at, updated_at
FROM users
ORDER BY active DESC, lower(name), email;

-- name: CreateUser :one
INSERT INTO users (email, name, password_hash, role)
VALUES ($1, $2, $3, $4)
RETURNING id, email, name, password_hash, role, active, created_at, updated_at;

-- Os campos a NULL ficam como estão.
-- name: UpdateUser :one
UPDATE users SET
  name          = COALESCE(sqlc.narg('name'), name),
  role          = COALESCE(sqlc.narg('role'), role),
  active        = COALESCE(sqlc.narg('active'), active),
  password_hash = COALESCE(sqlc.narg('password_hash'), password_hash)
WHERE id = sqlc.arg('id')
RETURNING id, email, name, password_hash, role, active, created_at, updated_at;
