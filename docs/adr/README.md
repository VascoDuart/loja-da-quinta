# Architecture Decision Records

Cada decisão de arquitetura relevante fica registada num ADR, no formato de [Michael Nygard](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions): contexto, decisão e consequências. Um ADR aceite não se edita; se a decisão mudar, cria-se um novo que o substitui.

Para um novo ADR, copia [`template.md`](template.md) e usa o número seguinte.

| ADR | Decisão | Estado |
|---|---|---|
| [0001](0001-monolito-modular.md) | Monólito modular em Go | Aceite |
| [0002](0002-monorepo.md) | Monorepo com `api/`, `web/` e `openapi/` | Aceite |
| [0003](0003-openapi-primeiro.md) | OpenAPI primeiro, com geração de código | Aceite |
| [0004](0004-pgx-sqlc-goose.md) | pgx + sqlc em vez de ORM; goose para migrações | Aceite |
| [0005](0005-autenticacao-propria.md) | Autenticação própria com argon2id, JWT curto e refresh rotativo | Aceite |
| [0006](0006-spa-com-vite.md) | SPA com Vite em vez de Next.js | Aceite |
| [0007](0007-mesma-origem-via-rewrite.md) | Mesma origem através do rewrite da Vercel | Aceite (validado no spike #3) |
| [0008](0008-alojamento-gratuito.md) | Alojamento gratuito: Vercel, Render, Neon e Resend | Aceite |
| [0009](0009-dinheiro-em-centimos.md) | Dinheiro em cêntimos e quantidades em `numeric(12,3)` | Aceite |
| [0010](0010-stock-por-lotes-fefo.md) | Stock por lotes com saída FEFO | Aceite |
| [0011](0011-venda-nao-e-documento-fiscal.md) | A venda ao balcão não é documento fiscal | Aceite |
| [0012](0012-estrategia-de-branches.md) | `main` + `dev` com branches curtos | Aceite |
| [0013](0013-design-system-horta-fresca.md) | Design system "Horta Fresca" com modo claro e escuro | Aceite |
