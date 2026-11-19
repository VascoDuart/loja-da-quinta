# 0013. Design system "Horta Fresca" com modo claro e escuro

- **Estado:** Aceite
- **Data:** 2026-11-19

## Contexto

Foram avaliadas três direções visuais (A · Horta Fresca, B · Caderno de Campo, C · Estação) num quadro de design, seguido de um protótipo interativo. A app precisa de modo claro e escuro e tem de funcionar bem em tablet, ao balcão.

## Opções consideradas

1. Uma das três direções sem alterações.
2. A direção A, com mais verde, imagens dos produtos, a dica da época da C e modo escuro.
3. shadcn/ui + Tailwind (o previsto em `frontend.md`) vs. componentes próprios com variáveis CSS.

## Decisão

**Direção A, refinada** (opção 2), implementada com **componentes próprios e variáveis CSS**, sem Tailwind nem shadcn/ui.

- **Paleta da A:** fundo `#F6F8F4`, superfícies brancas, texto `#17251B`, verde `#2F7D32`, "folha" `#E3F1E1`.
- **Mais verde onde orienta:** barra lateral e cabeçalho verdes com letras brancas; filtros ativos, total e ação principal em verde.
- **Imagem em cada produto** e **dica da época** em âmbar discreto.
- **Plus Jakarta Sans**, servida pelo próprio site (`@fontsource`), sem pedidos ao Google Fonts.
- **Modo escuro** com os mesmos componentes: só os tokens mudam. Segue `prefers-color-scheme`; a escolha manual fica em `data-theme` no `<html>` e é lembrada no browser.

Tokens e componentes em [docs/design/design-system.md](../design/design-system.md).

## Consequências

- Nenhuma cor solta nos componentes: tudo passa por tokens, por isso um terceiro tema é só mais um bloco de variáveis.
- Sem Tailwind nem shadcn/ui: menos dependências e um CSS pequeno (~9 KB), mas os componentes são escritos à mão.
- A fonte local evita pedidos a terceiros, o que facilita uma CSP restrita (`font-src 'self'`).
- A preferência de tema usa `localStorage`, numa exceção explícita à regra do ESLint: é só uma preferência visual, nunca dados de sessão.
- **Imagens dos produtos entram no MVP**, o que exige decidir onde as guardar (ADR próprio, pendente).
- Protótipo de referência: quadro "Loja da Quinta · Propostas de design".
