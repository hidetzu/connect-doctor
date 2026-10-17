#!/usr/bin/env bash
# The entry point for every tier (.claude/skills/verify/SKILL.md).
#
#   scripts/verify.sh fast     [--only=REGEX] [--list]
#   scripts/verify.sh final    [--only=REGEX] [--list]
#   scripts/verify.sh external [--list]
#   scripts/verify.sh image    [--only=REGEX] [--list]
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
  image)    ;;
  *) echo "usage: scripts/verify.sh fast|final|external|image [--only=REGEX] [--list]" >&2; exit 2 ;;
esac

echo "verify: tier=$tier subset=${only:-all}$([ $list = 1 ] && echo ' (list only, nothing runs)')"

# ⚠ The image tier checks what Cloud Run runs: the Dockerfile, built from this
#   tree, started the way Cloud Run starts it (PORT set, no -addr). It is not
#   go test: the other tiers build with the Go in go.mod, ⚠ the image builds
#   with the Go in the Dockerfile (hidetzu/connect-doctor#58, #59).
# ⚠ The container sits on an --internal network: no route out, so a case
#   that would reach the internet fails instead of reaching it.
image_cases=(image_serves_landing_page image_refuses_metadata_address image_carries_ca_certificates)
if [ "$tier" = image ]; then
  cases=$(printf '%s\n' "${image_cases[@]}" | { if [ -n "$only" ]; then grep -E "$only"; else cat; fi; })
  if [ $list = 1 ]; then
    printf '%s\n' "$cases" | grep . | sed 's|^|  Dockerfile: |'
    echo "verify: $(printf '%s' "$cases" | grep -c . ) image cases in tier=image subset=${only:-all}"
    exit 0
  fi
  if ! docker info >/dev/null 2>&1; then
    echo "verify: NOT-VERIFIED — no usable Docker daemon here (docker info failed)"
    exit 3
  fi
  run=cd-verify-$$
  # ⚠ The image itself is kept: its ID can be the very image a release tag
  #   names (same tree, same layers), and removing it would untag nothing safely.
  img=""; ctr=""
  cleanup() {
    [ -n "$ctr" ] && docker rm -f "$ctr" >/dev/null 2>&1
    docker network rm "$run" >/dev/null 2>&1
  }
  trap cleanup EXIT
  # ⚠ Run the image by the ID this build printed, never by a tag:
  #   a tag can still name an image from an earlier run.
  out=$(docker build -q . 2>&1); img=$(printf '%s\n' "$out" | tail -1)
  if [[ "$img" != sha256:* ]]; then
    echo "FAIL docker build"; printf '%s\n' "$out" | grep -v -i 'buildx\|legacy builder' | tail -15; img=""
    echo "verify: tier=image subset=${only:-all} — 0 passed, 1 failed, 0 skipped (image cases)"
    echo "verify: FAIL"; exit 1
  fi
  echo "verify: built $img from this tree"
  docker network create --internal "$run" >/dev/null
  ctr=$(docker run -d --network "$run" -e PORT=8080 "$img")
  ip=$(docker inspect -f "{{(index .NetworkSettings.Networks \"$run\").IPAddress}}" "$ctr")
  base="http://$ip:8080"
  for _ in $(seq 50); do curl -s -o /dev/null "$base/" && break; sleep 0.1; done

  image_serves_landing_page() {
    curl -sf "$base/" | grep '<title>ConnectDoctor</title>' >/dev/null
  }
  # ⚠ grep reads to the end (no -q): with pipefail, -q closing the pipe early
  #   fails the producer with SIGPIPE and the case with it.
  # ⚠ The metadata server is the address Cloud Run adds (docs/adr/0008).
  image_refuses_metadata_address() {
    curl -s "$base/api/check?url=http://169.254.169.254/" | grep '"code": *"input.refused_address"' >/dev/null
  }
  # ⚠ The TLS step trusts the system root store only (docs/DESIGN.md § 2):
  #   an image without it fails every https check, and nothing else shows it.
  image_carries_ca_certificates() {
    docker cp "$ctr:/etc/ssl/certs/ca-certificates.crt" - 2>/dev/null | tar -xO | grep 'BEGIN CERTIFICATE' >/dev/null
  }

  pass=0; fails=0
  for c in $cases; do
    if $c; then echo "ok   $c"; pass=$((pass+1)); else echo "FAIL $c"; fails=$((fails+1)); fi
  done
  [ $fails = 0 ] || { echo "--- container output"; docker logs "$ctr" 2>&1 | tail -20; }
  echo "verify: tier=image subset=${only:-all} — $pass passed, $fails failed, 0 skipped (image cases)"
  [ $fails = 0 ] && echo "verify: PASS" || echo "verify: FAIL"
  exit $([ $fails = 0 ] && echo 0 || echo 1)
fi

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
  if scripts/review.sh selftest; then :; else fail=1; fi
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
