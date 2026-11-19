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

## Resultados (2026-11-18/19)

Ambiente: Render Free em Frankfurt, Vercel Hobby (`cdg1`, Paris), pedidos feitos de Portugal com `curl`.

| Teste | Resultado |
|---|---|
| Rewrite `/api/*` com a API acordada | `200` em 0,2–0,6 s |
| Arranque a frio após 17 min sem tráfego, através da Vercel | `200` em **12,5 s**; `startedAt` confirma um processo novo |
| Pedido seguinte | `200` em 0,2 s |
| Pedidos lentos de 10, 30, 60 e 90 s | Todos `200` |
| Pedido lento de 120 s | `502 ROUTER_EXTERNAL_TARGET_ERROR` aos 120 s |
| Cookie `HttpOnly; Secure; SameSite=Strict` | Definido e recebido de volta através do rewrite |

## Conclusões

1. O rewrite da Vercel serve para pôr a API na mesma origem. O ADR-0007 fica aceite.
2. O tempo limite do proxy está entre 91 e 120 s, muito acima do arranque a frio medido (12,5 s). Há folga mesmo que o arranque chegue ao minuto que a documentação da Render indica.
3. Só houve uma medição de arranque a frio. Os timeouts do cliente devem assumir o pior caso documentado (~60 s), não os 12,5 s.
4. Decisões para a implementação:
   - O cliente mostra “a acordar o servidor” após 3 s e espera até 75 s antes de dar erro.
   - Nenhum endpoint da API pode demorar mais de 60 s; o timeout de escrita do servidor fica nos 30 s.
   - O cookie de refresh com `SameSite=Strict` funciona como previsto no ADR-0005.
