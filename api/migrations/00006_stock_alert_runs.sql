-- Alertas diários de stock (#14, RF-12, ADR-0015).
--
-- Um dia (de Lisboa) em que o alerta foi enviado. A chave primária é o que
-- garante o "no máximo uma vez por dia", também com dois disparos ao mesmo
-- tempo: quem chega em segundo não consegue inserir o mesmo dia. A linha é
-- inserida antes do envio e apagada se o envio falhar, para se voltar a
-- tentar.

-- +goose Up
CREATE TABLE stock_alert_runs (
  day        date        PRIMARY KEY,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE stock_alert_runs;
