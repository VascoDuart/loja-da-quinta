# Atalhos de desenvolvimento. No Windows, usa Git Bash com make
# (por exemplo, via `winget install ezwinports.make`) ou corre os comandos à mão.

.DEFAULT_GOAL := help

GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
SQLC_IMAGE    := sqlc/sqlc:1.31.1

.PHONY: help up down db-up db-down db-reset db-psql \
        api-gen api-sqlc api-test api-test-integration api-lint api-run \
        web-install web-dev web-test web-lint web-build

help: ## Mostra os alvos disponíveis
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-22s %s\n", $$1, $$2}'

# --- Ambiente local ---------------------------------------------------------

up: ## Arranca PostgreSQL, migrações e API (docker compose)
	docker compose up -d --build

down: ## Para tudo (mantém os dados)
	docker compose down

db-up: ## Arranca só o PostgreSQL local
	docker compose up -d db

db-down: ## Para o PostgreSQL local (mantém os dados)
	docker compose down

db-reset: ## Apaga os dados locais e recria os papéis
	docker compose down -v
	docker compose up -d db

db-psql: ## Abre o psql como readonly
	docker compose exec db psql -U readonly -d loja

# --- API (Go) ---------------------------------------------------------------

api-gen: ## Gera o código a partir da OpenAPI (oapi-codegen)
	cd api && go tool oapi-codegen -config oapi-codegen.yaml ../openapi/openapi.yaml

api-sqlc: ## Gera o código das queries SQL (sqlc, via Docker)
	docker run --rm -v "$(CURDIR)/api:/src" -w /src $(SQLC_IMAGE) generate

api-test: ## Testes unitários da API
	cd api && go test -cover ./...

api-test-integration: ## Testes de integração (precisa de Docker)
	cd api && go test -tags=integration -count=1 ./test/...

api-lint: ## golangci-lint (inclui gosec e depguard)
	cd api && $(GOLANGCI_LINT) run ./...

api-run: ## Corre a API localmente contra o PostgreSQL do compose
	cd api && DATABASE_URL="postgres://app:$${DB_APP_PASSWORD:-troca-me-app}@localhost:5432/loja?sslmode=disable" go run ./cmd/api

# --- Frontend (React + TS) --------------------------------------------------

web-install: ## Instala as dependências exatas do package-lock.json
	cd web && npm ci

web-dev: ## Servidor de desenvolvimento em http://localhost:5173 (/api vai para a API local)
	cd web && npm run dev

web-test: ## Testes do frontend (Vitest)
	cd web && npm test

web-lint: ## ESLint e verificação de tipos
	cd web && npm run lint && npm run typecheck

web-build: ## Build de produção em web/dist
	cd web && npm run build
