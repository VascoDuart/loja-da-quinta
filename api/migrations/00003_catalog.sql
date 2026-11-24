-- Catálogo: categorias, produtos e imagens (#10, RF-04, ADR-0009, ADR-0014).

-- +goose Up
CREATE TABLE categories (
  id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text        NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 60),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- "Fruta" e "fruta" são a mesma categoria.
CREATE UNIQUE INDEX categories_name_key ON categories (lower(name));

CREATE TRIGGER categories_set_updated_at
  BEFORE UPDATE ON categories
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE products (
  id              uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
  -- RESTRICT: uma categoria com produtos não se apaga.
  category_id     uuid          NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
  name            text          NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  unit            text          NOT NULL CHECK (unit IN ('kg', 'l', 'un', 'molho', 'duzia')),
  price_cents     integer       NOT NULL CHECK (price_cents BETWEEN 0 AND 10000000),
  vat_rate        smallint      NOT NULL CHECK (vat_rate IN (6, 13, 23)),
  -- Meses de 1 a 12, sem repetições. Vazio quer dizer todo o ano.
  season_months   smallint[]    NOT NULL DEFAULT '{}'
                                CHECK (season_months <@ ARRAY[1,2,3,4,5,6,7,8,9,10,11,12]::smallint[]),
  low_stock       numeric(12,3) NOT NULL DEFAULT 0 CHECK (low_stock >= 0),
  is_public       boolean       NOT NULL DEFAULT false,
  active          boolean       NOT NULL DEFAULT true,
  created_at      timestamptz   NOT NULL DEFAULT now(),
  updated_at      timestamptz   NOT NULL DEFAULT now(),
  -- Só kg e litros têm casas decimais.
  CHECK (unit IN ('kg', 'l') OR low_stock = trunc(low_stock))
);

-- Dois produtos com o mesmo nome confundiriam a pesquisa no ecrã de venda.
CREATE UNIQUE INDEX products_name_key ON products (lower(name));
CREATE INDEX products_category_idx ON products (category_id);

CREATE TRIGGER products_set_updated_at
  BEFORE UPDATE ON products
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Duas versões por produto, sempre JPEG reprocessado pela API (ADR-0014).
-- Trocar a imagem apaga e volta a inserir as duas linhas numa transação.
CREATE TABLE product_images (
  product_id   uuid        NOT NULL REFERENCES products (id) ON DELETE CASCADE,
  size         text        NOT NULL CHECK (size IN ('thumb', 'large')),
  data         bytea       NOT NULL CHECK (length(data) BETWEEN 1 AND 2000000),
  content_type text        NOT NULL DEFAULT 'image/jpeg' CHECK (content_type = 'image/jpeg'),
  width        integer     NOT NULL CHECK (width BETWEEN 1 AND 800),
  height       integer     NOT NULL CHECK (height BETWEEN 1 AND 800),
  sha256       bytea       NOT NULL CHECK (length(sha256) = 32),
  created_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (product_id, size)
);

-- +goose Down
DROP TABLE product_images;
DROP TABLE products;
DROP TABLE categories;
