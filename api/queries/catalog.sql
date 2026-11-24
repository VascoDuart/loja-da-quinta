-- Queries do módulo catalog. Só este módulo lê ou escreve categories,
-- products e product_images.
--
-- As quantidades saem e entram em milésimos inteiros (low_stock_milli), como
-- o dinheiro em cêntimos (ADR-0009): nunca há vírgula flutuante entre a base
-- de dados e a API.

-- name: ListCategories :many
SELECT c.id, c.name, c.created_at, c.updated_at, count(p.id)::int AS product_count
FROM categories c
LEFT JOIN products p ON p.category_id = c.id
GROUP BY c.id
ORDER BY lower(c.name);

-- name: CreateCategory :one
INSERT INTO categories (name) VALUES ($1)
RETURNING id, name, created_at, updated_at;

-- name: RenameCategory :one
UPDATE categories SET name = $2 WHERE id = $1
RETURNING id, name, created_at, updated_at;

-- name: DeleteCategory :execrows
DELETE FROM categories WHERE id = $1;

-- name: ListProducts :many
SELECT p.id, p.category_id, c.name AS category_name, p.name, p.unit, p.price_cents, p.vat_rate,
       p.season_months, (p.low_stock * 1000)::bigint AS low_stock_milli, p.is_public, p.active,
       i.sha256 AS image_sha256, p.created_at, p.updated_at
FROM products p
JOIN categories c ON c.id = p.category_id
LEFT JOIN product_images i ON i.product_id = p.id AND i.size = 'large'
ORDER BY lower(p.name);

-- name: GetProduct :one
SELECT p.id, p.category_id, c.name AS category_name, p.name, p.unit, p.price_cents, p.vat_rate,
       p.season_months, (p.low_stock * 1000)::bigint AS low_stock_milli, p.is_public, p.active,
       i.sha256 AS image_sha256, p.created_at, p.updated_at
FROM products p
JOIN categories c ON c.id = p.category_id
LEFT JOIN product_images i ON i.product_id = p.id AND i.size = 'large'
WHERE p.id = $1;

-- name: CreateProduct :one
INSERT INTO products (category_id, name, unit, price_cents, vat_rate, season_months, low_stock, is_public, active)
VALUES (sqlc.arg(category_id), sqlc.arg(name), sqlc.arg(unit), sqlc.arg(price_cents), sqlc.arg(vat_rate),
        sqlc.arg(season_months)::smallint[], sqlc.arg(low_stock_milli)::bigint::numeric / 1000,
        sqlc.arg(is_public), sqlc.arg(active))
RETURNING id;

-- Só grava se ninguém mudou o produto desde que o cliente o leu (If-Match):
-- sem linhas devolvidas, o produto não existe ou a versão é outra.
-- name: UpdateProduct :one
UPDATE products SET
  category_id   = sqlc.arg(category_id),
  name          = sqlc.arg(name),
  unit          = sqlc.arg(unit),
  price_cents   = sqlc.arg(price_cents),
  vat_rate      = sqlc.arg(vat_rate),
  season_months = sqlc.arg(season_months)::smallint[],
  low_stock     = sqlc.arg(low_stock_milli)::bigint::numeric / 1000,
  is_public     = sqlc.arg(is_public),
  active        = sqlc.arg(active)
WHERE id = sqlc.arg(id) AND updated_at = sqlc.arg(expected_updated_at)
RETURNING id;

-- name: ProductExists :one
SELECT EXISTS (SELECT 1 FROM products WHERE id = $1);

-- name: DeleteProduct :execrows
DELETE FROM products WHERE id = $1;

-- Bloqueia o produto durante a troca da imagem: dois uploads ao mesmo tempo
-- não deixam versões misturadas.
-- name: LockProduct :one
SELECT id FROM products WHERE id = $1 FOR UPDATE;

-- name: DeleteProductImages :execrows
DELETE FROM product_images WHERE product_id = $1;

-- name: InsertProductImage :exec
INSERT INTO product_images (product_id, size, data, width, height, sha256)
VALUES ($1, $2, $3, $4, $5, $6);

-- A versão pública da imagem é o hash da versão grande: entra no URL e
-- muda sempre que a imagem muda.
-- name: GetProductImage :one
SELECT i.data, i.sha256, l.sha256 AS version
FROM product_images i
JOIN product_images l ON l.product_id = i.product_id AND l.size = 'large'
WHERE i.product_id = $1 AND i.size = $2;

-- Catálogo público: só produtos ativos e públicos, e só as colunas que o
-- público pode ver (nunca IVA, limiar de stock, ativo nem datas). O limiar
-- vem só para o serviço decidir a disponibilidade; não sai para a API.
-- name: ListPublicProducts :many
SELECT p.id, c.name AS category_name, p.name, p.unit, p.price_cents, p.season_months,
       (p.low_stock * 1000)::bigint AS low_stock_milli, i.sha256 AS image_sha256
FROM products p
JOIN categories c ON c.id = p.category_id
LEFT JOIN product_images i ON i.product_id = p.id AND i.size = 'large'
WHERE p.active AND p.is_public
ORDER BY lower(c.name), lower(p.name);
