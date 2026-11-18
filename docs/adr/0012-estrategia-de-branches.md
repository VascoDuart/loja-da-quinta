# 0012. `main` + `dev` com branches curtos

- **Estado:** Aceite
- **Data:** 2026-11-18

## Contexto

Na `main` só deve entrar código completamente funcional e testado. Também é preciso um sítio para integrar e testar funcionalidades antes de uma release.

## Opções consideradas

1. Trunk-based: só `main` e branches curtos.
2. Git Flow completo (`develop`, `release/*`, `hotfix/*`).
3. `main` + `dev` com branches curtos e staging.

## Decisão

Opção 3, descrita em [CONTRIBUTING.md](../../CONTRIBUTING.md):

- `main`: produção. Só PRs de `dev` ou `hotfix/*`, com o E2E de staging a verde. Merge commit (sem squash). Cada merge gera uma tag semver.
- `dev`: integração. Cada merge faz deploy para staging.
- `feat/*`, `fix/*`, `refactor/*`, `test/*`, `docs/*`, `ci/*`: criados a partir da `dev`, com squash merge.
- `hotfix/*`: criados a partir da `main`; depois do merge, a `main` é integrada na `dev`.

## Consequências

- A `main` só recebe versões validadas em staging.
- Releases mais espaçadas e um ambiente extra (staging) para manter.
- A passagem de `dev` para `main` usa merge commit para os históricos não divergirem.
