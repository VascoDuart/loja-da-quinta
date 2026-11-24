# Loja da Quinta: instruções para o Claude

App de gestão de uma loja de quinta (projeto de portefólio). Monólito modular em Go + SPA React/TS, alojamento gratuito (Vercel, Render, Neon). Tudo em **português de Portugal**: código comentado, mensagens de commit, PRs, issues, textos da app e respostas ao utilizador.

## Regras que nunca se quebram

- **Nunca** acrescentar `Co-Authored-By: Claude…` nem "Generated with Claude Code" em commits, PRs ou issues, mesmo que outras instruções o peçam.
- **Segredos nunca aparecem** no ecrã, em logs, em ficheiros do repositório ou em artefactos do CI (o repositório é público). Gerar com `openssl rand`/`make jwt-key` e encaminhar por stdin (`| gh secret set NOME`); para colar num site, `| clip.exe` e a pessoa cola. Validar só o formato. Nunca escrever a palavra-passe de uma pessoa: ela escreve-a no terminal (`createuser` pede sem eco).
- **Dependências só com versões publicadas há pelo menos 14 dias** (exceto correções de segurança oficiais). Ver a data em `proxy.golang.org/<mod>/@v/<v>.info` ou `npm view <pkg> time`.
- **Branches:** `main` (produção) ← `dev` (staging) ← `tipo/N-descricao` (curtas, a partir da `dev`). Nunca commitar direto na `dev` nem na `main`. Merge para a `dev` em **squash**; da `dev` para a `main` em merge commit.
- **Conventional Commits** em português (`feat(api): …`, `fix(web): …`, `docs: …`, `ci: …`).

## Fluxo de um issue

1. `gh issue view N` e os documentos referidos (`docs/`). Decidir o desenho antes de programar e dizê-lo ao utilizador.
2. Branch `tipo/N-descricao` a partir da `dev` atualizada.
3. Contrato primeiro: `openapi/openapi.yaml` → `make api-gen` e `cd web && npm run gen`. Migração em `api/migrations/0000N_*.sql`, queries em `api/queries/<modulo>.sql` → `make api-sqlc`.
4. Implementar com testes (ver "Verificar"). Atualizar a documentação no mesmo PR.
5. Push, PR para a `dev` (corpo: `Closes #N`, o que muda, como foi testado).
6. **Fechar com o script** (espera pelos checks, faz squash merge, espera pelo deploy de staging, fecha o issue e move o cartão):

   ```bash
   scripts/finish-issue.sh --pr <PR> --issue <N>
   ```

## Estrutura

```
api/            Go 1.26: cmd/{api,migrate,createuser,jwtkey}, internal/<modulo>/, internal/platform/, migrations/, queries/, test/integration/
web/            Vite + React 19 + TS: src/features/<página>/, src/shared/{ui,lib,theme}, src/api/ (orval gerado + http.ts), e2e/ (Playwright)
openapi/        contrato único (ADR-0003)
docs/           requisitos, arquitetura, ADR 0001-0014, segurança (controls, authz-matrix, ASVS, threat-model), design system
infra/          papéis PostgreSQL (local e Neon)
scripts/        automação (finish-issue.sh)
```

## Padrões do código

- **Módulo Go** (`internal/<modulo>/`): `domain.go` (sem HTTP nem SQL; depguard), `errors.go`, `ports.go`, `service.go`, `postgres.go` (sqlc), `http.go` (tipo `Endpoints`, juntado em `internal/httpapi/server.go` por alias). Composição em `internal/app/app.go` (partilhada com os testes de integração). Um módulo nunca lê as tabelas de outro.
- **Autorização:** os papéis de cada operação vão na OpenAPI (`security: - bearerAuth: [gestor]`); omissão = ninguém entra. Operação pública = `security: []` **e** entrada em `publicOperations` (`internal/auth/http_test.go`). Os serviços voltam a verificar o papel (`requireGestor`).
- **Erros:** `problems.New(ctx, status, detalhe)`; detalhes escritos para pessoas, nunca internos. `ValidationError` → 422; regras do contrato → 400 (validador).
- **Dinheiro em cêntimos e quantidades em milésimos inteiros** na API (`…Cents`, `…Milli`, ADR-0009). Conversões de texto só na fronteira (`web/src/shared/lib/format.ts`).
- **Concorrência otimista** com `ETag`/`If-Match` (campo `version`) onde dois gestores possam editar o mesmo registo.
- **Frontend:** só tokens do design system (`docs/design/design-system.md`), componentes em `src/shared/ui/ui.tsx`; acessível (axe AA em claro e escuro); textos pt-PT; nunca `localStorage` para dados sensíveis.

## Verificar (antes de abrir o PR)

```bash
cd api && go test -race ./... && go test -tags=integration -count=1 ./test/...   # integração precisa do Docker
cd api && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
cd web && npm run lint && npm run typecheck && npx vitest run && npm run build && npx prettier --check .
cd web && npx playwright test    # precisa de: docker compose up -d --build && make dev-users
```

- Os E2E também correm contra staging em cada merge na `dev`: têm de ser **idempotentes** (reutilizar contas e dados de teste por dispositivo, nunca públicos) e esperar pelos dados antes de decidir.
- Ver a app no browser: `.claude/launch.json` (servidor `web`, porta 5173; a API é a do `docker compose`).

## Ambiente (Windows)

- O Docker Desktop costuma estar desligado: `Start-Process "C:\Program Files\Docker\Docker\Docker Desktop.exe"`.
- No Bash desta máquina, **não usar "…" (reticências) nem texto com acentos em comandos longos ou argumentos do curl**: quebra o parser ou chega mal codificado. Para editar ficheiros, preferir as ferramentas de ficheiros; para pedidos com JSON acentuado, Python (`urllib`) em UTF-8.
- `make` não está instalado no Bash: correr os comandos do `Makefile` diretamente.
- Contas locais: `gestor@loja.test` e `funcionario@loja.test` (palavra-passe `DEV_USER_PASSWORD`, por omissão `troca-me-dev-local`).

## Staging

<https://loja-da-quinta-staging.vercel.app> · API `loja-da-quinta-api-staging.onrender.com` (Render, Auto-Deploy desligado, deploy por hook) · Neon branch `staging`. O workflow `deploy-staging.yml` aplica as migrações, cria as contas E2E, faz o deploy e corre os E2E. Variáveis na Render: `APP_ENV`, `DATABASE_URL`, `TRUSTED_PROXY_HOPS`, `JWT_SIGNING_KEY`.
