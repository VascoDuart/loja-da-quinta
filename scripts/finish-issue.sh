#!/usr/bin/env bash
# Finishes an issue end to end, with no manual steps:
#
#   1. if the PR branch is behind dev, updates it with dev;
#   2. waits for the PR's required checks (ci-ok and security-ok);
#   3. squash-merges into dev with "Closes #N" and deletes the branch
#      (if dev moved in the meantime, updates the branch and tries again);
#   4. waits for that commit's deploy-staging run and summarises the E2E tests;
#   5. if the deploy passed, closes the issue (linking the run) and moves its
#      GitHub Projects card to Done.
#
# Usage:
#   scripts/finish-issue.sh --pr 35 --issue 10   # everything
#   scripts/finish-issue.sh --pr 35              # merge and deploy (no issue)
#   scripts/finish-issue.sh --issue 10           # only wait for dev's deploy and close
#
# If the deploy fails, it stops before closing the issue and prints the run.
set -euo pipefail

REPO_OWNER="VascoDuart"
PROJECT_TITLE="Loja da Quinta"
WORKFLOW="deploy-staging.yml"
# Checks required by the dev and main branch protection (see CONTRIBUTING.md).
REQUIRED_CHECKS="ci-ok security-ok"

pr="" issue=""
while [ $# -gt 0 ]; do
  case "$1" in
    --pr) pr="$2"; shift 2 ;;
    --issue) issue="$2"; shift 2 ;;
    -h|--help) sed -n '2,18p' "$0"; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done
[ -n "$pr$issue" ] || { sed -n '2,18p' "$0"; exit 2; }

step() { printf '\n== %s\n' "$*"; }
cd "$(git rev-parse --show-toplevel)"

# Retries a gh command on transient network errors (e.g. "error connecting to
# api.github.com"), which otherwise abort a 10-minute wait halfway through.
retry() {
  local attempt
  for attempt in 1 2 3 4 5; do
    "$@" && return 0
    [ "$attempt" = 5 ] && return 1
    sleep 15
  done
}

# Waits until every required check exists and passes. Checks are matched by
# name: one that has not started yet is missing from the list, and "nothing
# pending" does not mean "everything passed".
wait_for_checks() {
  local states missing failed check bucket
  for _ in $(seq 1 120); do  # up to 40 minutes
    states=$(gh pr checks "$pr" --json name,bucket -q '.[] | "\(.name)=\(.bucket)"' 2>/dev/null || true)
    missing=0 failed=""
    for check in $REQUIRED_CHECKS; do
      # || true: a check that does not exist yet must not end the script (pipefail).
      bucket=$( (grep -E "^$check=" <<<"$states" || true) | head -1 | cut -d= -f2)
      case "$bucket" in
        pass) ;;
        fail|cancel) failed="$failed $check" ;;
        *) missing=1 ;;
      esac
    done
    if [ -n "$failed" ]; then
      echo "Failing checks:$failed. Not merging." >&2
      gh pr checks "$pr" >&2 || true
      exit 1
    fi
    [ "$missing" = 0 ] && { for check in $REQUIRED_CHECKS; do echo "   $check: pass"; done; return 0; }
    sleep 20
  done
  echo "Checks did not finish in time." >&2
  exit 1
}

# Updates the PR branch with dev when it is behind (branch protection only
# merges up-to-date branches). Returns 0 if it updated, 1 if nothing to do.
update_if_behind() {
  if [ "$(gh pr view "$pr" --json mergeStateStatus -q .mergeStateStatus)" = "BEHIND" ]; then
    echo "   branch is behind dev: updating it"
    gh pr update-branch "$pr" > /dev/null
    sleep 10  # let GitHub register the new head before polling checks
    return 0
  fi
  return 1
}

# --- 1 to 3: checks and merge ------------------------------------------------
if [ -n "$pr" ]; then
  title=$(gh pr view "$pr" --json title -q .title)
  body=""
  [ -n "$issue" ] && body="Closes #$issue"
  merged=0
  for _ in 1 2 3; do
    update_if_behind || true
    step "PR #$pr: waiting for required checks ($REQUIRED_CHECKS)"
    wait_for_checks
    step "Squash merge: $title (#$pr)"
    if gh pr merge "$pr" --squash --delete-branch --subject "$title (#$pr)" --body "$body" > /dev/null 2>&1; then
      merged=1
      break
    fi
    # Usually dev moved while the checks ran; update and go round again.
    update_if_behind || { echo "Merge refused and the branch is not behind dev:" >&2; gh pr view "$pr" >&2; exit 1; }
  done
  [ "$merged" = 1 ] || { echo "Could not merge PR #$pr after 3 attempts." >&2; exit 1; }
fi

git checkout -q dev
git pull -q --ff-only
sha=$(git rev-parse HEAD)
echo "   dev at ${sha:0:7}: $(git log -1 --format=%s)"

# --- 4: staging deploy -------------------------------------------------------
step "Waiting for deploy-staging of ${sha:0:7}"
run=""
for _ in $(seq 1 24); do  # the run can take a few seconds to appear
  run=$(gh run list --workflow "$WORKFLOW" --commit "$sha" --limit 1 --json databaseId -q '.[0].databaseId // empty' 2>/dev/null || true)
  [ -n "$run" ] && break
  sleep 5
done
[ -n "$run" ] || { echo "No deploy-staging run found for $sha." >&2; exit 1; }
url=$(retry gh run view "$run" --json url -q .url)
echo "   $url"

# gh run watch can drop on a network error while the run keeps going on
# GitHub: watch again until the run itself reports that it finished.
for _ in 1 2 3 4 5; do
  gh run watch "$run" --interval 20 > /dev/null 2>&1 || true
  [ "$(retry gh run view "$run" --json status -q .status)" = "completed" ] && break
  sleep 15
done
conclusion=$(retry gh run view "$run" --json conclusion -q .conclusion)
retry gh run view "$run" --json jobs -q '.jobs[] | "   \(.name): \(.conclusion)"'

e2e=$(retry gh run view "$run" --json jobs -q '.jobs[] | select(.name=="e2e") | .databaseId')
summary=""
if [ -n "$e2e" ]; then
  # Playwright's closing lines: "N passed", "N flaky", "N failed", "N skipped".
  summary=$(gh run view --job "$e2e" --log 2>/dev/null \
    | grep -oE '[[:space:]][0-9]+ (passed|failed|flaky|skipped)([[:space:]]|$)' \
    | awk '{print $1, $2}' | paste -sd ',' - | sed 's/,/, /g')
  echo "   E2E: ${summary:-no summary}"
  if grep -q 'flaky' <<<"$summary"; then
    echo "   Warning: there are flaky tests. A flaky test usually hides a bug; check the run log." >&2
  fi
fi

if [ "$conclusion" != "success" ]; then
  echo "deploy-staging ended as '$conclusion': the issue stays open. See $url" >&2
  exit 1
fi

# --- 5: close the issue and move the card ------------------------------------
[ -n "$issue" ] || { step "Done (no issue to close)."; exit 0; }

step "Issue #$issue: closing and moving the card to Done"
if [ "$(gh issue view "$issue" --json state -q .state)" = "OPEN" ]; then
  # Issues are written in Portuguese, like the rest of the project's issues.
  gh issue close "$issue" --comment "Concluído${pr:+ no PR #$pr}. deploy-staging verde${summary:+ (E2E: $summary)}: $url" > /dev/null
  echo "   closed"
else
  echo "   was already closed"
fi

number=$(gh project list --owner "$REPO_OWNER" --format json -q ".projects[] | select(.title==\"$PROJECT_TITLE\") | .number")
project_id=$(gh project view "$number" --owner "$REPO_OWNER" --format json -q .id)
field_id=$(gh project field-list "$number" --owner "$REPO_OWNER" --format json -q '.fields[] | select(.name=="Status") | .id')
done_id=$(gh project field-list "$number" --owner "$REPO_OWNER" --format json -q '.fields[] | select(.name=="Status") | .options[] | select(.name=="Done") | .id')
item=$(gh project item-list "$number" --owner "$REPO_OWNER" --limit 300 --format json -q ".items[] | select(.content.number==$issue) | .id")
if [ -n "$item" ]; then
  gh project item-edit --id "$item" --project-id "$project_id" --field-id "$field_id" --single-select-option-id "$done_id" > /dev/null
  echo "   card in Done"
else
  echo "   the issue is not on the board" >&2
fi

step "Done: #$issue${pr:+ (PR #$pr)} is in staging."
