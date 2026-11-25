-- Stock por lotes e movimentos de stock (#12, RF-05, ADR-0009, ADR-0010).
--
-- As quantidades são numeric(12,3): milésimos da unidade (gramas, mililitros).
-- A API recebe-as e devolve-as em milésimos inteiros (ADR-0009).

-- +goose Up
CREATE TABLE stock_lots (
  id           uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
  -- RESTRICT: um produto com histórico de stock não se apaga (409); desativa-se.
  product_id   uuid          NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
  qty_received numeric(12,3) NOT NULL CHECK (qty_received > 0),
  -- O stock nunca fica negativo, mesmo com um bug (ADR-0010, RNF-11).
  qty_left     numeric(12,3) NOT NULL CHECK (qty_left >= 0),
  harvested_on date          NOT NULL,
  expires_on   date          NOT NULL,
  created_at   timestamptz   NOT NULL DEFAULT now(),
  updated_at   timestamptz   NOT NULL DEFAULT now(),
  CHECK (expires_on >= harvested_on)
);

-- Stock disponível por produto e, mais tarde, a saída FEFO (#15): os lotes
-- com stock de um produto, por validade.
CREATE INDEX stock_lots_product_expiry_idx ON stock_lots (product_id, expires_on) WHERE qty_left > 0;

CREATE TRIGGER stock_lots_set_updated_at
  BEFORE UPDATE ON stock_lots
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Histórico de stock: cada alteração de um lote deixa aqui um movimento, com
-- quem a fez. A quantidade tem sinal: entra (+) ou sai (-) do lote. A coluna
-- sale_id chega com as vendas (#15), já com a chave estrangeira.
CREATE TABLE stock_movements (
  id         bigint        GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  lot_id     uuid          NOT NULL REFERENCES stock_lots (id) ON DELETE RESTRICT,
  type       text          NOT NULL CHECK (type IN ('entrada', 'venda', 'ajuste', 'quebra', 'anulacao')),
  quantity   numeric(12,3) NOT NULL,
  user_id    uuid          NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  reason     text          CHECK (reason IS NULL OR length(btrim(reason)) BETWEEN 1 AND 500),
  created_at timestamptz   NOT NULL DEFAULT now(),
  CHECK (CASE type
           WHEN 'entrada'  THEN quantity > 0
           WHEN 'anulacao' THEN quantity > 0
           WHEN 'venda'    THEN quantity < 0
           WHEN 'quebra'   THEN quantity < 0
           ELSE quantity <> 0 -- ajuste: para mais ou para menos
         END)
);

CREATE INDEX stock_movements_lot_idx ON stock_movements (lot_id);

-- Só de acrescento, para toda a gente (incluindo o dono das tabelas): um
-- movimento nunca se altera nem se apaga. Corrigir um erro é registar outro
-- movimento (ajuste).
-- +goose StatementBegin
CREATE FUNCTION stock_movements_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'stock_movements só aceita INSERT (% recusado)', TG_OP
    USING ERRCODE = 'insufficient_privilege';
END $$;
-- +goose StatementEnd

CREATE TRIGGER stock_movements_no_update_delete
  BEFORE UPDATE OR DELETE ON stock_movements
  FOR EACH ROW EXECUTE FUNCTION stock_movements_append_only();

CREATE TRIGGER stock_movements_no_truncate
  BEFORE TRUNCATE ON stock_movements
  FOR EACH STATEMENT EXECUTE FUNCTION stock_movements_append_only();

-- Além do trigger, o papel app (o da API) fica só com INSERT e SELECT nos
-- movimentos. Os privilégios por omissão dão-lhe DML completo
-- (infra/postgres/init-roles.sh); aqui tira-se o resto. O papel só existe no
-- compose e na Neon, não no PostgreSQL dos testes de integração.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app') THEN
    REVOKE UPDATE, DELETE, TRUNCATE ON stock_movements FROM app;
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE stock_movements;
DROP FUNCTION stock_movements_append_only();
DROP TABLE stock_lots;
