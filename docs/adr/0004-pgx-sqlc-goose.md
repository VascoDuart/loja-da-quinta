# 0004. pgx + sqlc em vez de ORM; goose para migrações

- **Estado:** Aceite
- **Data:** 2026-11-18

## Contexto

A lógica de stock depende de SQL preciso: bloqueios (`FOR UPDATE`), ordenação por validade e restrições `CHECK`.

## Opções consideradas

1. ORM (GORM, ent).
2. SQL escrito à mão com `database/sql`.
3. `sqlc`: SQL escrito à mão, código Go tipado gerado.

## Decisão

`pgx` v5 como driver, `sqlc` para gerar código tipado a partir de `api/queries/*.sql` e `goose` para migrações em SQL puro (`api/migrations/`).

## Consequências

- Queries explícitas, revistas em PR e tipadas em compilação.
- Sem SQL gerado escondido nem problemas de N+1 acidentais.
- Escreve-se mais SQL do que com um ORM.
- As migrações correm com o papel `migrator`, separado do papel `app` da API.
