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

## Testes: o que escrever

Cada comportamento é testado **numa só camada**, a mais barata que o prova. Repetir o mesmo caso em várias camadas gasta tempo e tokens sem apanhar mais erros.

| Camada | Escrever só para | Não escrever |
|---|---|---|
| Unitários Go (`*_test.go`, fakes) | Regras do domínio e do serviço: validações, cálculos, papéis no serviço | O que o SQL ou o HTTP já provam |
| Integração Go (`test/integration`, PostgreSQL real) | SQL, transações, concorrência (`FOR UPDATE`), restrições da base de dados, `ETag`/`If-Match`, um caminho feliz por endpoint | 401/403 por endpoint (já gerados), validações que o contrato recusa (400) |
| Vitest | Lógica de ecrã não trivial: conversões, estados de erro, o que é enviado à API | Que um texto aparece, cada ramo de cada formulário |
| E2E (Playwright) | O fluxo principal de cada ecrã novo, ponta a ponta, e o axe da página em claro e escuro | Variações e erros (ficam nas camadas de baixo) |

- **Autorização gerada:** `test/integration/authz_spec_test.go` lê a OpenAPI e prova, para **cada** operação, que sem sessão dá 401 e que cada papel fora da lista do `bearerAuth` dá 403. Um endpoint novo fica coberto sem escrever nada; nos testes do issue só entra o caso positivo.
- **Um teste a pensar num risco concreto vale sempre a pena** (corrida, segurança, perda de dados, dinheiro). Foram esses que apanharam os bugs reais deste projeto.
- Os E2E correm contra staging em cada merge na `dev`: têm de ser **idempotentes** (reutilizar contas e dados de teste por dispositivo, nunca públicos, sem acumular) e esperar pelos dados antes de decidir.

## Testes: o que correr

Localmente, só o que a alteração toca. A suíte completa corre no CI (PR) e no staging (merge): ler os resultados de lá só quando falham.

```bash
cd api && go test ./internal/<modulo>/...                                     # unitários do módulo
cd api && go test -tags=integration -count=1 -run 'TestX' ./test/...           # só os testes novos (Docker)
cd api && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
cd web && npm run lint && npm run typecheck && npx vitest run --maxWorkers=4 src/features/<pagina>
cd web && npx playwright test e2e/<ficheiro>.spec.ts --project=desktop        # só o E2E novo, só desktop
```

- O PR só abre com estes a verde; o CI corre o resto (todas as camadas e os 3 dispositivos) e o `finish-issue.sh` mostra o resultado do staging.
- O E2E local precisa de `docker compose up -d --build` e `make dev-users`. Com mais de 4 workers, o Vitest dá timeouts falsos nesta máquina.
- Ver a app no browser: `.claude/launch.json` (servidor `web`, porta 5173; a API é a do `docker compose`).

## Ambiente (Windows)

- O Docker Desktop costuma estar desligado: `Start-Process "C:\Program Files\Docker\Docker\Docker Desktop.exe"`.
- No Bash desta máquina, **não usar "…" (reticências) nem texto com acentos em comandos longos ou argumentos do curl**: quebra o parser ou chega mal codificado. Para editar ficheiros, preferir as ferramentas de ficheiros; para pedidos com JSON acentuado, Python (`urllib`) em UTF-8.
- `make` não está instalado no Bash: correr os comandos do `Makefile` diretamente.
- Contas locais: `gestor@loja.test` e `funcionario@loja.test` (palavra-passe `DEV_USER_PASSWORD`, por omissão `troca-me-dev-local`).

## Staging

<https://loja-da-quinta-staging.vercel.app> · API `loja-da-quinta-api-staging.onrender.com` (Render, Auto-Deploy desligado, deploy por hook) · Neon branch `staging`. O workflow `deploy-staging.yml` aplica as migrações, cria as contas E2E, faz o deploy e corre os E2E. Variáveis na Render: `APP_ENV`, `DATABASE_URL`, `TRUSTED_PROXY_HOPS`, `JWT_SIGNING_KEY`.
