-- Queries of the sales module. Only this module reads or writes sales and
-- sale_items. Quantities go in and out as integer thousandths (…_milli) and
-- money as integer cents (ADR-0009).

-- Inserts the sale unless the same user already used this idempotency key.
-- No row back: the key is taken. With another transaction halfway through
-- inserting the same key, this waits for it to commit (no row) or roll back
-- (inserts).
-- name: InsertSale :one
INSERT INTO sales (user_id, payment, total_cents, vat_cents, idempotency_key, request_hash)
VALUES (sqlc.arg(user_id), sqlc.arg(payment), sqlc.arg(total_cents), sqlc.arg(vat_cents),
        sqlc.arg(idempotency_key), sqlc.arg(request_hash))
ON CONFLICT (user_id, idempotency_key) DO NOTHING
RETURNING id, number, user_id, payment, total_cents, vat_cents, status, request_hash, created_at;

-- name: GetSaleByKey :one
SELECT id, number, user_id, payment, total_cents, vat_cents, status, request_hash, created_at
FROM sales
WHERE user_id = sqlc.arg(user_id) AND idempotency_key = sqlc.arg(idempotency_key);

-- name: InsertSaleItem :exec
INSERT INTO sale_items (sale_id, product_id, product_name, unit, quantity, unit_price_cents, vat_rate,
                        line_total_cents, vat_cents)
VALUES (sqlc.arg(sale_id), sqlc.arg(product_id), sqlc.arg(product_name), sqlc.arg(unit),
        sqlc.arg(quantity_milli)::bigint::numeric / 1000, sqlc.arg(unit_price_cents), sqlc.arg(vat_rate),
        sqlc.arg(line_total_cents), sqlc.arg(vat_cents));

-- name: ListSaleItems :many
SELECT product_id, product_name, unit, (quantity * 1000)::bigint AS quantity_milli, unit_price_cents,
       vat_rate, line_total_cents, vat_cents
FROM sale_items
WHERE sale_id = sqlc.arg(sale_id)
ORDER BY product_id;
