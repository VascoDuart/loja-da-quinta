-- Papéis e schema da aplicação numa base de dados Neon (staging ou produção).
-- Equivale a infra/postgres/init-roles.sh, que faz o mesmo no compose local.
--
-- Corre uma vez por branch da Neon, ligado como dono da base de dados
-- (loja_owner), com ligação direta (sem pooling) e as palavras-passe dos três
-- papéis passadas por variável. Nunca ficam num ficheiro nem no histórico:
--
--   psql "$OWNER_URL" -v migrator_pw="$MIGRATOR_PW" -v app_pw="$APP_PW" \
--        -v readonly_pw="$READONLY_PW" -1 -f infra/neon/init-roles.sql
--
--   migrator  dono do schema "loja"; corre as migrações (DDL)
--   app       usado pela API; só DML (a migração da auditoria restringe audit_log)
--   readonly  só leitura; para o DBeaver e diagnóstico
\set ON_ERROR_STOP on

CREATE ROLE migrator LOGIN PASSWORD :'migrator_pw';
CREATE ROLE app      LOGIN PASSWORD :'app_pw';
CREATE ROLE readonly LOGIN PASSWORD :'readonly_pw';

-- O dono tem de herdar e poder assumir o migrator: precisa disso para dar
-- privilégios por omissão e permissões no schema que o migrator possui.
GRANT migrator TO CURRENT_USER WITH INHERIT TRUE, SET TRUE;

REVOKE ALL ON DATABASE loja FROM PUBLIC;
GRANT CONNECT ON DATABASE loja TO migrator, app, readonly;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;

CREATE SCHEMA loja AUTHORIZATION migrator;
GRANT USAGE ON SCHEMA loja TO app, readonly;

ALTER DEFAULT PRIVILEGES FOR ROLE migrator IN SCHEMA loja
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app;
ALTER DEFAULT PRIVILEGES FOR ROLE migrator IN SCHEMA loja
  GRANT USAGE, SELECT ON SEQUENCES TO app;
ALTER DEFAULT PRIVILEGES FOR ROLE migrator IN SCHEMA loja
  GRANT SELECT ON TABLES TO readonly;

ALTER ROLE migrator SET search_path = loja;
ALTER ROLE app      SET search_path = loja;
ALTER ROLE readonly SET search_path = loja;
