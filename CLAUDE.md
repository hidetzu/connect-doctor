# connect-doctor — how we work

This file holds **how to work**.
⚠ **What ConnectDoctor is, and what it refuses to become, is [`docs/PRODUCT.md`](docs/PRODUCT.md)
— the single source.** ⚠ **Never restate the product here.**
**What may be claimed** goes in `docs/SPEC.md`; **why a decision was made** goes in `docs/adr/`.
⚠ **How to write code** (the language, the layer split, testing priorities, forbidden git
operations) lives in `.claude/rules/`.

⚠ **Never duplicate.** ⚠ **Written in two places, one of them goes stale.**
When the spec changes, fix the spec. Do not restate it here.

Before starting work, read `.claude/rules/README.md`.

⚠ **`⚠` marks "it hurts if you step on it".** ⚠ It is not decoration.

---

## 0. What this repository is

ConnectDoctor is a web service and JSON API that takes one URL and answers "why does this not
connect?" by checking DNS → TCP → TLS → HTTP from our server, naming the first layer that failed.
⚠ **What it is, is [`docs/PRODUCT.md`](docs/PRODUCT.md).** ⚠ **How it is built against hostile
input is [`docs/DESIGN.md`](docs/DESIGN.md) and [`.claude/rules/security.md`](.claude/rules/security.md).**

⚠ **`.claude/` is a port of [`hidetzu/claude-dev-template`](https://github.com/hidetzu/claude-dev-template).**
⚠ **When this project has to fight the template to do the right thing, the template is wrong** —
the fix goes back there naming this project.

---

## 1. The first principle

> **Refusing to connect before connecting. ⚠ The one answer before many features.**

⚠ **The order is never swapped.** ⚠ **A check that reaches somewhere it should not is worse than
a check that does not run** ([`.claude/rules/security.md`](.claude/rules/security.md)).
⚠ **A feature that answers a question other than "why does this not connect?" is not added,
however cheap** ([`docs/PRODUCT.md`](docs/PRODUCT.md) § 5).

⚠ **Whatever it says, it never outranks the evidence rules.**
⚠ **Those are [`.claude/rules/evidence.md`](.claude/rules/evidence.md), and they hold everywhere** —
in the code, in the tests, and in every report.

⚠ **Never restate them here.** ⚠ **They are the one thing this template does not let a project
re-word**, because re-wording them is how they get softened.

---

## 2. Verification

⚠ **The contract is [`.claude/rules/verification.md`](.claude/rules/verification.md).**
⚠ **What to actually run is `.claude/skills/verify/SKILL.md`, which each project writes for itself.**
⚠ Neither belongs here (never two copies).

---

## 3. Architecture boundaries

⚠ **One Go binary.** ⚠ **Every address decision is `internal/policy`; every dial goes through
`internal/dial`; every limit is `internal/limits`; every human sentence is `internal/diag/words.go`.**
⚠ **The page and the API render the same `diag.Result`** ([`docs/DESIGN.md`](docs/DESIGN.md) § 5).
⚠ **Each of those is settled in [`docs/adr/`](docs/adr/).**

⚠ **Not to be introduced without a reason recorded in an ADR:**

```text
any third-party Go module (⚠ including golang.org/x/...)
a JavaScript framework, a frontend build, a second runtime
a database, or anything that remembers a check after it returns
an HTTP client that resolves or dials on its own (http.Get, DefaultTransport, ...)
a port other than 80 and 443
a second vantage point (⚠ PRODUCT § 5)
```

Two clauses hold regardless of the domain:

- ⚠ **Never keep two implementations that answer the same question.**
  If one is unavoidable, cross-check them mechanically.
  ⚠ **Writing the same decision in two places is how the two silently diverge.**
- ⚠ **A layer split belongs in an ADR before it belongs in code.**

---

## 4. Words

- **Never leak internal state into what a human reads.** Not an error code, but a sentence
  that says what happened and what to do about it.
- **Name things after the concept the domain already named, not after the data structure.**
  ⚠ **Borrow the existing name exactly.** ⚠ If a name here differs, that difference is a claim —
  justify it.
- **Never rename in bulk.** Changing a term does not license a sweep through the ADRs and past
  discussions.

⚠ **Layer names are the protocols' own**: DNS, TCP, TLS, HTTP — not "network", "connection",
"security". ⚠ **Error names borrow the protocol's word**: NXDOMAIN (RFC 1035 § 4.1.1, RCODE 3),
connection refused (RST, RFC 9293), certificate expired (RFC 5280 § 4.1.2.5).

⚠ **Marks already spoken for**: ✅ the step completed; ❌ the step failed; ⛔ ConnectDoctor refused
to perform it (⚠ **ours, never the target's**); `—` not attempted. ⚠ **Never reuse one for
another meaning.**

⚠ **The status and code vocabulary is [`docs/DESIGN.md`](docs/DESIGN.md) § 1–2.** ⚠ **A new code is
a spec change** (`.claude/rules/owner-decisions.md`).

⚠ **UI and conclusion sentences are Japanese** (the owner's brief). ⚠ **Documents, code, commits
and issues are English** (a public repository, and the template's language).

### 4-1. Never open with what does not work

⚠ **This is not about hiding anything.** §1 outranks it, and **limitations are always stated**.
What changes is the **order, the subject, and the tense** — not whether it is said.

- **Say what this does first.** What it does not do comes after, with the reason
  and with what to do instead.
- **Do not use the progressive tense for a state.** "not receiving" reads as something
  happening right now on the reader's machine.
- **Never phrase our own gap as the other side's fault.** If we never implemented it,
  do not report it as "no response". ⚠ **The reader's next move depends on which it is** —
  retry, wait, or give up.
- **Do not sound stalled.** "not implemented yet" beats "unavailable": leave a reason to come back.

---

## 5. Comments

Comments carry **why this, why this value, what is being avoided**. That is an asset.
⚠ **But a stale comment misleads harder than stale code**, because it is believed.

Change code, and update the whole set:

```
implementation → test → comment → README → docs/SPEC.md
```

⚠ **When a check reads documentation or comments, strip the comments first.**
⚠ Otherwise the check picks up the very words written to describe it.

---

## 6. How to write numbers

⚠ **Owned by [`.claude/rules/evidence.md`](.claude/rules/evidence.md).** ⚠ Not here.

---

## 7. How to proceed

1. **Measure before polishing.** Before fixing anything, state what it does now, in numbers.
2. Report **observation** (measured values, captured output) and **inference** (interpretation)
   separately.
3. Never report as confirmed what was not verified.
4. Do not widen a change past its `Non-goals`. The smallest change that meets the goal is the default.

⚠ **Deciding yourself vs. asking, and `ready-for-ai`, are owned by
[`.claude/rules/owner-decisions.md`](.claude/rules/owner-decisions.md).** ⚠ Not here.

---

## 8. git

⚠ **Owned by [`.claude/rules/git.md`](.claude/rules/git.md)** — Conventional Commits, permission
for `git push` and merge, how to split commits, and what never goes into anything public.
⚠ Not here.

---

## 9. Pitfalls we have stepped on

⚠ **This table starts empty, and that is correct.**

⚠ **Nothing goes in here that did not happen in this repository.** Not an analogy from another
project, not something plausible, not something an AI expects to be true.
⚠ **A pitfall is a measurement**: it names what happened, and what to do instead.

⚠ **This is not where engineering constraints go.** A rule that holds because of the language,
because of a protocol, or because the input is hostile ⚠ **belongs in `.claude/rules/`, and binds
already.**
⚠ **Never manufacture an incident to move a constraint in here**, and
⚠ **never soften a constraint on the grounds that this table has no row for it yet.**

⚠ **When you fill a row in, also leave the test behind.** A row with no test is a note;
a row with a test is a wall.
⚠ **If the incident also produces a new rule, the rule goes to `.claude/rules/` citing this row.**
⚠ **Both records stay. Neither replaces the other.**

| What happened | What to do instead |
|---|---|
| — | — |
