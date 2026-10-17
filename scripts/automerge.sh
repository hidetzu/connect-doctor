#!/usr/bin/env bash
# Dependabot pull requests merge themselves, then start the release
# (docs/adr/0011, hidetzu/connect-doctor#62). Run by .github/workflows/automerge.yml.
#
#   scripts/automerge.sh run SHA          the checks on SHA finished: merge its PR if decide says so,
#                                         then dispatch release.yml and check.yml on main
#   scripts/automerge.sh decide DIR SHA   read DIR/{pr,commits,checks,branch}.json; print "merge" or
#                                         "skip: <reason>"; exit 0 to merge, 1 to skip
#   scripts/automerge.sh selftest         decide every case in scripts/automerge-fixtures/ (no network)
#
# ⚠ Nothing from the pull request is checked out or run. Every fact comes from
#   the GitHub API, never from the workflow_run payload's actor.
# ⚠ The required checks are read from branch protection, not listed here: a
#   second list would drift from the one GitHub enforces.
# ⚠ The merge names the head it decided on (--match-head-commit): a push that
#   lands after the checks is not merged with their verdict.
# ⚠ A merge made with GITHUB_TOKEN starts no push run, so the release is
#   dispatched explicitly — never --auto, never a personal token (adr/0011).
set -euo pipefail
cd "$(dirname "$0")/.."

BOT=dependabot[bot]

# decide: every condition is its own skip reason, so a skipped PR says why.
decide() {
  local d="$1" sha="$2" r
  r=$(jq -r --arg bot "$BOT" --arg sha "$sha" '
    if .state != "open" or .merged == true then "skip: not open"
    elif .base.ref != "main" then "skip: base is \(.base.ref), not main"
    elif .user.login != $bot or .user.type != "Bot" then "skip: opened by \(.user.login) (\(.user.type)), not the \($bot) app"
    elif .head.repo.full_name != .base.repo.full_name then "skip: head is in another repository"
    elif (.head.ref | startswith("dependabot/") | not) then "skip: head branch \(.head.ref) is not dependabot/*"
    elif .head.sha != $sha then "skip: head moved to \(.head.sha[0:7]) after the checks ran on \($sha[0:7])"
    else "ok" end' "$d/pr.json")
  [ "$r" = ok ] || { echo "$r"; return 1; }

  # ⚠ author.login comes from the commit's author email, which whoever pushes can
  #   set: this catches the owner's own commit on a Dependabot branch, it is not
  #   proof against someone who can push. Only the owner can push here.
  r=$(jq -r --arg bot "$BOT" '
    [.[] | (.author.login // "unknown")] | map(select(. != $bot)) |
    if length == 0 then "ok" else "skip: commits by \(unique | join(", ")) as well as \($bot)" end' "$d/commits.json")
  [ "$r" = ok ] || { echo "$r"; return 1; }

  r=$(jq -r --slurpfile branch "$d/branch.json" '
    ($branch[0].protection.required_status_checks.checks // []) as $req |
    .check_runs as $runs |
    if ($req | length) == 0 then "skip: no required checks could be read from branch protection"
    else
      ([$req[] | . as $q |
        ([$runs[] | select(.name == $q.context and .app.id == $q.app_id)] | sort_by(.started_at) | last) as $run |
        if $run == null then "\($q.context) has not reported"
        elif $run.status != "completed" then "\($q.context) is \($run.status)"
        elif $run.conclusion != "success" then "\($q.context) concluded \($run.conclusion)"
        else empty end]) as $missing |
      ([$runs[] | select(.status == "completed" and (.conclusion | IN("success","skipped","neutral") | not)) | "\(.name) concluded \(.conclusion)"]) as $failed |
      if ($missing | length) > 0 then "skip: \($missing | join("; "))"
      elif ($failed | length) > 0 then "skip: \($failed | join("; "))"
      else "ok" end
    end' "$d/checks.json")
  [ "$r" = ok ] || { echo "$r"; return 1; }

  echo merge
}

cmd="${1:-}"; shift || true
case "$cmd" in
decide)
  decide "$1" "$2"
  ;;

selftest)
  # ⚠ Each case must print exactly the expected line (scripts/automerge-fixtures/expect.tsv):
  #   the decision and its reason, not a substring of them.
  pass=0; fails=0; seen=0
  while IFS=$'\t' read -r c sha want; do
    case "$c" in ''|'#'*) continue ;; esac
    seen=$((seen+1))
    out=$(decide "scripts/automerge-fixtures/$c" "$sha" || true)
    if [ "$out" = "$want" ]; then pass=$((pass+1))
    else echo "FAIL automerge selftest: $c printed \"$out\", want \"$want\""; fails=$((fails+1)); fi
  done < scripts/automerge-fixtures/expect.tsv
  n=$(find scripts/automerge-fixtures -mindepth 1 -maxdepth 1 -type d | wc -l)  # README is not a case
  [ "$n" = "$seen" ] || { echo "FAIL automerge selftest: $n cases on disk, $seen in expect.tsv"; fails=$((fails+1)); }
  echo "automerge selftest: $pass passed, $fails failed"
  [ $fails = 0 ]
  ;;

run)
  sha="$1"
  d=$(mktemp -d); trap 'rm -rf "$d"' EXIT
  pr=$(gh api "repos/{owner}/{repo}/commits/$sha/pulls" -q "[.[] | select(.state == \"open\")] | first | .number // empty")
  if [ -z "$pr" ]; then echo "automerge: skip: no open pull request has head ${sha:0:7}"; exit 0; fi
  gh api "repos/{owner}/{repo}/pulls/$pr" > "$d/pr.json"
  gh api "repos/{owner}/{repo}/pulls/$pr/commits" --paginate > "$d/commits.json"
  gh api "repos/{owner}/{repo}/commits/$sha/check-runs?per_page=100" > "$d/checks.json"
  gh api "repos/{owner}/{repo}/branches/main" > "$d/branch.json"
  if ! verdict=$(decide "$d" "$sha"); then
    echo "automerge: PR $pr: $verdict"; exit 0
  fi
  echo "automerge: PR $pr at ${sha:0:7}: merging"
  # Branch deletion is the repository setting (delete_branch_on_merge), not this script.
  gh pr merge "$pr" --squash --match-head-commit "$sha"
  gh workflow run release.yml --ref main
  # ⚠ check.yml too, so the README badge reports this commit, not the one before it.
  gh workflow run check.yml --ref main
  echo "automerge: PR $pr merged; release.yml and check.yml dispatched on main"
  ;;

*) echo "usage: scripts/automerge.sh run SHA | decide DIR SHA | selftest" >&2; exit 2 ;;
esac
