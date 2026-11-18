# Spike #3 · rewrite da Vercel contra o arranque a frio da Render

Código descartável para validar o [ADR-0007](../docs/adr/0007-mesma-origem-via-rewrite.md). Este branch nunca é integrado; só a conclusão entra na `dev`, no ADR.

## Perguntas

1. Quanto tempo demora a API a acordar na Render (plano gratuito)?
2. O rewrite `/api/*` da Vercel espera esse tempo, ou devolve erro antes?
3. Qual é o tempo limite do proxy da Vercel?
4. Um cookie `SameSite=Strict` funciona através do rewrite?

## Peças

| Pasta | Onde corre | O que faz |
|---|---|---|
| `api/` | Render (Docker, free) | `/api/v1/healthz` com uptime, `/api/v1/slow?ms=`, `/api/v1/cookie` |
| `web/` | Vercel (Hobby) | Página com botões e `vercel.json` com o rewrite |

## Resultados

_Por preencher._
