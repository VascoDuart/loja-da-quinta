# Modelo de dados

```mermaid
erDiagram
    users ||--o{ refresh_tokens : "tem"
    users ||--o{ audit_log : "gera"
    users ||--o{ sales : "regista"
    users ||--o{ stock_movements : "regista"
    categories ||--o{ products : "agrupa"
    products ||--o{ stock_lots : "tem"
    products ||--o{ product_images : "tem até 2 versões"
    products ||--o{ sale_items : "vendido em"
    sales ||--|{ sale_items : "contém"
    stock_lots ||--o{ stock_movements : "movimenta"
    sales ||--o{ stock_movements : "origina"

    users {
        uuid id PK
        citext email UK
        text password_hash
        text role "gestor | funcionario"
        bool active
        timestamptz created_at
        timestamptz updated_at
    }
    refresh_tokens {
        uuid id PK
        uuid user_id FK
        bytea token_hash UK
        uuid family_id
        timestamptz expires_at
        timestamptz revoked_at "null"
        timestamptz created_at
    }
    audit_log {
        bigint id PK
        uuid actor_id FK "null"
        text action
        text entity
        text entity_id
        jsonb metadata
        inet ip
        timestamptz created_at
    }
    categories {
        uuid id PK
        text name UK
        timestamptz created_at
        timestamptz updated_at
    }
    products {
        uuid id PK
        uuid category_id FK
        text name
        text unit "kg | l | un | molho | duzia"
        int price_cents "CHECK >= 0"
        smallint vat_rate "6 | 13 | 23"
        smallint_array season_months
        numeric low_stock "12,3"
        bool is_public
        bool active
        timestamptz created_at
        timestamptz updated_at
    }
    product_images {
        uuid product_id PK "FK"
        text size PK "thumb | large"
        bytea data "JPEG reprocessado"
        text content_type "image/jpeg"
        int width
        int height
        bytea sha256 "versão no URL"
        timestamptz created_at
    }
    stock_lots {
        uuid id PK
        uuid product_id FK
        numeric qty_received "12,3"
        numeric qty_left "12,3 CHECK >= 0"
        date harvested_on
        date expires_on
        timestamptz created_at
        timestamptz updated_at
    }
    sales {
        uuid id PK
        text number UK
        uuid user_id FK
        text payment "numerario | mbway"
        int total_cents
        text status "concluida | anulada"
        timestamptz created_at
        timestamptz updated_at
    }
    sale_items {
        uuid id PK
        uuid sale_id FK
        uuid product_id FK
        numeric quantity "12,3"
        int unit_price_cents
        smallint vat_rate
    }
    stock_movements {
        bigint id PK
        uuid lot_id FK
        text type "entrada | venda | ajuste | quebra | anulacao"
        numeric quantity "12,3"
        uuid sale_id FK "null"
        uuid user_id FK
        text reason
        timestamptz created_at
    }
```

## Decisões

- **Stock disponível** é a soma de `qty_left` dos lotes. Cada alteração deixa um registo em `stock_movements`, que funciona como histórico completo ([ADR-0010](../adr/0010-stock-por-lotes-fefo.md)).
- **Dinheiro em cêntimos inteiros** e quantidades em `numeric(12,3)` para pesos em kg e volumes em litros ([ADR-0009](../adr/0009-dinheiro-em-centimos.md)).
- **As linhas da venda guardam o preço e o IVA da altura**, para que mudar um preço não altere vendas antigas.
- **Datas em UTC** (`timestamptz`); a conversão para Europe/Lisbon é feita na apresentação.
- **`created_at` e `updated_at` nas tabelas que mudam** (`users`, `categories`, `products`, `stock_lots`, `sales`), ambos `timestamptz NOT NULL DEFAULT now()`. O `updated_at` é mantido por um trigger, para nunca depender de o código se lembrar de o atualizar:

  ```sql
  CREATE FUNCTION set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
  BEGIN
    NEW.updated_at := now();
    RETURN NEW;
  END $$;

  CREATE TRIGGER products_set_updated_at
    BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
  ```

- **Sem `updated_at` onde os registos nunca mudam:** `audit_log` e `stock_movements` são só de acrescento, e `sale_items` é gravado com a venda e não se altera. Um `updated_at` nestas tabelas daria a entender que podem ser editadas. `refresh_tokens` só tem `created_at`; a única alteração possível já fica em `revoked_at`.
- **O `updated_at` serve também de controlo de concorrência otimista:** a API devolve-o como `ETag` e exige `If-Match` ao editar um produto. Se dois gestores editarem o mesmo produto ao mesmo tempo, o segundo recebe `412 Precondition Failed` em vez de apagar as alterações do primeiro sem saber.
- **O `updated_at` diz quando, não quem nem o quê:** para isso existe a auditoria.
- **Imagens em `product_images`** ([ADR-0014](../adr/0014-imagens-dos-produtos-no-postgresql.md)): duas versões por produto (`thumb` 160 px, `large` 800 px), sempre reprocessadas em JPEG pela API. O `sha256` entra no URL, que é imutável e fica na cache da CDN. Trocar a imagem apaga e volta a inserir as duas linhas na mesma transação; por isso não há `updated_at`. `ON DELETE CASCADE` a partir de `products`.
- **Enums como `text` com `CHECK`**, em vez de tipos `ENUM` do PostgreSQL, para facilitar migrações.
- **Auditoria só de acrescento:** o papel `app` tem apenas `INSERT` e `SELECT` em `audit_log`.

## Papéis PostgreSQL

| Papel | Permissões | Usado por |
|---|---|---|
| `migrator` | DDL no schema da app | Workflow de deploy (goose) |
| `app` | DML nas tabelas da app; só `INSERT`/`SELECT` em `audit_log` | API |
| `readonly` | `SELECT` | DBeaver e diagnóstico |

Localmente, os papéis são criados por [`infra/postgres/init-roles.sh`](../../infra/postgres/init-roles.sh). As tabelas ficam no schema `loja`, cujo dono é o `migrator`.
