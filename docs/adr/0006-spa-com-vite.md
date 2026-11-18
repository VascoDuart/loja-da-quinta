# 0006. SPA com Vite em vez de Next.js

- **Estado:** Aceite
- **Data:** 2026-11-18

## Contexto

O backend é uma API em Go. O frontend tem de ser alojado gratuitamente. O SEO só importa no catálogo público de uma quinta local.

## Opções consideradas

1. Next.js (SSR/SSG, servidor Node).
2. SPA React + TypeScript construída com Vite.

## Decisão

SPA com Vite. O resultado é só ficheiros estáticos, servidos pela CDN da Vercel.

## Consequências

- Arquitetura simples: frontend estático → API Go → PostgreSQL, sem um segundo servidor com lógica.
- Sem SSR: o catálogo público tem SEO limitado. Se isso vier a importar, a página do catálogo pode ser pré-renderizada no build.
