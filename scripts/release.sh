#!/usr/bin/env bash
# The one release procedure (docs/DEPLOY.md § 2-3, docs/adr/0011), run by
# .github/workflows/release.yml and by hand alike.
#
#   scripts/release.sh env                       print the release variables (eval it)
#   scripts/release.sh candidate IMAGE SUFFIX    deploy IMAGE as revision connect-doctor-SUFFIX,
#                                                with no traffic, tagged candidate; prints the
#                                                revision and its URL as REVISION= / URL=
#   scripts/release.sh check URL                 DEPLOY.md § 3 against URL; non-zero on any failure
#   scripts/release.sh promote REVISION          send 100 % of traffic to REVISION
#   scripts/release.sh untag                     remove the candidate tag and its URL
#
# ⚠ Every subcommand except env and check acts on the real service.
# ⚠ There is no dry-run mode (.claude/rules/verification.md § An exercise must
#   not change the world).
set -euo pipefail

PROJECT=connect-doctor-511201
REGION=asia-northeast1
REPO=connect-doctor
SERVICE=connect-doctor
SA=connect-doctor-runtime

cmd="${1:-}"; shift || true
case "$cmd" in
env)
  printf 'PROJECT=%s\nREGION=%s\nREPO=%s\nSERVICE=%s\nSA=%s\n' "$PROJECT" "$REGION" "$REPO" "$SERVICE" "$SA"
  echo "IMAGE_REPO=$REGION-docker.pkg.dev/$PROJECT/$REPO/connect-doctor"
  ;;

candidate)
  image="$1"; suffix="$2"
  # ⚠ --max-instances 1 bounds what the service can cost and what it can be made to do.
  # ⚠ --concurrency 8 matches limits.ConcurrentChecks; a 9th request gets 503 server.busy.
  # ⚠ --args=-trust-xff: Cloud Run appends the client address as the last X-Forwarded-For entry
  #   (adr/0008); the per-client limits key on it. ⚠ Never set it where no such proxy sits in front.
  # ⚠ -vantage names where checks leave from on the page's chip; ⚠ keep it equal to $REGION.
  #   (^@^ makes @ the separator, because the value contains a comma.)
  # ⚠ -public-url makes og:url / og:image absolute; ⚠ keep it equal to the custom domain (adr/0009).
  # ⚠ --no-traffic: nobody reaches it until promote (adr/0011). ⚠ The revision name is fixed
  #   here so promote moves traffic to exactly what check checked, never to "latest".
  # ⚠ No --allow-unauthenticated: public access was granted once (DEPLOY.md § 1), and setting
  #   IAM on every deploy would need a role the deployer does not have.
  gcloud run deploy "$SERVICE" --image "$image" --region "$REGION" --project "$PROJECT" \
    --service-account "$SA@$PROJECT.iam.gserviceaccount.com" \
    --max-instances 1 --concurrency 8 --cpu 1 --memory 512Mi --timeout 60 \
    --args='^@^-trust-xff@-vantage=Tokyo, Japan@-public-url=https://connect-doctor.hidetzu.work' \
    --revision-suffix "$suffix" --no-traffic --tag candidate --quiet >&2
  url=$(gcloud run services describe "$SERVICE" --region "$REGION" --project "$PROJECT" --format=json \
    | jq -r '.status.traffic[] | select(.tag == "candidate") | .url')
  echo "REVISION=$SERVICE-$suffix"
  echo "URL=$url"
  ;;

check)
  url="$1"; pass=0; fails=0
  ok()   { echo "ok   $1"; pass=$((pass+1)); }
  bad()  { echo "FAIL $1: $2"; fails=$((fails+1)); }
  # get PATH -> body; on 429 waits Retry-After and tries again (at most 3 times),
  # so the per-client limit spent by an earlier case does not read as a failure.
  get() {
    local i h b
    h=$(mktemp); b=$(mktemp)
    for i in 1 2 3 4; do
      code=$(curl -s -D "$h" -o "$b" -w '%{http_code}' "$url$1")
      [ "$code" != 429 ] && break
      ra=$(awk -F': *' 'tolower($1)=="retry-after"{print $2+0}' "$h"); sleep "${ra:-5}"
    done
    cat "$b"; rm -f "$h" "$b"
  }
  echo "release: check $url"

  # ⚠ First, while this client's burst is still full: the per-client limit keys on the
  #   real client, not a forged first X-Forwarded-For entry.
  codes=""
  for i in 1 2 3 4; do
    codes="$codes$(curl -s -o /dev/null -w '%{http_code}' -H "X-Forwarded-For: 192.0.2.$i" \
      "$url/api/check?url=https://example.com") "
  done
  [ "$codes" = "200 200 200 429 " ] && ok "forged X-Forwarded-For still limited" \
    || bad "forged X-Forwarded-For still limited" "want 200 200 200 429, got $codes"

  body=$(get "/api/check?url=https://example.com")
  [ "$(jq -r .conclusion.status <<<"$body" 2>/dev/null)" = ok ] && ok "example.com is ok" \
    || bad "example.com is ok" "$(head -c 300 <<<"$body")"

  # ⚠ The metadata server is the address Cloud Run adds (adr/0008): refused by address and by name.
  body=$(get "/api/check?url=http://169.254.169.254/")
  [ "$(jq -r .conclusion.code <<<"$body" 2>/dev/null)" = input.refused_address ] && ok "169.254.169.254 refused" \
    || bad "169.254.169.254 refused" "$(head -c 300 <<<"$body")"
  body=$(get "/api/check?url=http://metadata.google.internal/")
  [ "$(jq -r .conclusion.code <<<"$body" 2>/dev/null)" = input.local_name ] && ok "metadata.google.internal refused" \
    || bad "metadata.google.internal refused" "$(head -c 300 <<<"$body")"

  # ⚠ The runtime account must still have no role bindings (adr/0008).
  # ⚠ Could not read the policy ≠ no roles (evidence.md): a failed read is a FAIL with its
  #   error, ⚠ and must not end the script before the count is announced.
  #   stderr is kept apart: a gcloud warning there must not read as a role.
  err=$(mktemp)
  if roles=$(gcloud projects get-iam-policy "$PROJECT" --flatten=bindings --quiet \
      --filter="bindings.members:serviceAccount:$SA@" --format='value(bindings.role)' 2>"$err"); then
    [ -z "$roles" ] && ok "runtime account has no roles" || bad "runtime account has no roles" "$roles"
  else
    bad "runtime account has no roles" "could not read the IAM policy: $(grep ERROR "$err" | head -1)"
  fi
  rm -f "$err"

  echo "release: check — $pass passed, $fails failed"
  [ $fails = 0 ]
  ;;

promote)
  rev="$1"
  gcloud run services update-traffic "$SERVICE" --region "$REGION" --project "$PROJECT" \
    --to-revisions "$rev=100" --quiet >&2
  echo "release: $rev now serves 100 % of traffic"
  ;;

untag)
  # ⚠ A tagged revision answers on its own public URL with its own instance, so its own
  #   in-memory limits (adr/0010) and its own --max-instances 1. ⚠ Left behind after a failed
  #   check, it would be a second, unchecked endpoint until the next release; so it is removed
  #   after every release, whatever the outcome.
  gcloud run services update-traffic "$SERVICE" --region "$REGION" --project "$PROJECT" \
    --remove-tags candidate --quiet >&2
  echo "release: candidate tag removed"
  ;;

*) echo "usage: scripts/release.sh env | candidate IMAGE SUFFIX | check URL | promote REVISION | untag" >&2; exit 2 ;;
esac
