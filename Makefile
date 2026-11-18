# Atalhos de desenvolvimento. No Windows, usa Git Bash com make
# (por exemplo, via `winget install ezwinports.make`) ou corre os comandos à mão.
#
# Os alvos de api/ e web/ (gen, test, lint, dev) são acrescentados
# quando esses projetos forem criados.

.DEFAULT_GOAL := help

.PHONY: help db-up db-down db-reset db-psql

help: ## Mostra os alvos disponíveis
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-10s %s\n", $$1, $$2}'

db-up: ## Arranca o PostgreSQL local
	docker compose up -d db

db-down: ## Para o PostgreSQL local (mantém os dados)
	docker compose down

db-reset: ## Apaga os dados locais e recria os papéis
	docker compose down -v
	docker compose up -d db

db-psql: ## Abre o psql como readonly
	docker compose exec db psql -U readonly -d loja
