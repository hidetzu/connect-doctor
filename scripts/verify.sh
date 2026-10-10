#!/usr/bin/env bash
# The entry point for every tier (.claude/skills/verify/SKILL.md).
#
#   scripts/verify.sh fast     [--only=REGEX] [--list]
#   scripts/verify.sh final    [--only=REGEX] [--list]
#   scripts/verify.sh external [--list]
#
# ⚠ The first line of output names the tier and the subset.
# ⚠ The last line announces the count. ⚠ Copy it into reports; never write
#   it into a document (.claude/rules/evidence.md § Counts).
# ⚠ --list counts without running: it reads test names from the source and
#   compiles nothing.
set -uo pipefail
cd "$(dirname "$0")/.."

tier="${1:-}"; shift || true
only=""; list=0
for a in "$@"; do
  case "$a" in
    --only=*) only="${a#--only=}" ;;
    --list) list=1 ;;
    *) echo "verify: unknown argument $a" >&2; exit 2 ;;
  esac
done

case "$tier" in
  fast)     tags="";         pkgs="./..." ;  dirs="internal cmd" ;;
  final)    tags="e2e";      pkgs="./e2e/";  dirs="e2e" ;;
  external) tags="external"; pkgs="./e2e/";  dirs="e2e" ;;
  *) echo "usage: scripts/verify.sh fast|final|external [--only=REGEX] [--list]" >&2; exit 2 ;;
esac

echo "verify: tier=$tier subset=${only:-all}$([ $list = 1 ] && echo ' (list only, nothing runs)')"

list_tests() {
  # Test functions in files of this tier's build tag (untagged for fast).
  for f in $(find $dirs -name '*_test.go' 2>/dev/null | sort); do
    first=$(grep -m1 '^//go:build' "$f" | sed 's|//go:build ||')
    if [ "${first}" != "${tags}" ]; then continue; fi
    grep -oE '^func (Test[A-Za-z0-9_]+)' "$f" | sed "s|func ||;s|^|  $f: |"
  done | { if [ -n "$only" ]; then grep -E "$only"; else cat; fi; }
}

if [ $list = 1 ]; then
  out=$(list_tests); echo "$out"
  echo "verify: $(printf '%s' "$out" | grep -c . ) test functions in tier=$tier subset=${only:-all}"
  exit 0
fi

fail=0
if [ "$tier" = fast ]; then
  bad=$(gofmt -l . ); if [ -n "$bad" ]; then echo "FAIL gofmt: $bad"; fail=1; else echo "ok   gofmt"; fi
  if go vet ./... && go vet -tags e2e ./... && go vet -tags external ./...; then echo "ok   go vet"; else echo "FAIL go vet"; fail=1; fi
  if node .claude/tools/docs-check.mjs; then :; else fail=1; fi
fi

args=(-count=1 -v)
[ -n "$tags" ] && args+=(-tags "$tags")
[ -n "$only" ] && args+=(-run "$only")
log=$(mktemp); trap 'rm -f "$log"' EXIT

# ⚠ The final gate makes real TCP connections, so it runs only inside an
#   empty network namespace with no route out (e2e/main_test.go refuses
#   anything else). ⚠ If one cannot be created, that is NOT-VERIFIED, not PASS.
wrap=()
if [ "$tier" = final ]; then
  if ! unshare -rn true 2>/dev/null; then
    echo "verify: NOT-VERIFIED — cannot create an unprivileged network namespace here (unshare -rn failed)"
    exit 3
  fi
  wrap=(unshare -rn)
  echo "verify: final gate runs inside an empty network namespace (unshare -rn)"
fi
"${wrap[@]}" go test "${args[@]}" $pkgs 2>&1 | tee "$log" | grep -vE '^(=== (RUN|PAUSE|CONT)|\s+--- PASS)'
[ "${PIPESTATUS[0]}" = 0 ] || fail=1

pass=$(grep -cE '^\s*--- PASS' "$log"); fails=$(grep -cE '^\s*--- FAIL' "$log"); skips=$(grep -cE '^\s*--- SKIP' "$log")
echo "verify: tier=$tier subset=${only:-all} — $pass passed, $fails failed, $skips skipped (tests and subtests)"
[ $fail = 0 ] && echo "verify: PASS" || echo "verify: FAIL"
exit $fail
