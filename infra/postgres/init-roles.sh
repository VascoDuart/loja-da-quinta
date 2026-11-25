#!/bin/sh
# Cria os papéis da aplicação na primeira inicialização do PostgreSQL local.
# Em staging e produção (Neon), os mesmos papéis são criados à mão, uma vez,
# com palavras-passe guardadas nos segredos do GitHub e da Render.
#
#   migrator  dono do schema "loja"; corre as migrações (DDL)
#   app       usado pela API; só DML (as migrações restringem stock_movements e audit_log)
#   readonly  só leitura; para o DBeaver e diagnóstico
set -eu

psql -v ON_ERROR_STOP=1 \
  --username "$POSTGRES_USER" \
  --dbname "$POSTGRES_DB" \
  -v migrator_pw="$DB_MIGRATOR_PASSWORD" \
  -v app_pw="$DB_APP_PASSWORD" \
  -v readonly_pw="$DB_READONLY_PASSWORD" \
  -v db_name="$POSTGRES_DB" <<'SQL'
CREATE ROLE migrator LOGIN PASSWORD :'migrator_pw';
CREATE ROLE app      LOGIN PASSWORD :'app_pw';
CREATE ROLE readonly LOGIN PASSWORD :'readonly_pw';

REVOKE ALL ON DATABASE :"db_name" FROM PUBLIC;
GRANT CONNECT ON DATABASE :"db_name" TO migrator, app, readonly;
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
SQL
