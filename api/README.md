# api

Backend em Go: monólito modular com arquitetura hexagonal. Ver [docs/architecture/backend.md](../docs/architecture/backend.md).

## Correr

```bash
make up                     # PostgreSQL + migrações + API em http://localhost:8080
curl localhost:8080/api/v1/healthz
```

Ou sem Docker para a API (o PostgreSQL continua no compose):

```bash
make db-up
make api-run
```

## Comandos

| Comando | O que faz |
|---|---|
| `make api-gen` | Gera `internal/httpapi/gen` a partir de `openapi/openapi.yaml` |
| `make api-sqlc` | Gera o código das queries SQL (a partir da #8) |
| `make api-test` | Testes unitários, com cobertura (o CI acrescenta `-race`) |
| `make api-test-integration` | Testes com PostgreSQL real (testcontainers; precisa de Docker) |
| `make api-lint` | golangci-lint, com gosec e depguard |

## Configuração

Tudo por variáveis de ambiente, validadas no arranque. Com um valor inválido, a API recusa arrancar e diz o que está mal.

| Variável | Por omissão | Notas |
|---|---|---|
| `DATABASE_URL` | — | Obrigatória. Fora de `development` exige `sslmode=require` |
| `APP_ENV` | `development` | `development`, `staging` ou `production` |
| `PORT` | `8080` | A Render define-a sozinha |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. Logs em JSON fora de `development` |
| `DATABASE_MAX_CONNS` | `5` | O plano gratuito da Neon tem poucas ligações |
| `TRUSTED_PROXY_HOPS` | `0` | Proxies à frente da API. Vercel + Render = `2` |
| `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` | `10` / `20` | Pedidos por segundo por IP |
| `MAX_BODY_BYTES` | `1048576` | 1 MiB |
| `REQUEST_TIMEOUT` | `10s` | Prazo de cada pedido, propagado até à BD |
| `SHUTDOWN_TIMEOUT` | `15s` | Espera pelos pedidos em curso ao encerrar |

O comando `migrate` usa `MIGRATIONS_DATABASE_URL`, com o papel `migrator`.

## Estrutura

```
api/
├── cmd/
│   ├── api/                arranque e composição (sem framework de DI)
│   └── migrate/            aplica as migrações com o papel migrator
├── internal/
│   ├── platform/
│   │   ├── config/         variáveis de ambiente, validação
│   │   ├── logging/        slog com request_id
│   │   ├── httpx/          erros problem+json (RFC 9457)
│   │   ├── middleware/     recover, request ID, log, cabeçalhos, rate limit, limites
│   │   ├── httpserver/     router, validação OpenAPI, encerramento ordenado
│   │   └── db/             pool pgx
│   ├── httpapi/
│   │   ├── gen/            gerado pelo oapi-codegen (não editar)
│   │   └── server.go       junta os handlers dos módulos
│   └── health/             /healthz e /readyz
├── migrations/             goose (SQL), embebidas no binário
├── queries/                SQL para o sqlc
├── test/integration/       testes com PostgreSQL real
├── oapi-codegen.yaml
├── sqlc.yaml
├── .golangci.yml
└── Dockerfile              multi-stage, distroless, sem root
```
