#!/usr/bin/env bash
# Dispara o alerta diário de stock (RF-12, ADR-0015). É o que o workflow
# stock-alerts.yml corre, e também serve para o experimentar à mão:
#
#   scripts/stock-alerts.sh                       # staging; pede o token sem o mostrar
#   API_URL=http://localhost:8080 scripts/stock-alerts.sh
#
# O token vem de JOBS_TOKEN (no CI) ou é pedido no terminal. Pode correr-se as
# vezes que se quiser: a API envia no máximo uma vez por dia.
#
# Os logs do CI são públicos: só se mostra o estado, o dia e a forma de entrega.

set -euo pipefail

API_URL="${API_URL:-https://loja-da-quinta-api-staging.onrender.com}"

fail() {
  echo "::error::$1" >&2
  exit 1
}

token="${JOBS_TOKEN:-}"
if [ -z "$token" ]; then
  [ -t 0 ] || fail "Falta o JOBS_TOKEN."
  read -rsp "Token dos jobs (não aparece no ecrã): " token
  echo >&2
fi
token="$(printf '%s' "$token" | tr -d '[:space:]')"
[ -n "$token" ] || fail "Falta o JOBS_TOKEN."

# A API da Render adormece: o primeiro pedido pode demorar a acordá-la.
curl --fail --silent --show-error --max-time 90 --retry 5 --retry-delay 15 --retry-all-errors \
  "$API_URL/api/v1/healthz" > /dev/null || fail "A API não responde."

body="$(mktemp)"
trap 'rm -f "$body"' EXIT

# O token vai por stdin: não aparece na lista de processos nem nos logs.
# --retry só repete falhas passageiras (429, 5xx, tempo esgotado), e repetir é
# seguro.
code="$(printf 'X-Job-Token: %s\n' "$token" |
  curl --silent --show-error --max-time 60 --retry 3 --retry-delay 20 \
    --request POST --header @- --output "$body" --write-out '%{http_code}' \
    "$API_URL/api/v1/jobs/stock-alerts")" || fail "O pedido do alerta falhou."

# field NOME: o valor de um campo de texto da resposta (vazio se não houver).
field() {
  grep -o "\"$1\":\"[^\"]*\"" "$body" | head -n 1 | cut -d '"' -f 4 || true
}

[ "$code" = "200" ] || fail "A API respondeu $code. $(field detail)"

delivery="$(field delivery)"
echo "Estado: $(field status) · dia: $(field day)${delivery:+ · entrega: $delivery}"
