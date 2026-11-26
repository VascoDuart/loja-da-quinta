-- Counter sales (#15, UC-06, RF-07..09, ADR-0009, ADR-0010, ADR-0011).
--
-- A sale is an internal record, not a tax document (ADR-0011). Money is in
-- integer cents; quantities are numeric(12,3), in the product's unit.

-- +goose Up
-- Internal sale numbers (V-000042). Gaps are fine: this is not an invoice
-- series, and a rolled-back sale still uses up its number.
CREATE SEQUENCE sale_number_seq;

CREATE TABLE sales (
  id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  number           text        NOT NULL UNIQUE DEFAULT ('V-' || lpad(nextval('sale_number_seq')::text, 6, '0')),
  user_id          uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  payment          text        NOT NULL CHECK (payment IN ('numerario', 'mbway')),
  total_cents      bigint      NOT NULL CHECK (total_cents >= 0),
  vat_cents        bigint      NOT NULL CHECK (vat_cents >= 0 AND vat_cents <= total_cents),
  status           text        NOT NULL DEFAULT 'concluida' CHECK (status IN ('concluida', 'anulada')),
  -- The same key from the same user never records two sales (UC-06 7c). The
  -- hash of the request tells a retry from a different cart reusing the key.
  idempotency_key  text        NOT NULL CHECK (length(idempotency_key) BETWEEN 16 AND 64),
  request_hash     bytea       NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, idempotency_key)
);

-- Reports (#17) read sales by day.
CREATE INDEX sales_created_at_idx ON sales (created_at);

CREATE TRIGGER sales_set_updated_at
  BEFORE UPDATE ON sales
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One line per product. Name, unit, price and VAT are copied at the time of
-- the sale, so changing the catalog never changes old sales.
CREATE TABLE sale_items (
  id               uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_id          uuid          NOT NULL REFERENCES sales (id) ON DELETE RESTRICT,
  product_id       uuid          NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
  product_name     text          NOT NULL,
  unit             text          NOT NULL CHECK (unit IN ('kg', 'l', 'un', 'molho', 'duzia')),
  quantity         numeric(12,3) NOT NULL CHECK (quantity > 0),
  unit_price_cents bigint        NOT NULL CHECK (unit_price_cents >= 0),
  vat_rate         smallint      NOT NULL CHECK (vat_rate IN (6, 13, 23)),
  line_total_cents bigint        NOT NULL CHECK (line_total_cents >= 0),
  vat_cents        bigint        NOT NULL CHECK (vat_cents >= 0 AND vat_cents <= line_total_cents),
  UNIQUE (sale_id, product_id)
);

CREATE INDEX sale_items_product_idx ON sale_items (product_id);

-- Stock movements caused by a sale (venda) or by voiding it (anulacao, #16)
-- point to it; no other movement does. The append-only trigger on
-- stock_movements does not block ALTER TABLE.
ALTER TABLE stock_movements ADD COLUMN sale_id uuid REFERENCES sales (id) ON DELETE RESTRICT;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_sale_id_check
  CHECK ((type IN ('venda', 'anulacao')) = (sale_id IS NOT NULL));

CREATE INDEX stock_movements_sale_idx ON stock_movements (sale_id) WHERE sale_id IS NOT NULL;

-- +goose Down
DROP INDEX stock_movements_sale_idx;
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_sale_id_check;
ALTER TABLE stock_movements DROP COLUMN sale_id;
DROP TABLE sale_items;
DROP TABLE sales;
DROP SEQUENCE sale_number_seq;
