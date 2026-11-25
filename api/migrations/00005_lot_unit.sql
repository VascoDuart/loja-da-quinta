-- A unidade de um produto com lotes não muda (#13).
--
-- As quantidades dos lotes estão na unidade do produto: mudar de kg para un
-- reinterpretava 12,5 kg como 12,5 unidades (stock fracionário numa unidade
-- inteira). O serviço do catálogo recusa a mudança com uma mensagem, mas a
-- verificação e a gravação são dois passos: entre eles pode entrar um lote.
--
-- A garantia fica aqui. Cada lote guarda a unidade em que a quantidade foi
-- validada, com uma chave estrangeira composta para (products.id, unit):
--
--   - mudar a unidade de um produto com lotes viola a chave (ON UPDATE
--     RESTRICT), mesmo que o lote tenha entrado entre a verificação e a
--     gravação;
--   - com (id, unit) numa chave única, mudar a unidade passa a ser uma
--     atualização de chave: o UPDATE fica com FOR UPDATE na linha do produto,
--     que choca com o FOR KEY SHARE da chave estrangeira de quem insere um
--     lote. Quem chega depois espera pelo outro e volta a verificar: ou a
--     unidade já não muda (há um lote), ou o lote já não entra (a unidade
--     mudou).
--
-- As outras alterações do produto (preço, nome) não tocam na chave e não
-- bloqueiam as entradas de stock.

-- +goose Up
ALTER TABLE products ADD CONSTRAINT products_id_unit_key UNIQUE (id, unit);

ALTER TABLE stock_lots ADD COLUMN unit text;
UPDATE stock_lots l SET unit = p.unit FROM products p WHERE p.id = l.product_id;
ALTER TABLE stock_lots ALTER COLUMN unit SET NOT NULL;

ALTER TABLE stock_lots DROP CONSTRAINT stock_lots_product_id_fkey;
ALTER TABLE stock_lots ADD CONSTRAINT stock_lots_product_unit_fkey
  FOREIGN KEY (product_id, unit) REFERENCES products (id, unit)
  ON UPDATE RESTRICT ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE stock_lots DROP CONSTRAINT stock_lots_product_unit_fkey;
ALTER TABLE stock_lots ADD CONSTRAINT stock_lots_product_id_fkey
  FOREIGN KEY (product_id) REFERENCES products (id) ON DELETE RESTRICT;
ALTER TABLE stock_lots DROP COLUMN unit;
ALTER TABLE products DROP CONSTRAINT products_id_unit_key;
