-- Queries do módulo auth. Só este módulo lê ou escreve a tabela refresh_tokens.

-- name: InsertRefreshToken :exec
INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at)
VALUES ($1, $2, $3, $4);

-- Bloqueia a linha até ao fim da transação: dois pedidos com o mesmo token
-- nunca rodam os dois.
-- name: GetRefreshTokenForUpdate :one
SELECT id, user_id, family_id, expires_at, revoked_at
FROM refresh_tokens
WHERE token_hash = $1
FOR UPDATE;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeRefreshFamily :execrows
UPDATE refresh_tokens SET revoked_at = now()
WHERE family_id = $1 AND revoked_at IS NULL;

-- name: GetRefreshFamilyByHash :one
SELECT family_id FROM refresh_tokens WHERE token_hash = $1;

-- Desativar um utilizador, mudar-lhe o papel ou a palavra-passe termina
-- todas as sessões dele.
-- name: RevokeUserRefreshTokens :execrows
UPDATE refresh_tokens SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;
