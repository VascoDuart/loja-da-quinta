# 0007. Mesma origem através do rewrite da Vercel

- **Estado:** Aceite (validado pelo spike #3 em 2026-11-19)
- **Data:** 2026-11-18

## Contexto

A SPA fica em `*.vercel.app` e a API em `*.onrender.com`: são sites diferentes. Cookies `SameSite=Strict` não seriam enviados e seria preciso CORS com credenciais.

## Opções consideradas

1. CORS com credenciais e cookies `SameSite=None`: mais superfície para CSRF.
2. Domínio próprio partilhado (`app.` e `api.`): tem custo.
3. Rewrite na Vercel: `/api/*` é encaminhado para a Render, e o browser só vê uma origem.

## Decisão

Rewrite em `web/vercel.json`.

## Validação (spike #3)

Medições com a API no plano gratuito da Render (Frankfurt) e a página na Vercel Hobby. Detalhes no branch `spike/3-rewrite-arranque-frio`, em `spike/README.md`.

| Teste | Resultado |
|---|---|
| Rewrite com a API acordada | `200` em 0,2–0,6 s |
| Arranque a frio após 17 min sem tráfego | `200` em 12,5 s |
| Tempo limite do proxy da Vercel | Entre 91 e 120 s (`502` aos 120 s) |
| Cookie `HttpOnly; Secure; SameSite=Strict` | Funciona através do rewrite |

## Consequências

- Sem CORS cross-site; o cookie de refresh pode ser `SameSite=Strict`.
- A CDN pode guardar em cache as respostas públicas do catálogo.
- O proxy espera bem mais do que o arranque a frio. Mesmo no pior caso documentado pela Render (~60 s), há folga.
- Regras que daqui resultam:
  - O cliente mostra “a acordar o servidor” após 3 s e só dá erro aos 75 s.
  - Nenhum endpoint pode demorar mais de 60 s; o timeout de escrita do servidor é de 30 s.
- Só houve uma medição de arranque a frio. Se em staging aparecerem 502 no primeiro pedido, reabrir esta decisão (alternativas: domínio próprio ou Cloud Run).
