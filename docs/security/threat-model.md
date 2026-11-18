# Modelo de ameaças (STRIDE)

## Ativos

| Ativo | Porque importa |
|---|---|
| Credenciais e sessões dos utilizadores | Dão acesso ao backoffice |
| Registos de venda e stock | Base dos relatórios e da gestão da loja |
| Preços e margens | Informação comercial |
| Registo de auditoria | Prova de quem fez o quê |
| Segredos (chaves JWT, credenciais da BD, chave do Resend) | Comprometem tudo o resto |

## Fronteiras de confiança

1. Internet → CDN da Vercel
2. Vercel → API na Render (proxy)
3. API → PostgreSQL na Neon (TLS)
4. API → Resend (HTTPS)
5. GitHub Actions → Render, Vercel e Neon (segredos de deploy)

## Ameaças e mitigações

| Ameaça | Cenário concreto | Mitigação | Verificação |
|---|---|---|---|
| **Spoofing** | Força bruta ou credential stuffing no login. | Rate limit por IP e por conta, argon2id e mensagem genérica (“credenciais inválidas”). | Teste de integração do rate limit |
| **Spoofing** | Refresh token roubado e reutilizado. | Rotação com deteção de reutilização; revogação da família. | Teste de integração |
| **Tampering** | Pedido de venda alterado com um preço mais baixo. | O servidor ignora preços enviados pelo cliente e recalcula a partir do catálogo. | Teste unitário do serviço de vendas |
| **Tampering** | SQL injection num filtro de relatório. | Só queries parametrizadas (sqlc); sem SQL dinâmico concatenado. | gosec + revisão de PR |
| **Repudiation** | Funcionário nega ter feito uma quebra ou uma venda. | Auditoria só de acrescento, com autor, IP e hora; o papel `app` não pode alterá-la. | Teste de permissões na BD |
| **Information disclosure** | O catálogo público revela quantidades ou margens. | DTO público separado, que devolve só o nível de disponibilidade. | Teste de contrato |
| **Information disclosure** | Erros expõem stack traces ou SQL. | Erros `problem+json` genéricos; detalhes só nos logs. | Teste de integração |
| **Information disclosure** | Segredo commitado no repositório. | gitleaks no CI; `.env` no `.gitignore`. | Workflow `security.yml` |
| **Denial of service** | Tráfego automatizado no catálogo esgota a instância gratuita. | Rate limit e cache HTTP de 60 s do catálogo na CDN da Vercel. | Teste k6 |
| **Elevation of privilege** | Um funcionário chama um endpoint de gestor. | RBAC com tudo negado por omissão, verificação dupla e [matriz](authz-matrix.md) testada. | Teste por cada linha da matriz |
| **Elevation of privilege** | Dependência comprometida no build. | Dependabot, lockfiles, Actions fixadas por SHA, `permissions` mínimas. | Workflow `security.yml` |

## Revisão

Este modelo é revisto sempre que se acrescenta uma fronteira de confiança (por exemplo, pagamentos ou contas de cliente na v2) e antes de cada release para a `main`.
