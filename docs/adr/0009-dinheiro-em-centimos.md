# 0009. Dinheiro em cêntimos e quantidades em `numeric(12,3)`

- **Estado:** Aceite
- **Data:** 2026-11-18

## Contexto

Vírgula flutuante produz erros de arredondamento (`0.1 + 0.2 != 0.3`). Há produtos vendidos ao kg e ao litro, com decimais.

## Decisão

- Valores monetários em **cêntimos inteiros** (`int` no PostgreSQL, `int64` em Go, `number` inteiro em TS).
- Quantidades em **`numeric(12,3)`** (até 3 casas decimais: gramas e mililitros).
- Arredondamento ao cêntimo **por linha**, meio para cima; o total é a soma das linhas.
- A conversão para euros com vírgula acontece só na apresentação.

## Consequências

- Totais exatos e reproduzíveis.
- Conversões na fronteira (formulários e apresentação), cobertas por testes.
- **Na API, as quantidades viajam em milésimos inteiros** (campos `…Milli`: `2500` = 2,5 kg), pela mesma razão dos cêntimos: o JSON nunca leva números com casas decimais. Para unidades, molhos e dúzias, o valor tem de ser múltiplo de 1000.
