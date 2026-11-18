# 0002. Monorepo com `api/`, `web/` e `openapi/`

- **Estado:** Aceite
- **Data:** 2026-11-18

## Contexto

Frontend e backend partilham um contrato (OpenAPI). Uma alteração ao contrato muda os dois lados.

## Opções consideradas

1. Dois repositórios (frontend e backend) e um terceiro para o contrato.
2. Monorepo.

## Decisão

Monorepo com `api/` (Go), `web/` (React), `openapi/` (contrato), `docs/` e `infra/`.

## Consequências

- Uma alteração ao contrato, ao backend e ao frontend entra num único PR, revisto e testado em conjunto.
- Um só sítio para documentação, issues e CI.
- O CI tem de evitar trabalho desnecessário: os jobs filtram por caminho (`api/**`, `web/**`).
