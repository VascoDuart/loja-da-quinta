# web

Frontend em React + TypeScript, construído com Vite. Ver [docs/architecture/frontend.md](../docs/architecture/frontend.md).

> Ainda sem código. O projeto Vite é criado no primeiro PR de funcionalidade.

## Estrutura prevista

```
web/
├── src/
│   ├── app/            router, providers, layouts
│   ├── features/       catalogo, painel, stock, venda, produtos, relatorios, utilizadores
│   ├── shared/         ui (shadcn), lib, hooks
│   └── api/            gerado pelo orval (não editar)
├── e2e/                Playwright
├── orval.config.ts
└── vercel.json         rewrite de /api e cabeçalhos de segurança
```
