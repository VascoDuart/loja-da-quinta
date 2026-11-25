-- Queries do módulo inventory. Só este módulo lê ou escreve stock_lots e
-- stock_movements.
--
-- As quantidades saem e entram em milésimos inteiros (…_milli), como o
-- dinheiro em cêntimos (ADR-0009): nunca há vírgula flutuante entre a base de
-- dados e a API.

-- name: InsertLot :one
-- A unidade é a do produto quando a quantidade foi validada: se entretanto
-- mudou, a chave estrangeira (product_id, unit) recusa o lote (00005).
INSERT INTO stock_lots (product_id, unit, qty_received, qty_left, harvested_on, expires_on)
VALUES (sqlc.arg(product_id), sqlc.arg(unit), sqlc.arg(quantity_milli)::bigint::numeric / 1000,
        sqlc.arg(quantity_milli)::bigint::numeric / 1000, sqlc.arg(harvested_on), sqlc.arg(expires_on))
RETURNING id, product_id, unit, (qty_received * 1000)::bigint AS qty_received_milli,
          (qty_left * 1000)::bigint AS qty_left_milli, harvested_on, expires_on, created_at;

-- name: InsertMovement :one
INSERT INTO stock_movements (lot_id, type, quantity, user_id, reason)
VALUES (sqlc.arg(lot_id), sqlc.arg(type), sqlc.arg(quantity_milli)::bigint::numeric / 1000,
        sqlc.arg(user_id), sqlc.narg(reason))
RETURNING id, created_at;

-- Stock disponível por produto: a soma do que resta nos lotes ainda dentro
-- da validade (um lote que expira hoje ainda conta). Os produtos sem stock
-- não aparecem.
-- name: StockByProduct :many
SELECT product_id, (sum(qty_left) * 1000)::bigint AS stock_milli
FROM stock_lots
WHERE product_id = ANY (sqlc.arg(product_ids)::uuid[])
  AND qty_left > 0
  AND expires_on >= sqlc.arg(today)::date
GROUP BY product_id;

-- Se o produto tem algum lote (com ou sem stock): a unidade dele já não muda.
-- name: ProductHasLots :one
SELECT EXISTS (SELECT 1 FROM stock_lots WHERE product_id = $1);

-- name: GetLot :one
SELECT id, product_id, unit, (qty_received * 1000)::bigint AS qty_received_milli,
       (qty_left * 1000)::bigint AS qty_left_milli, harvested_on, expires_on, created_at
FROM stock_lots
WHERE id = $1;

-- Bloqueia o lote até ao fim da transação: quem quiser mexer no mesmo lote
-- (outro ajuste, uma venda) espera, e depois lê o que restar. É a base de
-- todas as saídas de stock (ajustes, quebras e, no #15, as vendas por FEFO).
-- name: LockLot :one
SELECT id, product_id, unit, (qty_received * 1000)::bigint AS qty_received_milli,
       (qty_left * 1000)::bigint AS qty_left_milli, harvested_on, expires_on, created_at
FROM stock_lots
WHERE id = $1
FOR UPDATE;

-- Soma ao que resta no lote (delta com sinal). Usa-se só com o lote
-- bloqueado (LockLot) e depois de o serviço ver que não fica negativo; o
-- CHECK (qty_left >= 0) é a última defesa.
-- name: AddToLot :one
UPDATE stock_lots SET qty_left = qty_left + sqlc.arg(delta_milli)::bigint::numeric / 1000
WHERE id = sqlc.arg(id)
RETURNING id, product_id, unit, (qty_received * 1000)::bigint AS qty_received_milli,
          (qty_left * 1000)::bigint AS qty_left_milli, harvested_on, expires_on, created_at;

-- Lotes de um produto para os ajustes: os que ainda têm stock (também fora
-- de validade, para a quebra) e os que ainda estão dentro da validade (um
-- lote posto a 0 por engano ainda se corrige). Pela ordem em que o stock sai
-- (FEFO), no máximo 200.
-- name: ListLots :many
SELECT id, product_id, unit, (qty_received * 1000)::bigint AS qty_received_milli,
       (qty_left * 1000)::bigint AS qty_left_milli, harvested_on, expires_on, created_at
FROM stock_lots
WHERE product_id = sqlc.arg(product_id)
  AND (qty_left > 0 OR expires_on >= sqlc.arg(today)::date)
ORDER BY expires_on, created_at
LIMIT 200;
