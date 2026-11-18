# 0010. Stock por lotes com saída FEFO

- **Estado:** Aceite
- **Data:** 2026-11-18

## Contexto

Produtos frescos têm validade. Um contador único por produto não diz o que está prestes a estragar.

## Decisão

- Cada entrada cria um **lote** com quantidade, data de colheita e validade.
- Cada venda consome os lotes que **expiram primeiro** (FEFO), com `SELECT … FOR UPDATE` dentro da transação.
- Toda a alteração gera um **movimento** (`entrada`, `venda`, `ajuste`, `quebra`, `anulacao`).
- `qty_left` tem `CHECK (qty_left >= 0)`: o stock nunca fica negativo, mesmo com um bug.

## Consequências

- Alertas de validade precisos e histórico completo de stock.
- Uma anulação devolve a quantidade aos mesmos lotes.
- Modelo mais rico do que um contador, com mais testes de concorrência.
