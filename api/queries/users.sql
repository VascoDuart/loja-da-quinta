-- Queries do módulo users. Só este módulo lê ou escreve a tabela users.

-- name: GetUserByEmail :one
SELECT id, email, name, password_hash, role, active
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, name, password_hash, role, active
FROM users
WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (email, name, password_hash, role)
VALUES ($1, $2, $3, $4)
RETURNING id, email, name, password_hash, role, active;
