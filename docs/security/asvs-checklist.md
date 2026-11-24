# Checklist OWASP ASVS 4.0 · nível 1

Subconjunto aplicável ao MVP. Atualizar o estado em cada release para a `main` (RNF-01).

Estado: ⬜ por fazer · 🟡 em curso · ✅ feito · ➖ não aplicável

| Capítulo | Requisito (resumo) | Onde | Estado |
|---|---|---|---|
| V2 Autenticação | Palavras-passe com pelo menos 12 caracteres, sem regras de composição | `users` | ✅ |
| V2 Autenticação | Palavras-passe verificadas contra listas de palavras-passe comprometidas | `auth` | ⬜ |
| V2 Autenticação | Proteção contra força bruta (rate limit, bloqueio progressivo) | middleware + `auth` | ✅ |
| V2 Autenticação | Hash com algoritmo adaptativo (argon2id) | `platform/password` | ✅ |
| V3 Sessões | Tokens de sessão gerados com CSPRNG e com pelo menos 128 bits | `auth` (256 bits) | ✅ |
| V3 Sessões | Logout invalida a sessão no servidor | `auth` | ✅ |
| V3 Sessões | Cookies com `Secure`, `HttpOnly` e `SameSite` | `auth` | ✅ |
| V3 Sessões | Desativar um utilizador termina as suas sessões | `users` + `auth` | ✅ (também ao mudar o papel ou a palavra-passe) |
| V4 Controlo de acesso | Negado por omissão; verificação no servidor em cada pedido | OpenAPI (`security`) + `auth.Authorize` | ✅ |
| V4 Controlo de acesso | Proteção contra IDOR (verificação de posse/papel no serviço) | serviços | ⬜ |
| V4 Controlo de acesso | Proteção CSRF nos endpoints com cookie | `auth` (`SameSite=Strict` + `X-CSRF`) | ✅ |
| V5 Validação | Validação de entrada contra esquema (OpenAPI) | adaptador HTTP | ⬜ |
| V5 Validação | Queries parametrizadas em todo o acesso à BD | `postgres.go` (sqlc) | ⬜ |
| V5 Validação | Saída codificada no contexto (React + CSP) | `web/` | ⬜ |
| V7 Erros e logs | Erros sem detalhes internos para o cliente | adaptador HTTP | ⬜ |
| V7 Erros e logs | Logs sem credenciais, tokens nem dados pessoais desnecessários | `platform/logging` | ⬜ |
| V8 Dados | Dados sensíveis fora de URLs e de `localStorage` | `web/` (testado no E2E) | ✅ |
| V9 Comunicações | TLS em todas as ligações (browser, proxy, BD, e-mail) | infraestrutura | ⬜ |
| V10 Código malicioso | Dependências analisadas no CI | `security.yml` (govulncheck, Trivy, CodeQL) | ✅ |
| V12 Ficheiros | Tamanho máximo do upload (5 MB) e de píxeis (40 MP) | `catalog` (píxeis lidos antes de descodificar) | ✅ |
| V12 Ficheiros | Tipo verificado pelo conteúdo, nunca pela extensão nem pelo `Content-Type` | `catalog` | ✅ |
| V12 Ficheiros | Ficheiro reprocessado; o original não é guardado nem servido | `catalog` (EXIF e GPS removidos, testado) | ✅ |
| V12 Ficheiros | Ficheiros servidos com `Content-Type` fixo e `nosniff` | `catalog` + middleware | ✅ |
| V13 API | Limites de tamanho e de taxa nos pedidos | middleware (1 MiB; 5 MB só no upload de imagens) | ✅ |
| V14 Configuração | Cabeçalhos de segurança (CSP, HSTS, nosniff, frame-ancestors) | `vercel.json` + middleware | ⬜ |
| V14 Configuração | Segredos fora do código e do repositório | CI + `.gitignore` | 🟡 |
