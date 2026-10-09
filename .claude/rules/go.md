# Go

⚠ **`MUST` = required, `SHOULD` = default, `MAY` = optional.**

## Grounds

⚠ **The product is a fetcher of hostile URLs** ([`security.md`](security.md)).
⚠ **Every line of third-party code runs next to it**, and every hidden resolve or dial is a hole.

## Dependencies

- MUST: ⚠ **Standard library only**, ⚠ **including no `golang.org/x/...`**
  ([`adr/0006`](../../docs/adr/0006-one-go-binary-standard-library-only-no-frontend-build.md)).
  ⚠ **`go.mod` has no `require` block.** ⚠ **Adding one is an ADR first.**

## Errors become outcomes in one place

- MUST: ⚠ **A step classifies its own error into an outcome code** (`docs/DESIGN.md` § 2), ⚠ **in
  the step's file, once.** ⚠ **Callers never re-inspect the raw error.**
- MUST: ⚠ **An error that matches no known outcome is reported as the step's generic failure
  code, with the raw text kept in `detail.error`** — ⚠ **never silently mapped to the nearest
  known one.** ⚠ **A guess dressed as a classification is a guess dressed as a measurement**
  ([`evidence.md`](evidence.md)).
- MUST: ⚠ **Use `errors.As` / `errors.Is`.** ⚠ **Never match on an error's message string**
  except where the standard library offers nothing else, ⚠ **and then say so in a comment.**

## Where things live

- MUST: ⚠ **Every timeout and cap: `internal/limits`** ([`security.md`](security.md) § 4).
- MUST: ⚠ **Every sentence a human reads: `internal/diag/words.go`** (`CLAUDE.md` § 4).
  ⚠ **Templates and handlers render; they do not compose conclusions.**
- MUST: ⚠ **Every step takes a `context.Context` and honours it.**

## Tests

- SHOULD: ⚠ **Table tests for pure functions** (`internal/target`, `internal/policy`).
- MUST: ⚠ **A step that touches the network takes its dependency as an interface or a function
  value**, so the fast tier can run it with a fake. ⚠ **The fake never replaces the policy.**
- MUST: ⚠ **`gofmt` clean, `go vet` clean** — the fast tier runs both.
