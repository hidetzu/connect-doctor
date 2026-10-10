---
name: verify
description: Decide which of connect-doctor's checks to run, in what order, run them, and return PASS / FAIL / NOT-VERIFIED. Use before a PR, while fixing, and when CI fails. Never builds a new check runner.
---

# Verify

⚠ **The contract is [`rules/verification.md`](../../rules/verification.md).** ⚠ **This file only
names the entry points.** ⚠ **The checks themselves are the tests; this file never copies them.**

## 1. The entry points (⚠ this is all of them)

| Tier | Command | What separates it | ⚠ Leaves the machine? |
|---|---|---|---|
| **Fast** | `scripts/verify.sh fast` | No binary, no sockets except the fake resolver's loopback: gofmt, `go vet` (all tags), `docs-check.mjs`, unit tests incl. the source conformance checks | ⚠ **No** |
| **Final gate** | `scripts/verify.sh final` | ⚠ **Builds the binary from this tree, runs it, talks HTTP to it**, resolving through `internal/dnstest` on loopback, ⚠ **inside an empty network namespace** (`unshare -rn`) with listeners on permitted addresses | ⚠ **No — and cannot: the namespace has no route out**, and `e2e/main_test.go` refuses to run anywhere else |
| **External** | `scripts/verify.sh external` | ⚠ **The other end is a real resolver and real zones.** Records ours beside `getent`; asserts only that our step ran | ⚠ **Yes — DNS, and a TCP handshake to the resolved address (no bytes sent).** ⚠ **Never on a PR** |

**Partial runs** (every tier):

```text
scripts/verify.sh <tier> --only=REGEX   one named case (go test -run)
scripts/verify.sh <tier> --list         count without running — reads the source, compiles nothing
```

⚠ **The first output line names the tier and subset; the last announces the count.**
⚠ **Copy the announced count into the report. Never write it into a document.**

## 2. Order

```text
while fixing     scripts/verify.sh fast --only=<what you touched>   then   scripts/verify.sh fast
before the PR    scripts/verify.sh fast && scripts/verify.sh final
occasionally     scripts/verify.sh external    (⚠ say the date and the machine in the report)
```

## 3. ⚠ Am I measuring what I think I am?

- ⚠ **The final gate builds the binary inside the test, every run.** ⚠ **There is no prebuilt
  artefact to go stale.**
- ⚠ **It listens on `127.0.0.1:0` and reads the chosen port from the binary's first stdout line**
  — ⚠ **another process on a fixed port cannot be measured by mistake.**
- ⚠ **Each test starts its own fake DNS server**; query counts belong to that test only.
- ⚠ **`docs-check.mjs` reads `git ls-files`** — ⚠ **`git add` a new file before trusting its link check.**

## 4. ⚠ Proving a refusal (`rules/security.md` § 6)

⚠ **A refused URL must reach nothing.** ⚠ **The final gate counts queries at the fake DNS server
and accepted connections at listeners on `127.0.0.1:80/443`, and asserts zero for refused URLs,
⚠ after a control that reached the public listener.**

⚠ **The namespace** (`e2e/main_test.go`): only `lo` and no default route, checked first; then
`93.184.215.14` listening, `93.184.215.15` closed, `8.8.4.4` routed into a dummy interface
(silence), `9.9.9.9` an `unreachable` route, no IPv6 route. ⚠ **Documentation ranges are refused by
policy, so they cannot stand in for a target.**
⚠ **Needs unprivileged user namespaces.** ⚠ **Where `unshare -rn` fails, the runner says
NOT-VERIFIED and exits non-zero** — ⚠ **never a skipped PASS.**

## 5. Splitting a failure

| Failed | ⚠ Ours? |
|---|---|
| fast | ⚠ **Ours.** Nothing leaves the machine |
| final | ⚠ **Ours**, ⚠ unless `go build` itself could not run (toolchain) — say which |
| external | ⚠ **Probably theirs.** A resolver or a zone changed. ⚠ **Still FAIL; not evidence our code broke.** Rerun once |

## 6. What CI runs

`.github/workflows/check.yml`, on every PR and on pushes to `main`:

| Tier | CI | ⚠ Why |
|---|---|---|
| fast | runs | — |
| final gate | runs | ⚠ **Needs an unprivileged network namespace** — see below |
| external | ⚠ **never** | ⚠ **Depends on third parties' uptime.** ⚠ **Every run's summary says so** |

⚠ **Whether the hosted runner can create the namespace is measured on every run** (the workflow's
first step prints it, and the run summary records it).

⚠ **First measured 2026-10-10, GitHub-hosted `ubuntu-24.04` (hidetzu/connect-doctor#19):
`apparmor_restrict_unprivileged_userns=1`, so `unshare -rn` failed as is; after
`sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0` it worked, and the final gate ran
inside the namespace.** ⚠ **The workflow lifts that restriction on the runner only — it says so in
every run summary.**

## 7. Return

The block in [`rules/verification.md`](../../rules/verification.md) § What to return. ⚠ **No other shape.**
