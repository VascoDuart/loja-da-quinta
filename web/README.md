# web

Frontend em React + TypeScript, construído com Vite. Ver [docs/architecture/frontend.md](../docs/architecture/frontend.md).

> **Estado:** rotas, layouts e design system "Horta Fresca" com modo claro e escuro ([ADR-0013](../docs/adr/0013-design-system-horta-fresca.md), [tokens e componentes](../docs/design/design-system.md)). Cliente da API gerado da OpenAPI, com TanStack Query. As páginas de negócio ainda são provisórias.

## Correr

```bash
make web-install
make up        # API e base de dados (opcional, para /api responder)
make web-dev   # http://localhost:5173
```

Em desenvolvimento, o Vite encaminha `/api` para `http://localhost:8080`, tal como o rewrite da Vercel faz em produção (ADR-0007).

## Comandos

| Comando | O que faz |
|---|---|
| `make web-dev` | Servidor de desenvolvimento com recarregamento automático |
| `make web-test` | Testes com Vitest e Testing Library |
| `make web-lint` | ESLint e verificação de tipos (TypeScript strict) |
| `make web-build` | Build de produção em `web/dist` |
| `npm run gen` | Regenera o cliente da API a partir de `openapi/openapi.yaml` (orval) |
| `npm run format` | Formata o código com Prettier (o CI verifica com `format:check`) |

## Rotas

| Caminho | Página | Quem |
|---|---|---|
| `/` | Catálogo de produtos da época | Público |
| `/produtos/:id` | Detalhe de um produto | Público |
| `/entrar` | Início de sessão | Público |
| `/app` | Redireciona para `/app/painel` | — |
| `/app/painel` | Resumo do dia | Funcionário, gestor |
| `/app/venda` | Venda ao balcão | Funcionário, gestor |
| `/app/stock` | Stock e lotes | Funcionário, gestor |
| `/app/produtos` | Produtos e categorias | Gestor |
| `/app/relatorios` | Relatórios | Gestor |
| `/app/utilizadores` | Utilizadores | Gestor |
| `/app/auditoria` | Auditoria | Gestor |
| qualquer outro | Página não encontrada | — |

As rotas estão em [`src/app/routes.tsx`](src/app/routes.tsx), com o título, os papéis e se aparecem no menu. Cada página é carregada só quando é visitada. A guarda que aplica os papéis chega com a autenticação (#8); a autorização real é sempre da API.

## Falar com a API

1. O contrato está em `openapi/openapi.yaml`. Depois de o mudar, corre `npm run gen`.
2. O orval gera em `src/api/generated/` (não se edita à mão):
   - `endpoints/`: funções e hooks do TanStack Query (`useGetReadiness()`…);
   - `model/`: tipos;
   - `zod/`: esquemas para validar formulários.
3. Todos os pedidos passam por [`src/api/http.ts`](src/api/http.ts):
   - mesma origem (`/api/v1`) e cookies `same-origin`;
   - qualquer resposta fora de 2xx vira um `ApiError` com o problem+json e o request ID;
   - `errorMessage(err)` dá uma mensagem para pessoas, sem detalhes técnicos;
   - ao fim de 3 s aparece o aviso "A acordar o servidor"; ao fim de 75 s desiste (ADR-0007).
4. O TanStack Query repete uma vez falhas de rede e 5xx, nunca 4xx nem escritas ([`queryClient.ts`](src/api/queryClient.ts)).

## Testes

- **Vitest + Testing Library** para componentes e lógica.
- **MSW** simula a API ([`src/test/msw.ts`](src/test/msw.ts)): os testes nunca vão à rede, e um pedido sem resposta definida falha o teste.
- `renderWithQuery()` dá a cada teste um QueryClient novo.

## Deploy (Vercel)

[`vercel.json`](vercel.json):

- `/api/*` vai para a API na Render; tudo o resto vai para `index.html` (SPA).
- Cabeçalhos de segurança em todas as respostas: CSP só com `'self'` (sem `unsafe-inline`), HSTS, `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`, `Permissions-Policy`.
- `/assets/*` com cache de um ano (os nomes têm hash).
- As fontes nunca são embebidas como `data:` no CSS, para a CSP poder manter `font-src 'self'`.
- O URL da API e a versão de staging ficam definidos na #7.

Um teste ([`vercel-config.test.ts`](src/test/vercel-config.test.ts)) falha se o rewrite ou os cabeçalhos forem removidos.

## Estrutura

```
web/src/
├── main.tsx            arranque: tema, QueryClient, router
├── api/
│   ├── http.ts         cliente HTTP (ApiError, timeouts, aviso de lentidão)
│   ├── queryClient.ts  política de cache e novas tentativas
│   └── generated/      gerado pelo orval (não editar)
├── app/
│   ├── routes.tsx      todas as rotas e o menu do backoffice
│   ├── layouts/        PublicLayout, BackofficeLayout
│   ├── NotFoundPage.tsx
│   └── layouts.css     base global e layouts
├── shared/
│   ├── styles/tokens.css  tokens de cor, tipo e espaço (claro e escuro)
│   ├── theme/          ThemeToggle e preferência de tema
│   ├── lib/format.ts   €, kg, L e datas em pt-PT (hora de Lisboa)
│   └── ui/             Button, Chip, StockPill, SeasonTip, WakingServer, ícones
├── features/           uma pasta por funcionalidade
│   ├── catalogo/  auth/  painel/  venda/  stock/
│   └── produtos/  relatorios/  utilizadores/  auditoria/
└── test/              setup do Vitest, MSW, renderWithQuery
```

## Regras de segurança no lint

- `dangerouslySetInnerHTML` está proibido (XSS).
- `localStorage` está proibido: tokens e dados de sessão nunca vão para o browser de forma persistente (ADR-0005). A única exceção, explícita, é a preferência de tema em `shared/theme/theme.ts`.
