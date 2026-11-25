# 0016. Sem API Gateway dedicado: a borda é o rewrite da Vercel e a segurança fica na API

- **Estado:** Aceite
- **Data:** 2026-11-25

## Contexto

O browser precisa de falar com a API sem que o frontend e o backend fiquem expostos um ao outro. Um API Gateway (Kong, AWS API Gateway ou um serviço próprio) é o componente habitual para isto: ponto de entrada único, autenticação, limites de pedidos e encaminhamento. O projeto tem um backend, um monólito modular ([ADR-0001](0001-monolito-modular.md)), e alojamento gratuito ([ADR-0008](0008-alojamento-gratuito.md)).

## Opções consideradas

1. **Gateway dedicado** entre a Vercel e a API. Centraliza a autenticação e os limites, mas acrescenta um serviço a configurar e a manter, outro arranque a frio no plano gratuito e uma segunda camada que pode estar mal configurada.
2. **Rewrite da Vercel + segurança na API.** A Vercel encaminha `/api/*` para a API na mesma origem ([ADR-0007](0007-mesma-origem-via-rewrite.md)); a API faz a autenticação, a autorização e os limites, e cada regra é testada no código.

## Decisão

Opção 2. As funções habituais de um gateway estão distribuídas assim:

| Função | Onde está |
|---|---|
| Ponto de entrada único e separação frontend/backend | Rewrite da Vercel (`web/vercel.json`) |
| TLS, HSTS, CSP e cabeçalhos | Vercel e middleware da API |
| Cache na borda | CDN da Vercel (imagens imutáveis e catálogo de 60 s, [ADR-0014](0014-imagens-dos-produtos-no-postgresql.md)) |
| Autenticação e autorização | Middleware da API, com os papéis declarados na OpenAPI |
| Limites de pedidos | Por IP e, no login, por conta (middleware e serviço de autenticação) |
| Validação contra o contrato | Validador OpenAPI, antes dos handlers |
| Limites de tamanho e de tempo | Middleware da API |

## Consequências

- Um só serviço para configurar, testar e manter; sem um segundo arranque a frio.
- Não há CORS: o browser só vê a origem da Vercel.
- A segurança fica testada junto do código. O teste de autorização gerado a partir da OpenAPI ([`authz_spec_test.go`](../../api/test/integration/authz_spec_test.go)) cobre cada endpoint novo.
- Os limites de pedidos estão em memória, por isso só são exatos com uma instância da API.

## Quando reavaliar

- Separar o job de alertas ou as encomendas online (v2) em serviços próprios.
- Abrir a API a terceiros, com chaves e quotas por cliente.
- Ter mais de uma instância da API, porque os limites em memória deixam de ser globais.
