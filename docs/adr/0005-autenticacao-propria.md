# 0005. Autenticação própria com argon2id, JWT curto e refresh rotativo

- **Estado:** Aceite
- **Data:** 2026-11-18

## Contexto

Só há utilizadores internos (gestor e funcionários). O orçamento é zero. Um objetivo do projeto é demonstrar conhecimento de segurança.

## Opções consideradas

1. Keycloak (OIDC), alojado: pesado para o plano gratuito.
2. SaaS (Auth0, Clerk): rápido, mas mostra pouco e cria dependência externa.
3. Autenticação própria, seguindo as recomendações OWASP.

## Decisão

Autenticação própria:

- Palavras-passe com **argon2id** (parâmetros recomendados pela OWASP).
- **Access token JWT** de 15 minutos, assinado com Ed25519 e com `kid` para rodar chaves. Fica só em memória no browser.
- **Refresh token opaco** de 7 dias, guardado só como hash, **rodado a cada uso**, com deteção de reutilização: se um refresh já usado aparecer, a família inteira é revogada.
- Cookie `HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth`, com cabeçalho `X-CSRF` obrigatório em refresh e logout.
- Rate limiting por IP e por conta no login.

O fluxo está em [docs/security/controls.md](../security/controls.md).

## Consequências

- Toda a responsabilidade de segurança é nossa: exige testes dedicados e revisão contra o ASVS.
- Sem dependências externas nem custos.
- Login social e MFA ficam para depois; a arquitetura permite acrescentá-los.
