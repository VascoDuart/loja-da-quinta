-- Queries of the inventory module. Only this module reads or writes
-- stock_lots and stock_movements.
--
-- Quantities go in and out as integer thousandths (…_milli), like money in
-- cents (ADR-0009): there is never floating point between the database and
-- the API.

-- name: InsertLot :one
-- The unit is the product's when the quantity was validated: if it changed
-- meanwhile, the (product_id, unit) foreign key rejects the lot (00005).
INSERT INTO stock_lots (product_id, unit, qty_received, qty_left, harvested_on, expires_on)
VALUES (sqlc.arg(product_id), sqlc.arg(unit), sqlc.arg(quantity_milli)::bigint::numeric / 1000,
        sqlc.arg(quantity_milli)::bigint::numeric / 1000, sqlc.arg(harvested_on), sqlc.arg(expires_on))
RETURNING id, product_id, unit, (qty_received * 1000)::bigint AS qty_received_milli,
          (qty_left * 1000)::bigint AS qty_left_milli, harvested_on, expires_on, created_at;

-- name: InsertMovement :one
-- sale_id is set for sale (venda) and void (anulacao) movements only.
INSERT INTO stock_movements (lot_id, type, quantity, user_id, reason, sale_id)
VALUES (sqlc.arg(lot_id), sqlc.arg(type), sqlc.arg(quantity_milli)::bigint::numeric / 1000,
        sqlc.arg(user_id), sqlc.narg(reason), sqlc.narg(sale_id))
RETURNING id, created_at;

-- Sellable stock per product: the sum of what is left in the lots still
-- within their expiry date (a lot that expires today still counts). Products
-- without stock are left out.
-- name: StockByProduct :many
SELECT product_id, (sum(qty_left) * 1000)::bigint AS stock_milli
FROM stock_lots
WHERE product_id = ANY (sqlc.arg(product_ids)::uuid[])
  AND qty_left > 0
  AND expires_on >= sqlc.arg(today)::date
GROUP BY product_id;

-- Whether the product has any lot (with or without stock): its unit can no
-- longer change.
-- name: ProductHasLots :one
SELECT EXISTS (SELECT 1 FROM stock_lots WHERE product_id = $1);

-- name: GetLot :one
SELECT id, product_id, unit, (qty_received * 1000)::bigint AS qty_received_milli,
       (qty_left * 1000)::bigint AS qty_left_milli, harvested_on, expires_on, created_at
FROM stock_lots
WHERE id = $1;

-- Locks the lot until the end of the transaction: whoever wants to change the
-- same lot (another adjustment, a sale) waits, and then reads what is left.
-- It is the basis of every stock outflow (adjustments, breakage).
-- name: LockLot :one
SELECT id, product_id, unit, (qty_received * 1000)::bigint AS qty_received_milli,
       (qty_left * 1000)::bigint AS qty_left_milli, harvested_on, expires_on, created_at
FROM stock_lots
WHERE id = $1
FOR UPDATE;

-- Adds a signed delta to what is left in the lot. Used only with the lot
-- locked (LockLot, LockSellableLots) and after checking that it does not go
-- negative; the CHECK (qty_left >= 0) is the last line of defence.
-- name: AddToLot :one
UPDATE stock_lots SET qty_left = qty_left + sqlc.arg(delta_milli)::bigint::numeric / 1000
WHERE id = sqlc.arg(id)
RETURNING id, product_id, unit, (qty_received * 1000)::bigint AS qty_received_milli,
          (qty_left * 1000)::bigint AS qty_left_milli, harvested_on, expires_on, created_at;

-- Every lot with stock (expired ones too), per product and in FEFO order:
-- the basis of the stock overview and the alerts (#14). No limit: the stock
-- totals must count every lot.
-- name: ListStockedLots :many
SELECT id, product_id, unit, (qty_received * 1000)::bigint AS qty_received_milli,
       (qty_left * 1000)::bigint AS qty_left_milli, harvested_on, expires_on, created_at
FROM stock_lots
WHERE qty_left > 0
ORDER BY product_id, expires_on, created_at;

-- Claims the alert day. 0 rows: the day was already claimed. With another
-- request halfway through inserting the same day, waits for it to commit or
-- roll back.
-- name: ClaimAlertDay :execrows
INSERT INTO stock_alert_runs (day) VALUES (sqlc.arg(day)::date)
ON CONFLICT (day) DO NOTHING;

-- Releases the day when sending failed, so it is tried again.
-- name: ReleaseAlertDay :exec
DELETE FROM stock_alert_runs WHERE day = sqlc.arg(day)::date;

-- Lots of a product for adjustments: those that still have stock (expired
-- ones too, for breakage) and those still within their expiry date (a lot set
-- to 0 by mistake can still be corrected). In the order stock goes out
-- (FEFO), at most 200.
-- name: ListLots :many
SELECT id, product_id, unit, (qty_received * 1000)::bigint AS qty_received_milli,
       (qty_left * 1000)::bigint AS qty_left_milli, harvested_on, expires_on, created_at
FROM stock_lots
WHERE product_id = sqlc.arg(product_id)
  AND (qty_left > 0 OR expires_on >= sqlc.arg(today)::date)
ORDER BY expires_on, created_at
LIMIT 200;

-- Sellable lots of a product, locked until the end of the transaction, in
-- the order a sale takes stock (FEFO: earliest expiry first). A sale locks
-- its products in id order and each product's lots in this order, so two
-- sales always lock in the same order and never deadlock.
-- name: LockSellableLots :many
SELECT id, product_id, unit, (qty_received * 1000)::bigint AS qty_received_milli,
       (qty_left * 1000)::bigint AS qty_left_milli, harvested_on, expires_on, created_at
FROM stock_lots
WHERE product_id = sqlc.arg(product_id)
  AND qty_left > 0
  AND expires_on >= sqlc.arg(today)::date
ORDER BY expires_on, created_at, id
FOR UPDATE;
