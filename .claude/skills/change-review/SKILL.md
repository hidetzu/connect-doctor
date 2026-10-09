---
name: change-review
description: Review a change (git diff) against the issue's scope and the project rules, and return PASS / NEEDS-FIX / HUMAN-DECISION. Use before opening a PR, and when the Loop Controller reaches its review stage.
---

# Change / Scope Review

⚠ **Read `git diff` yourself.** ⚠ Never write from a summary or from memory.

⚠ **Running the checks is not this skill.** That is `.claude/skills/verify/SKILL.md`.
What this looks at is **whether the change itself sits inside what was asked and inside the rules**.

⚠ **Never invent a standard here.** [`CLAUDE.md`](../../../CLAUDE.md),
[`.claude/rules/`](../../rules/), [`docs/SPEC.md`](../../../docs/SPEC.md),
[`docs/adr/`](../../../docs/adr/) and the issue's Owner Decisions are authoritative.

---

## 1. What to read

```bash
git diff <default>...HEAD          # the change itself
git diff --stat <default>...HEAD   # where it moved
git log --oneline <default>..HEAD  # is it split by reason
```

⚠ **Never judge from `--stat` alone.** Read the contents.

---

## 2. Scope

| What to check | ⚠ How it fails |
|---|---|
| **Acceptance Criteria** | ⚠ **One at a time**, each with what confirmed it. ⚠ "Probably satisfied" is not satisfied |
| **Out of Scope / Non-goals** | Files outside it have moved |
| **One PR, one reason** | ⚠ Other work got swept in (the `git add -A` accident) |
| **Unrequested features** | Something not in the issue has appeared |
| **Incidental refactors** | Rewrites unrelated to the goal |

---

## 3. The rules

⚠ **These outrank scope.** ⚠ Inside scope but touching one of these is still `NEEDS-FIX`.

| What to check | ⚠ How it fails |
|---|---|
| **not observed ≠ did not happen** | Something that was merely not seen is reported as absent |
| **could not be obtained ≠ not there** | A failure to read gets reported as "there is nothing" |
| **the test passed ≠ it is correct** | A check that would pass with the behaviour removed |
| **it answered ≠ for the right reason** | An answer asserted, but not the thing that made it right |
| **never dress a guess as a measurement** | Probabilities, confidence figures, numbers nobody measured |
| **denominators** | A number with no scope, date, or conditions. ⚠ Numbers from different scopes side by side |
| **counts written down** | ⚠ A count put into a document instead of announced by whatever produced it |
| **claims against an authority** | A behaviour changed with no section named. ⚠ `MUST`, `SHOULD` and silence conflated |
| **two implementations of one question** | ⚠ The same decision made in two places. ⚠ **If unavoidable, is it cross-checked mechanically?** |
| **the meaning of a recorded value** | A counter, a state name, or a unit changed meaning without saying so |
| **untrusted input** | A length or an offset that came from outside, used without checking it against what was actually read |
| **silent rejections** | Something discarded with nothing counted (⚠ indistinguishable from something that never arrived) |
| **internal state shown to a human** | A raw enum, an errno, or a status code reaching output |

| **a second address decision** (`security.md` § 1) | ⚠ A host or IP check outside `internal/policy`, or a test that re-lists the ranges instead of calling it |
| **a dial that bypasses the hook** (`security.md` § 1) | ⚠ `http.Get`, `DefaultTransport`, `net.Dial`, or dialling a host name |
| **a hop not re-validated** (`security.md` § 3) | ⚠ A redirect `Location` reaches the dialler without going through `target` and `policy` |
| **an unbounded wait or read** (`security.md` § 4) | ⚠ A step with no deadline; a read with no cap; a limit written inline instead of `internal/limits` |
| **a refused address shown** (`security.md` § 5) | ⚠ A refused IP in `detail`, a log, or an error message passed through |
| **our refusal phrased as their failure** (`security.md` § 5, `CLAUDE.md` § 4-1) | ⚠ "unreachable" for something we declined to reach |
| **a raw error classified by guess** (`go.md`) | ⚠ An unknown error mapped to the nearest known code |
| **page and API diverge** (`adr/0006`) | ⚠ A handler or template that computes something `diag.Result` does not carry |

---

## 4. ⚠ Stale results and ordering

⚠ **Does an old result overwrite the current state?**

⚠ **Anything answered asynchronously has this defect available to it.** ⚠ **The premise can move
while the answer is in flight**, and the answer arrives believing the old one.

| What to check | ⚠ How it fails |
|---|---|
| ⚠ **Was the premise the same when it was sent and when it came back?** | An answer is applied to state that has since moved on |
| ⚠ **A late reply overtaking a newer one** | ⚠ **A slow response arrives after, and overwrites, a newer one.** Arrival order is not send order |
| ⚠ **A sample attributed to the wrong attempt** | ⚠ **A measurement taken from a retried exchange cannot say which attempt it belongs to.** Attributing it anyway is a guess wearing the face of a measurement |
| **Timers still armed** | The condition changed but the timer is still scheduled to fire |
| **Ordering of failure and success** | "Nothing arrived" is emitted, then an old success lands. Or the reverse |
| ⚠ **Where state is written** | ⚠ The same state written from two places (⚠ **nothing decides which is last**) |
| **Cancellation** | Something cancelled still writes its result afterwards |
| **Identity** | A response matched to a request by position or timing rather than by an identifier |

⚠ **The way to check is not to read — it is to reorder.**
⚠ **Delay it, invert the order, and see whether it actually happens.**
⚠ **Never pass it because "it is unlikely."**

⚠ **This is a correctness question, not a presentation one.** ⚠ **State overwritten by a stale
result reads, from outside, as something that was observed.**

⚠ **Do not prescribe the fix.** Generation counters, identifier matching, or cancellation are
implementation calls. ⚠ **Only judge whether it is handled.** If it is not, `NEEDS-FIX`.

---

## 5. Tidy-up

| What to check | ⚠ How it fails |
|---|---|
| **Dead code** | Things no longer used are still there |
| ⚠ **Stale comments** | ⚠ **They mislead harder than code.** Old numbers, counts, filenames |
| ⚠ **Copied comments** | Moved to another file, ⚠ **and the numbers no longer match** |
| **Changes with no effect** | Written, but nothing changes (⚠ confirm by measuring) |
| **Fixed as a set** | implementation -> check -> comment -> README -> `docs/SPEC.md` |
| **Bare issue numbers** | ⚠ Write them with the repository name (a bare number points elsewhere after a migration) |

⚠ **Never write "confirmed" for something that was not verified.**

---

## 6. What to return

```
Verdict: PASS / NEEDS-FIX / HUMAN-DECISION

Scope:
  N files changed / M reasons        ⚠ only what was actually read

Acceptance Criteria:
  1. <AC>  -> <what confirmed it>  OK / NO / ⚠ unverified

Blocking:
  - <breaks a rule / outside scope / an AC unmet / ⚠ a stale result can overwrite current state>

Non-blocking:
  - <worth fixing, not worth blocking on>

Human Decision:
  - <spec, behaviour, meaning of a recorded value>
```

| Verdict | When |
|---|---|
| **PASS** | Every AC met, nothing outside scope, no rule touched |
| **NEEDS-FIX** | ⚠ **Any AC unmet** / outside scope / a rule touched / ⚠ **a stale result can overwrite** / tidy-up outstanding |
| **HUMAN-DECISION** | ⚠ **The spec, the behaviour, or the meaning of a value** must be settled first. ⚠ **Never settle it yourself** |

⚠ **One blocking item means it is not a PASS.**
⚠ **One "unverified" AC means it is not a PASS** (either `NEEDS-FIX`, or back to verifying to measure).
