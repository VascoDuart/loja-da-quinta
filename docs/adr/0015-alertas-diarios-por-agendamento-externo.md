# 0015. Alertas diários disparados pelo GitHub Actions

- **Estado:** Aceite
- **Data:** 2026-11-25

## Contexto

O gestor deve receber todos os dias, às 07:00 de Lisboa, um e-mail com o stock baixo e os lotes a expirar (RF-12). A arquitetura previa uma goroutine agendada dentro da API. No plano gratuito da Render ([ADR-0008](0008-alojamento-gratuito.md)) a API adormece ao fim de 15 minutos sem pedidos: às 07:00 a loja está fechada, o processo não está a correr e a goroutine nunca dispararia.

## Opções consideradas

1. **Goroutine agendada no processo.** Simples, mas não corre com a API a dormir.
2. **Manter a API acordada com pedidos periódicos.** Gasta as 750 horas mensais e contorna o plano gratuito.
3. **Cron job da Render.** Pago.
4. **Workflow agendado do GitHub Actions que chama um endpoint da API.** Gratuito no repositório público; o próprio pedido acorda a API.

## Decisão

Opção 4.

- **Disparo:** `.github/workflows/stock-alerts.yml`, com `cron` às 06:00 e às 07:00 UTC (07:00 em Lisboa no verão e no inverno) e `workflow_dispatch`. Corre `scripts/stock-alerts.sh`.
- **Endpoint:** `POST /api/v1/jobs/stock-alerts`, com um esquema de segurança próprio na OpenAPI (`jobToken`, cabeçalho `X-Job-Token`). Não aceita sessões de utilizadores, e o token não abre mais nada. A comparação é em tempo constante (sobre o SHA-256 dos dois valores). Continua tudo negado por omissão: cada operação que usa o `jobToken` tem de estar na lista `jobOperations` do teste `TestEveryOperationDeclaresItsAccess`. Sem `JOBS_TOKEN` (pelo menos 32 caracteres), a API recusa arrancar fora do ambiente local; localmente, o endpoint recusa todos os pedidos.
- **Idempotência por dia de Lisboa:** a API só envia a partir das 07:00 de Lisboa e no máximo uma vez por dia. O dia fica na tabela `stock_alert_runs`, com o dia como chave primária. É marcado **antes** do envio: entre dois disparos ao mesmo tempo, só um consegue inserir a linha, e o outro responde `ja_enviado`. Se o envio falhar, a linha é apagada e a API responde `502`.
- **Sem nada a avisar**, não envia nem marca o dia.
- **Envio:** o `inventory` declara a porta `Notifier`. Há um adaptador do Resend que usa a API HTTP com a biblioteca padrão (sem SDK) e um que escreve o e-mail no log, usado quando não há `RESEND_API_KEY`. Os testes usam um fake.
- **Destinatários:** os gestores ativos, pela porta `Recipients` ligada ao módulo `users`. A variável opcional `ALERTS_TO` substitui-os.

| Variável da API | Para quê | Obrigatória |
|---|---|---|
| `JOBS_TOKEN` | abre o endpoint do job | fora do ambiente local |
| `RESEND_API_KEY` | envia pelo Resend; sem ela, vai para o log | não |
| `ALERTS_TO` | destinatários, separados por vírgulas, em vez dos gestores | não |
| `ALERTS_FROM` | remetente; por omissão `onboarding@resend.dev` | não |

No GitHub, o segredo `STAGING_JOBS_TOKEN` tem o mesmo valor que o `JOBS_TOKEN` do staging.

## Consequências

- **Os workflows agendados, e o `workflow_dispatch`, só funcionam com o ficheiro no branch por omissão (`main`).** O envio automático começa com a release (#19). Até lá, corre-se `scripts/stock-alerts.sh` à mão. O workflow aponta para o staging; a release tem de decidir o ambiente de produção e o respetivo segredo.
- O GitHub não garante a hora exata de um agendamento: pode atrasar-se. A API envia a qualquer hora depois das 07:00, por isso um atraso só atrasa o e-mail. O GitHub também desliga os agendamentos de um repositório público ao fim de 60 dias sem atividade.
- Um alerta pode perder-se se o processo morrer entre marcar o dia e enviar: prefere-se isso a enviar duas vezes. Se o Resend aceitar o e-mail mas a resposta não chegar, a tentativa seguinte pode repeti-lo. Para reenviar à mão, apaga-se a linha do dia em `stock_alert_runs`.
- No plano gratuito do Resend, sem domínio próprio, só se entrega no e-mail do dono da conta. Os gestores do staging são `@loja.test`, por isso é preciso o `ALERTS_TO`. Um só destinatário recusado faz falhar o envio todo.
- Os logs do workflow são públicos: o script mostra só o estado, o dia e a forma de entrega. Sem Resend, o e-mail (produtos e quantidades) fica nos logs da API, que são privados, sem os endereços.
- Há um segredo novo partilhado entre o GitHub e a Render. Para o trocar, muda-se nos dois sítios.
- Substitui a ideia da goroutine agendada da arquitetura. Se o alojamento passar a um processo sempre ligado, o agendamento pode voltar para dentro, mantendo a idempotência.
