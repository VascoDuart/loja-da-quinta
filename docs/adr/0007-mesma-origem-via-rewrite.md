# 0007. Mesma origem através do rewrite da Vercel

- **Estado:** Aceite (a validar com um spike)
- **Data:** 2026-11-18

## Contexto

A SPA fica em `*.vercel.app` e a API em `*.onrender.com`: são sites diferentes. Cookies `SameSite=Strict` não seriam enviados e seria preciso CORS com credenciais.

## Opções consideradas

1. CORS com credenciais e cookies `SameSite=None`: mais superfície para CSRF.
2. Domínio próprio partilhado (`app.` e `api.`): tem custo.
3. Rewrite na Vercel: `/api/*` é encaminhado para a Render, e o browser só vê uma origem.

## Decisão

Rewrite em `web/vercel.json`.

## Consequências

- Sem CORS cross-site; o cookie de refresh pode ser `SameSite=Strict`.
- A CDN pode guardar em cache as respostas públicas do catálogo.
- **Risco:** o proxy da Vercel tem um tempo limite. Se o arranque a frio da Render o ultrapassar, o primeiro pedido falha. Mitigação: o cliente repete o pedido e mostra “a acordar o servidor”. Se não chegar, a alternativa é a opção 2.
- **Ação:** validar com um spike antes de implementar a autenticação.
