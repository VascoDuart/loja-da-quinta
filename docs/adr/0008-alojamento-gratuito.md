# 0008. Alojamento gratuito: Vercel, Render, Neon e Resend

- **Estado:** Aceite
- **Data:** 2026-11-18

## Contexto

O orçamento é 0 € por mês. A API em Go precisa de um processo sempre disponível, o que a Vercel não oferece.

## Decisão

| Componente | Serviço | Plano |
|---|---|---|
| Frontend | Vercel | Hobby |
| API Go (Docker) | Render | Free web service |
| PostgreSQL | Neon | Free (com branches para staging) |
| E-mail | Resend | Free |
| CI/CD | GitHub Actions | Gratuito em repositório público |

## Consequências

- A API adormece após 15 minutos sem pedidos. A documentação da Render indica cerca de 1 minuto para acordar; o spike #3 mediu 12,5 s (RNF-04, [ADR-0007](0007-mesma-origem-via-rewrite.md)).
- **O plano Hobby da Vercel é só para uso pessoal e não comercial.** Serve enquanto a app for um projeto de portefólio. Se a quinta a usar para vender, é preciso mudar o frontend para um serviço que permita uso comercial (por exemplo, Cloudflare Pages) ou passar a um plano pago.
- Se as 750 horas mensais da Render acabarem, todos os serviços gratuitos da conta ficam suspensos até ao mês seguinte.
- Alternativa avaliada para a API: Google Cloud Run (arranque a frio abaixo de 1 s para Go, mas exige registar um cartão). Fica como plano B.
- Staging usa um segundo serviço gratuito na Render e um branch da Neon. Como os serviços adormecem, deve caber nas 750 horas mensais; a confirmar no primeiro mês.
- As condições dos planos gratuitos mudam: reavaliar este ADR a cada 6 meses.
- A aplicação não depende de SDKs proprietários; mudar de fornecedor é mudar a configuração de deploy.
