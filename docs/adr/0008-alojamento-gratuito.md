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

- A API adormece após 15 minutos sem pedidos; o primeiro pedido pode demorar 30 a 60 segundos (RNF-04).
- Staging usa um segundo serviço gratuito na Render e um branch da Neon. Como os serviços adormecem, deve caber nas 750 horas mensais; a confirmar no primeiro mês.
- As condições dos planos gratuitos mudam: reavaliar este ADR a cada 6 meses.
- A aplicação não depende de SDKs proprietários; mudar de fornecedor é mudar a configuração de deploy.
