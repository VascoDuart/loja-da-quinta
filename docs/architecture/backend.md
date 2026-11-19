# Arquitetura do backend

**Monólito modular em Go com arquitetura hexagonal** ([ADR-0001](../adr/0001-monolito-modular.md)). Cada módulo tem as suas camadas e só expõe a interface do seu serviço. Um módulo nunca lê as tabelas de outro.

## Diagrama

```mermaid
flowchart TB
    REQ(["Pedido HTTP /api/v1"])

    subgraph MW["Middleware (por ordem)"]
        direction LR
        M1[Recover] --> M2[Request ID] --> M3[Logging] --> M4[Cabeçalhos] --> M5[Rate limit] --> M6[Tamanho do corpo] --> M7[Timeout] --> M8[JWT] --> M9[RBAC]
    end

    HTTP["Adaptador HTTP<br/><small>handlers gerados (oapi-codegen) · validação contra a OpenAPI · DTO ↔ domínio · erros problem+json</small>"]

    subgraph MOD["Módulos (aplicação · domínio · portas)"]
        direction LR
        AUTH["<b>auth</b><br/>Login · Logout · Refresh<br/><small>Sessão · Credencial</small><br/><i>SessionRepo · Hasher</i>"]
        USERS["<b>users</b><br/>Criar · Editar · Desativar<br/><small>Utilizador · Papel</small><br/><i>UserRepo</i>"]
        CAT["<b>catalog</b><br/>Produtos · Categorias<br/><small>Produto · IVA · Unidade · Época</small><br/><i>ProductRepo</i>"]
        INV["<b>inventory</b><br/>Entrada · Ajuste · Alertas<br/><small>Lote · FEFO · Movimento</small><br/><i>StockRepo · Notifier</i>"]
        SALES["<b>sales</b><br/>Registar · Anular<br/><small>Venda · Linha · Pagamento</small><br/><i>SaleRepo · TxManager</i>"]
        REP["<b>reports</b><br/>Por período · Top produtos<br/><small>modelos de leitura</small><br/><i>ReportQuery</i>"]
        AUD["<b>audit</b><br/>Registar · Pesquisar<br/><small>Evento</small><br/><i>AuditRepo</i>"]
    end

    subgraph AD["Adaptadores (implementam as portas)"]
        direction LR
        PGA["PostgreSQL<br/><small>pgx + sqlc · transações</small>"]
        MA["E-mail<br/><small>Resend</small>"]
        CR["Argon2id · JWT · relógio"]
    end

    PG[("PostgreSQL (Neon)")]
    RS["API do Resend"]
    MAIN["cmd/api/main.go<br/><small>lê a configuração, cria os adaptadores e injeta-os nos módulos</small>"]

    REQ --> MW --> HTTP -- "chama casos de uso" --> MOD
    SALES -- "mesma transação" --> INV
    AD -. "implementam" .-> MOD
    PGA --> PG
    MA --> RS
    MAIN -. "compõe" .-> AD
    MAIN -. "compõe" .-> MOD

    classDef hot fill:#e1eee0,stroke:#2e6a3b,stroke-width:2px,color:#1b221e
    class INV,SALES,M7,M8 hot
```

As dependências apontam para dentro: o domínio não importa nada de HTTP nem de SQL. Os adaptadores implementam as portas, por isso os serviços testam-se com fakes em memória e os repositórios com PostgreSQL real (testcontainers).

`inventory` e `sales` estão em destaque porque partilham a transação da venda: `sales` chama o serviço de `inventory` dentro do mesmo `TxManager`. O job de alertas é uma goroutine agendada no mesmo processo e é o primeiro candidato a serviço separado (v2, com o outbox pattern).

## Organização de um módulo

```
internal/sales/
├── domain.go      entidades, value objects e regras (sem dependências externas)
├── service.go     casos de uso; recebe as portas por injeção
├── ports.go       interfaces que o módulo precisa (SaleRepo, TxManager…)
├── errors.go      erros de domínio tipados
├── postgres.go    adaptador PostgreSQL (usa o código gerado pelo sqlc)
├── http.go        handlers: traduz DTOs e erros
└── *_test.go      unitários (fakes) e de integração (testcontainers)
```

## Middleware

Ordem em `internal/platform/httpserver/server.go`. O primeiro é o mais exterior.

| # | Middleware | Porquê aqui |
|---|---|---|
| 1 | Recover | Primeiro, para apanhar um panic em qualquer camada abaixo |
| 2 | Request ID | Antes do log, para cada linha e cada erro terem o ID |
| 3 | Access log | Regista o status final, incluindo 429 e 500 |
| 4 | Cabeçalhos de segurança | Aplicam-se também às respostas de erro |
| 5 | Rate limit | Antes de qualquer trabalho caro |
| 6 | Tamanho do corpo (1 MiB) | Antes de ler o corpo |
| 7 | Timeout (10 s) | Prazo propagado pelo contexto até à base de dados |
| 8 | Validação OpenAPI | Só nas rotas da spec; recusa pedidos fora do contrato com 400 |
| — | JWT e RBAC | Chegam com a autenticação (#8) |

**Sem CORS:** a API está na mesma origem que o frontend, graças ao rewrite da Vercel ([ADR-0007](../adr/0007-mesma-origem-via-rewrite.md)). Sem cabeçalhos CORS, o browser bloqueia pedidos de outros sites, que é o comportamento que queremos.

## Health checks

| Endpoint | Responde 200 quando | Usado por |
|---|---|---|
| `GET /api/v1/healthz` | O processo está vivo (nunca consulta a BD) | Health check da Render |
| `GET /api/v1/readyz` | A base de dados responde em menos de 2 s | Smoke tests do deploy |

Se o `/healthz` dependesse da base de dados, uma falha na Neon faria a Render reiniciar a API em ciclo, sem resolver nada.

## Regras de dependência

- Um módulo só importa de outro módulo a **interface do seu serviço**, nunca o repositório nem os tipos internos.
- `domain.go` não importa `net/http`, `database/sql`, `pgx` nem outro módulo.
- Estas regras são verificadas no CI com `depguard` (golangci-lint).

## Convenções

- **Erros de domínio tipados** (`ErrInsufficientStock`, `ErrForbidden`…), traduzidos para HTTP só no adaptador.
- **Contexto em todas as chamadas**, com timeout por pedido e cancelamento propagado até à BD.
- **Idempotência** em `POST /sales` através da `Idempotency-Key`, guardada 24 h.
- **Concorrência no stock:** `SELECT … FOR UPDATE` nos lotes, ordenados por validade, dentro da transação.
- **Configuração por variáveis de ambiente**, validada no arranque: a API recusa arrancar se faltar um segredo.
- **Logs** com `log/slog` em JSON, sempre com `request_id`; nunca palavras-passe, tokens nem cookies.

## Stack

| Área | Escolha |
|---|---|
| Linguagem | Go 1.26 |
| HTTP | `net/http` (router nativo) |
| Contrato | OpenAPI 3.1 + `oapi-codegen` |
| Base de dados | `pgx` v5 + `sqlc` |
| Migrações | `goose` (SQL puro) |
| Logs | `log/slog` |
| Testes | `go test`, `testcontainers-go` |
| Lint | `golangci-lint` (incl. `gosec`, `depguard`) |
