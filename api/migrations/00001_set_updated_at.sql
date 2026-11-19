-- Função usada por um trigger em cada tabela com updated_at
-- (docs/architecture/data-model.md). Fica na primeira migração porque as
-- tabelas dos módulos dependem dela.

-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION set_updated_at();
