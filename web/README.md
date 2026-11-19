# web

Frontend em React + TypeScript, construído com Vite. Ver [docs/architecture/frontend.md](../docs/architecture/frontend.md).

> **Estado:** esqueleto com rotas e layouts. O aspeto visual (design system, componentes) espera pela escolha entre as propostas de design.

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

## Estrutura

```
web/src/
├── main.tsx            arranque: router e estilos base
├── app/
│   ├── routes.tsx      todas as rotas e o menu do backoffice
│   ├── layouts/        PublicLayout, BackofficeLayout
│   ├── NotFoundPage.tsx
│   └── base.css        estilos neutros provisórios
├── features/           uma pasta por funcionalidade
│   ├── catalogo/  auth/  painel/  venda/  stock/
│   └── produtos/  relatorios/  utilizadores/  auditoria/
└── test/setup.ts       configuração do Vitest
```

## Regras de segurança no lint

- `dangerouslySetInnerHTML` está proibido (XSS).
- `localStorage` está proibido: tokens e dados de sessão nunca vão para o browser de forma persistente (ADR-0005).
