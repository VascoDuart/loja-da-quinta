-- Utilizadores internos e refresh tokens (#8, ADR-0005).
--
-- O e-mail guarda-se já normalizado em minúsculas (o CHECK garante-o) em vez
-- de usar citext: assim não é preciso instalar extensões, o que o papel
-- migrator não pode fazer na Neon.

-- +goose Up
CREATE TABLE users (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  email         text        NOT NULL UNIQUE
                            CHECK (email = lower(email) AND length(email) BETWEEN 3 AND 254),
  name          text        NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
  password_hash text        NOT NULL,
  role          text        NOT NULL CHECK (role IN ('gestor', 'funcionario')),
  active        boolean     NOT NULL DEFAULT true,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER users_set_updated_at
  BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Só o hash SHA-256 do refresh token: quem ler a tabela não consegue usar os
-- tokens. Todos os tokens de uma sessão partilham o family_id, para a
-- deteção de reutilização revogar a sessão inteira.
CREATE TABLE refresh_tokens (
  id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  token_hash bytea       NOT NULL UNIQUE CHECK (length(token_hash) = 32),
  family_id  uuid        NOT NULL,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_family_active_idx ON refresh_tokens (family_id) WHERE revoked_at IS NULL;
CREATE INDEX refresh_tokens_user_active_idx ON refresh_tokens (user_id) WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE refresh_tokens;
DROP TABLE users;
