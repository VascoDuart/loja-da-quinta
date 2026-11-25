# Modelo de ameaças (STRIDE)

## Ativos

| Ativo | Porque importa |
|---|---|
| Credenciais e sessões dos utilizadores | Dão acesso ao backoffice |
| Registos de venda e stock | Base dos relatórios e da gestão da loja |
| Preços e margens | Informação comercial |
| Registo de auditoria | Prova de quem fez o quê |
| Segredos (chaves JWT, credenciais da BD, chave do Resend, token dos jobs) | Comprometem tudo o resto |

## Fronteiras de confiança

1. Internet → CDN da Vercel
2. Vercel → API na Render (proxy)
3. API → PostgreSQL na Neon (TLS)
4. API → Resend (HTTPS)
5. GitHub Actions → Render, Vercel e Neon (segredos de deploy)
6. GitHub Actions → endpoint dos jobs da API (token dos jobs)

## Ameaças e mitigações

| Ameaça | Cenário concreto | Mitigação | Verificação |
|---|---|---|---|
| **Spoofing** | Força bruta ou credential stuffing no login. | Rate limit por IP e por conta, argon2id e mensagem genérica (“credenciais inválidas”). | Teste de integração do rate limit |
| **Spoofing** | Refresh token roubado e reutilizado. | Rotação com deteção de reutilização; revogação da família. | Teste de integração |
| **Spoofing** | Alguém chama o job dos alertas para o fazer enviar, ou adivinha o token. | Esquema próprio com um token de pelo menos 32 caracteres, comparado em tempo constante; sem ele ninguém entra; uma sessão não serve; rate limit por IP. O job só envia a partir das 07:00, uma vez por dia e a destinatários fixados no servidor: o pedido não leva dados ([ADR-0015](../adr/0015-alertas-diarios-por-agendamento-externo.md)). | Testes `TestAuthorizeJobToken`, `TestAuthorizationMatrix` e `TestEveryOperationDeclaresItsAccess` |
| **Denial of service** | Dois disparos ao mesmo tempo, ou repetidos, enviam o alerta várias vezes. | O dia é a chave primária de `stock_alert_runs` e é marcado antes do envio. | Teste de integração `TestStockAlerts` (simultâneos) |
| **Information disclosure** | O token dos jobs, a chave do Resend ou os endereços aparecem nos logs (os do CI são públicos). | O token vai por stdin, nunca na linha de comandos; o script mostra só estado, dia e entrega; os erros do Resend dizem só o código e o tipo; o log sem Resend não leva endereços; redirecionamentos do Resend não são seguidos. | Testes `TestResendNotifierErrorsRevealNeitherKeyNorAddresses` e `TestLogNotifierWritesTheEmailWithoutAddresses` |
| **Tampering** | Pedido de venda alterado com um preço mais baixo. | O servidor ignora preços enviados pelo cliente e recalcula a partir do catálogo. | Teste unitário do serviço de vendas |
| **Tampering** | SQL injection num filtro de relatório. | Só queries parametrizadas (sqlc); sem SQL dinâmico concatenado. | gosec + revisão de PR |
| **Tampering** | Alguém altera ou apaga o histórico de stock para esconder uma diferença. | `stock_movements` só de acrescento: trigger que recusa `UPDATE`, `DELETE` e `TRUNCATE` a toda a gente e papel `app` só com `INSERT` e `SELECT`. `qty_left >= 0` e o sinal de cada tipo de movimento em `CHECK`. O autor de cada movimento vem da sessão. | Testes de integração `TestStockLots` e `TestStockMovementsPrivileges` |
| **Repudiation** | Funcionário nega ter feito uma quebra ou uma venda. | Auditoria só de acrescento, com autor, IP e hora; o papel `app` não pode alterá-la. | Teste de permissões na BD |
| **Tampering** | Upload de um ficheiro malicioso disfarçado de imagem (polyglot, SVG com script, bomba de descompressão). | Tipo pelo conteúdo, limite de píxeis, reprocessamento para JPEG, original descartado, `nosniff` ([ADR-0014](../adr/0014-imagens-dos-produtos-no-postgresql.md)). | Testes com ficheiros maliciosos |
| **Information disclosure** | Fotografia carregada com a localização GPS da quinta nos metadados. | O reprocessamento descarta todos os metadados EXIF. | Teste que confirma a ausência de EXIF |
| **Information disclosure** | O catálogo público revela quantidades ou margens. | DTO público separado (`CatalogItem`), que devolve só o nível de disponibilidade (`disponivel`, `pouco`, `esgotado`), nunca a quantidade, o IVA, o limiar de stock, o estado "ativo" nem datas. A query só lê produtos ativos e públicos; o limiar de stock sai da base de dados só para decidir a disponibilidade e fica no módulo `catalog`. | Teste de integração `TestPublicCatalog` (lista fechada de campos e texto cru da resposta) |
| **Information disclosure** | Erros expõem stack traces ou SQL. | Erros `problem+json` genéricos; detalhes só nos logs. | Teste de integração |
| **Information disclosure** | Segredo commitado no repositório. | gitleaks no CI; `.env` no `.gitignore`. | Workflow `security.yml` |
| **Denial of service** | Tráfego automatizado no catálogo esgota a instância gratuita. | Rate limit e cache HTTP de 60 s do catálogo (`Cache-Control: public, max-age=60`, enviado pela API; a imagem é imutável). | Teste k6 |
| **Elevation of privilege** | Um funcionário chama um endpoint de gestor. | RBAC com tudo negado por omissão, verificação dupla e [matriz](authz-matrix.md) testada. | Teste por cada linha da matriz |
| **Elevation of privilege** | Dependência comprometida no build. | Dependabot, lockfiles, Actions fixadas por SHA, `permissions` mínimas. | Workflow `security.yml` |

## Revisão

Este modelo é revisto sempre que se acrescenta uma fronteira de confiança (por exemplo, pagamentos ou contas de cliente na v2) e antes de cada release para a `main`.
