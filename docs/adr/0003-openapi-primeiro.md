# 0003. OpenAPI primeiro, com geração de código

- **Estado:** Aceite
- **Data:** 2026-11-18

## Contexto

Tipos duplicados à mão entre Go e TypeScript divergem com o tempo e só se nota em produção.

## Opções consideradas

1. Escrever o código e gerar a spec a partir dele.
2. Escrever a spec primeiro (`openapi/openapi.yaml`) e gerar o código dos dois lados.
3. Não ter spec formal.

## Decisão

Spec primeiro, em OpenAPI 3.1.

- Backend: `oapi-codegen` gera a interface do servidor e os tipos; a validação de pedidos usa a mesma spec.
- Frontend: `orval` gera o cliente, os hooks do TanStack Query e os esquemas zod.

## Consequências

- Uma mudança no contrato que parta um dos lados falha na compilação, não em produção.
- A spec é documentação sempre atualizada da API.
- Escreve-se YAML antes do código. O CI verifica que o código gerado está atualizado (`make gen` seguido de `git diff --exit-code`).
