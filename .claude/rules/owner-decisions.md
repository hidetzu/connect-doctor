# Owner decisions

⚠ **`MUST` = required, `SHOULD` = default, `MAY` = optional.**

⚠ **This file is the owner of `CLAUDE.md` §7-1 and §7-2.** ⚠ **Never restate them there.**

## Grounds

⚠ **Stalling costs more than deciding.** ⚠ **But a decision the owner cannot walk back costs
more than either.** ⚠ **The line between them is the only thing this file draws.**

## Decide yourself vs. ask

⚠ **The default is: decide, and carry it through to the end.**
Ask only where **being wrong cannot be walked back**.

| Decide yourself | Ask |
|---|---|
| Implementation order, how to split it, where tests go | ⚠ **Anything a human reads** on screen or in output |
| What to measure and how | ⚠ **When the scope moves** (crossing `Non-goals`) |
| A bug with exactly one sensible fix | ⚠ **When what may be claimed changes** (`docs/SPEC.md`) |
| Adding docs, comments, tests | Two presentable options that **measurement cannot settle** |

- MUST: Ask with **`AskUserQuestion`**. ⚠ **Never bury the question in prose** (it gets missed).
  ⚠ **With `.claude/hooks/ask-slack.mjs` configured, this asks in Slack and receives the answer back.**
- MUST: ⚠ **Nothing else goes to Slack** — no progress, no completions, no failures
  (⚠ **they would bury the one thing that needs an answer**).
- MUST NOT: ⚠ **Never ask what measuring would settle.** Measure first.
- MUST: ⚠ **Asking stops the work.** Finish everything that does not depend on the answer
  **before** asking.
- MUST: ⚠ **Record what was asked.** Once decided, put the reason in `docs/adr/`.
- SHOULD: ⚠ **When the thing being settled is how something looks, hand over the artefacts, not
  a description of them** ([`visual-decision`](../skills/visual-decision/SKILL.md) owns how).
  ⚠ **That skill is not restated here, and this line is not restated there.**

## ⚠ An owner decision outranks your judgement

- MUST: ⚠ **Copy `Owner Decisions` out of an issue verbatim**, and never change one on your own
  judgement.
- MUST: ⚠ **A later comment overrides the issue body.** ⚠ **A decision made mid-flight often
  exists only in a comment.**
- MUST: ⚠ **When body and comment conflict and you cannot tell which is authoritative, ask.**
- MUST: ⚠ **When the issue, the spec, the ADRs and the code disagree, do not decide which is
  right.** ⚠ **Return that a human must decide.**

## ⚠ `ready-for-ai`

⚠ **This is not a priority label.** It marks "an AI can carry this to the end without a human
deciding mid-way".

⚠ **The AI applies it, under the conditions below.** ⚠ **It never removes it.**

### ⚠ The gate did not get weaker. It moved.

```text
before   human applies ready-for-ai  ->  human approves the plan  ->  ... -> merge (covered by that approval)
now      AI applies ready-for-ai     ->  ... -> PR                ->  ⚠ human approves the merge
```

- MUST: ⚠ **Exactly one human gate exists per issue, and it is the merge.**
  ⚠ **Never end up with zero.** ⚠ **If you are about to merge without having asked, you have
  misread this file.**
- MUST: ⚠ **The merge is where it sits because that is the point that cannot be walked back.**
  ⚠ **Applying a label can be.** ⚠ **The gate moved from the reversible end to the irreversible
  one; it was not removed.**
- MUST: ⚠ **`git push` and opening the PR are covered inside the Loop Controller and nowhere
  else** ([`git.md`](git.md)). ⚠ **Merge never is.**

### ⚠ When the AI may apply it

- MAY: ⚠ **Apply it only when [`../skills/issue-ready/SKILL.md`](../skills/issue-ready/SKILL.md)
  returns `Ready for AI: YES`**, ⚠ **and the verdict was posted to the issue first.**
  ⚠ **The reasoning goes on the issue before the label does** — ⚠ **a label with no verdict behind
  it is indistinguishable from one applied by mistake.**
- MUST: ⚠ **Apply it through one step that also records that the AI did it.**
  ⚠ **Never by hand, and never as a bare label edit.**
  ⚠ **Post-then-label-then-record as three commands drifts apart on the first busy day** —
  ⚠ **a rule every call site has to remember is not a rule, it is a hope.**
  ⚠ **Every project writes that step for itself; ⚠ this file names what it has to do, not how.**
- MUST: ⚠ **Re-run the gate immediately before work starts, every time**, even on an issue the AI
  labelled itself. ⚠ **Bodies and comments change after a label is applied.**

### ⚠ When the AI must not apply it

⚠ **Any one of these means no label.** ⚠ **Not "probably fine". No label.**

| # | ⚠ Condition | ⚠ Why |
|---|---|---|
| 1 | ⚠ **An owner decision on the issue is unresolved** | ⚠ **The owner has not decided yet** |
| 2 | ⚠ **A dependency the issue names is still open** | ⚠ **The work can be started and not finished** |
| 3 | ⚠ **The environment cannot run the verification the issue needs** | ⚠ **Then nobody can show it green** (`issue-ready` clause 10) |
| 4 | ⚠ **Anything the "Ask" column above covers is unsettled** | ⚠ **Scope, what may be claimed, anything a human reads** |
| 5 | `issue-ready` returned anything but `YES` | ⚠ **The gate already said no** |

- MUST NOT: ⚠ **Never remove the label.** ⚠ **Applying and removing are not symmetric:**
  ⚠ **removing one the owner applied overrides the owner.**
  ⚠ **When an issue should lose it, say so and stop.**
- MUST: ⚠ **When condition 1 or 4 is what stopped it, mark the issue as needing an owner decision
  and move to another issue.** ⚠ **Do not stall the whole queue on one owner question.**

### ⚠ Who applied it has to stay answerable

⚠ **Grounds: the question this change has to keep answering is "how often does the owner still
have to do it".** ⚠ **A number that only ever goes down without being looked at is not evidence
of autonomy; it is evidence that nobody looked.**

- MUST: ⚠ **Keep a record of the AI's own applications**, ⚠ **and compare it against what the
  host actually recorded.**
- MUST: ⚠ **Where the AI acts through the owner's own credentials, the host's actor field cannot
  separate them.** ⚠ **Using it anyway is dressing a guess as a measurement**
  ([`evidence.md`](evidence.md)).
  ⚠ **The owner's count is then a subtraction, ⚠ and it says so wherever it is printed.**
- MUST NOT: ⚠ **Never score it.** ⚠ **A count of zero is not a result.**

### ⚠ The label is still only an entry condition

- MUST: ⚠ **It is not a guarantee that the issue can be implemented.**
  ⚠ **Labels get applied by mistake — ⚠ including by the AI.**
- MUST: ⚠ **The Loop Controller re-runs the gate before touching anything, and stops on `NO`**
  ([`../skills/loop-controller/SKILL.md`](../skills/loop-controller/SKILL.md)).
