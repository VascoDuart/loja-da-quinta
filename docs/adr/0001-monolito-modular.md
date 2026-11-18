# 0001. Monólito modular em Go

- **Estado:** Aceite
- **Data:** 2026-11-18

## Contexto

O backend tem sete áreas de negócio (auth, users, catalog, inventory, sales, reports, audit). A venda e o abate de stock têm de ser consistentes: não pode existir uma venda sem o stock abatido, nem o contrário. A equipa é uma pessoa e o alojamento tem de ser gratuito.

## Opções consideradas

1. **Microsserviços**, um por área, com base de dados própria.
2. **Microsserviços com um middleware central** de comunicação interna.
3. **Monólito modular**: um processo, módulos com fronteiras explícitas.

## Decisão

Monólito modular em Go, com arquitetura hexagonal em cada módulo.

- Venda e stock ficam na mesma transação ACID; com microsserviços seria preciso uma saga com compensações.
- Menos superfície de ataque: não há tráfego interno pela rede a autenticar, cifrar e limitar.
- Um middleware central seria um ponto único de falha e de compromisso, contrário ao modelo zero trust.
- O plano gratuito da Render não tem rede privada, e as 750 horas mensais chegam para um serviço sempre ligado, não para vários.

As fronteiras são verificadas no CI: um módulo só importa a interface do serviço de outro, nunca o repositório nem os tipos internos (`depguard`).

## Consequências

- Um único deploy, uma única imagem e testes de integração simples.
- Não há isolamento por processo: uma falha grave derruba a API inteira.
- Escala-se a aplicação inteira, não um módulo. Para o volume de uma loja, isto é irrelevante.
- Na v2, o job de alertas é o primeiro candidato a serviço separado, com o outbox pattern (tabela de eventos lida com `FOR UPDATE SKIP LOCKED`), sem broker pago.
