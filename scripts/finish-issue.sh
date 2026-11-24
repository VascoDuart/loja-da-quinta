#!/usr/bin/env bash
# Fecha um issue de ponta a ponta, sem passos à mão:
#
#   1. espera pelos checks obrigatórios do PR (ci-ok e security-ok);
#   2. faz squash merge para a dev, com "Closes #N", e apaga o branch;
#   3. espera pelo deploy-staging desse commit e resume os testes E2E;
#   4. se o deploy passou, fecha o issue (com o link do run) e move o cartão
#      do GitHub Projects para Done.
#
# Uso:
#   scripts/finish-issue.sh --pr 35 --issue 10   # tudo
#   scripts/finish-issue.sh --pr 35              # merge e deploy (sem issue)
#   scripts/finish-issue.sh --issue 10           # só esperar pelo deploy da dev e fechar
#
# Se o deploy falhar, pára antes de fechar o issue e mostra o run.
set -euo pipefail

REPO_OWNER="VascoDuart"
PROJECT_TITLE="Loja da Quinta"
WORKFLOW="deploy-staging.yml"
# Os checks que a proteção da dev e da main exige (ver CONTRIBUTING.md).
REQUIRED_CHECKS="ci-ok security-ok"

pr="" issue=""
while [ $# -gt 0 ]; do
  case "$1" in
    --pr) pr="$2"; shift 2 ;;
    --issue) issue="$2"; shift 2 ;;
    -h|--help) sed -n '2,17p' "$0"; exit 0 ;;
    *) echo "Argumento desconhecido: $1" >&2; exit 2 ;;
  esac
done
[ -n "$pr$issue" ] || { sed -n '2,17p' "$0"; exit 2; }

step() { printf '\n== %s\n' "$*"; }
cd "$(git rev-parse --show-toplevel)"

# --- 1 e 2: checks e merge -------------------------------------------------
if [ -n "$pr" ]; then
  step "PR #$pr: à espera dos checks obrigatórios ($REQUIRED_CHECKS)"
  # Espera-se por nome: um check que ainda não começou não aparece na lista,
  # e "nenhum pendente" não quer dizer "todos passaram".
  for _ in $(seq 1 120); do  # até 40 minutos
    states=$(gh pr checks "$pr" --json name,bucket -q '.[] | "\(.name)=\(.bucket)"' 2>/dev/null || true)
    missing=0 failed=""
    for check in $REQUIRED_CHECKS; do
      bucket=$(grep -E "^$check=" <<<"$states" | head -1 | cut -d= -f2)
      case "$bucket" in
        pass) ;;
        fail|cancel) failed="$failed $check" ;;
        *) missing=1 ;;
      esac
    done
    if [ -n "$failed" ]; then
      echo "Checks a falhar:$failed. Não faço merge." >&2
      gh pr checks "$pr" >&2 || true
      exit 1
    fi
    [ "$missing" = 0 ] && break
    sleep 20
  done
  [ "$missing" = 0 ] || { echo "Os checks não terminaram a tempo." >&2; exit 1; }
  for check in $REQUIRED_CHECKS; do echo "   $check: pass"; done

  title=$(gh pr view "$pr" --json title -q .title)
  body=""
  [ -n "$issue" ] && body="Closes #$issue"
  step "Squash merge: $title (#$pr)"
  gh pr merge "$pr" --squash --delete-branch --subject "$title (#$pr)" --body "$body" > /dev/null
fi

git checkout -q dev
git pull -q --ff-only
sha=$(git rev-parse HEAD)
echo "   dev em ${sha:0:7}: $(git log -1 --format=%s)"

# --- 3: deploy de staging --------------------------------------------------
step "À espera do deploy-staging de ${sha:0:7}"
run=""
for _ in $(seq 1 24); do  # o run pode demorar alguns segundos a aparecer
  run=$(gh run list --workflow "$WORKFLOW" --commit "$sha" --limit 1 --json databaseId -q '.[0].databaseId // empty')
  [ -n "$run" ] && break
  sleep 5
done
[ -n "$run" ] || { echo "Não encontrei o deploy-staging de $sha." >&2; exit 1; }
url=$(gh run view "$run" --json url -q .url)
echo "   $url"

if gh run watch "$run" --interval 20 --exit-status > /dev/null 2>&1; then
  ok=1
else
  ok=0
fi
gh run view "$run" --json jobs -q '.jobs[] | "   \(.name): \(.conclusion)"'

e2e=$(gh run view "$run" --json jobs -q '.jobs[] | select(.name=="e2e") | .databaseId')
summary=""
if [ -n "$e2e" ]; then
  # Linhas finais do Playwright: "N passed", "N flaky", "N failed", "N skipped".
  summary=$(gh run view --job "$e2e" --log 2>/dev/null \
    | grep -oE '[[:space:]][0-9]+ (passed|failed|flaky|skipped)([[:space:]]|$)' \
    | awk '{print $1, $2}' | paste -sd ',' - | sed 's/,/, /g')
  echo "   E2E: ${summary:-sem resumo}"
  if grep -q 'flaky' <<<"$summary"; then
    echo "   Atenção: há testes instáveis. Um teste instável costuma esconder um bug; vê o log do run." >&2
  fi
fi

if [ "$ok" != 1 ]; then
  echo "O deploy-staging falhou: o issue fica aberto. Vê $url" >&2
  exit 1
fi

# --- 4: fechar o issue e mover o cartão -----------------------------------
[ -n "$issue" ] || { step "Feito (sem issue para fechar)."; exit 0; }

step "Issue #$issue: fechar e mover o cartão para Done"
if [ "$(gh issue view "$issue" --json state -q .state)" = "OPEN" ]; then
  gh issue close "$issue" --comment "Concluído${pr:+ no PR #$pr}. deploy-staging verde${summary:+ (E2E: $summary)}: $url" > /dev/null
  echo "   fechado"
else
  echo "   já estava fechado"
fi

number=$(gh project list --owner "$REPO_OWNER" --format json -q ".projects[] | select(.title==\"$PROJECT_TITLE\") | .number")
project_id=$(gh project view "$number" --owner "$REPO_OWNER" --format json -q .id)
field_id=$(gh project field-list "$number" --owner "$REPO_OWNER" --format json -q '.fields[] | select(.name=="Status") | .id')
done_id=$(gh project field-list "$number" --owner "$REPO_OWNER" --format json -q '.fields[] | select(.name=="Status") | .options[] | select(.name=="Done") | .id')
item=$(gh project item-list "$number" --owner "$REPO_OWNER" --limit 300 --format json -q ".items[] | select(.content.number==$issue) | .id")
if [ -n "$item" ]; then
  gh project item-edit --id "$item" --project-id "$project_id" --field-id "$field_id" --single-select-option-id "$done_id" > /dev/null
  echo "   cartão em Done"
else
  echo "   o issue não está no board" >&2
fi

step "Feito: #$issue${pr:+ (PR #$pr)} em staging."
