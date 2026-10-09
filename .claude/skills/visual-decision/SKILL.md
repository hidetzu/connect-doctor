---
name: visual-decision
description: Turn "please decide" into "you can decide by looking" — build the current state and two or three candidates as artefacts the owner can compare, name the recommendation and what it costs, and say which candidates were dropped and why. Use before asking the owner to settle a presentation question.
---

# Visual decision

⚠ **`MUST` = required, `SHOULD` = default, `MAY` = optional.**

⚠ **This skill decides nothing.** ⚠ **It lowers what a decision costs the owner.**

⚠ **Whether to ask at all is owned by [`rules/owner-decisions.md`](../../rules/owner-decisions.md).**
⚠ **Never restate it here.** ⚠ **This skill starts after that file has already said "ask".**

⚠ **A picture is never verification** ([`rules/verification.md`](../../rules/verification.md)).
⚠ **It is material for a decision.** ⚠ **A check is what defends the claim afterwards.**

---

## Grounds

⚠ **A decision handed over as prose and numbers gets answered with a guess, or not answered.**
⚠ **The same decision handed over as "here it is now, here are two candidates" gets answered.**

⚠ **Measure your own project before quoting any number here** ([`rules/evidence.md`](../../rules/evidence.md)).
⚠ **Count how many of your open owner decisions turn on how something looks, and how many of
those carried an artefact.** ⚠ **Another project's ratio is not a claim about yours.**

> ⚠ **Provenance.** ⚠ **This skill has run in one domain, not two** (README §3).
> ⚠ **It is here because the contract is domain-independent, not because it reproduced.**
> ⚠ **The capture tooling did not come with it, on purpose** — same split the template already
> made for `verify`: ⚠ **the contract travels, the commands do not.**

---

## 1. Is this a decision made by looking?

⚠ **Separate this first.** ⚠ **Building artefacts for something settled by reading wastes the
owner's attention, which is the thing this skill exists to protect.**

| Decided by looking | Decided by reading |
|---|---|
| ⚠ **Colour, contrast, weight, density** | What may be claimed |
| ⚠ **Spacing, size, whether it fits** | What a value means |
| ⚠ **Order, and whether to fold something away** | Whether to add a source |
| ⚠ **Whether to keep an element or drop it** | Where the checks go |
| ⚠ **Two or more candidates that measurement cannot separate** | Anything counting settles |

- MUST: ⚠ **Decide by what is being settled, not by the word in the request.**
  ⚠ "Colour" in "should colour live in one file" is a structure question, not a look question.
- MUST NOT: ⚠ **Never hand over a look-at-it decision as numbers alone.**
- MUST NOT: ⚠ **Never ask what measuring would settle** (`owner-decisions.md`).

---

## 2. What to hand over

```text
Now            ⚠ before the change
Candidates     ⚠ two or three. ⚠ Never more
Short note     ⚠ one or two lines. ⚠ What changed
Recommended    ⚠ which one, why, ⚠ and what it loses
Dropped        ⚠ what was tried and rejected, ⚠ and the number that rejected it
```

- MUST: ⚠ **Always name a recommendation.** ⚠ **"Which do you prefer?" alone hands the work back.**
- MUST: ⚠ **Say what the recommendation costs.** ⚠ **A candidate with no downside was not measured.**
- MUST: ⚠ **Show what was dropped.** ⚠ **Otherwise the owner re-proposes it.**
- MUST NOT: ⚠ **Never offer more than three.** ⚠ **Beyond that the answer becomes "you pick".**

---

## 3. ⚠ Before you draw anything

- MUST: ⚠ **Make the candidate real and read the number yourself.**
  ⚠ **The capture tool does not judge**, and a tool that pretends to is worse than none.
- MUST: ⚠ **Confirm the property you are changing is the one in force.**
  ⚠ A candidate can look applied while the thing it targets has already moved elsewhere.
- MUST NOT: ⚠ **A candidate that moves no number is not a candidate.**
  ⚠ **Drop it, and say which number stayed put.**

### ⚠ Every option offered must be one that works

⚠ **This is the expensive one.** ⚠ **An option that cannot fit, cannot build, or breaks its
neighbour is not an option** — ⚠ **and offering it costs a full round trip through the owner.**

- MUST: ⚠ **Build each option and check it end to end at the worst end of the range**
  before it goes on the list.
- MUST: ⚠ **Check what it does to what sits next to it.**
  ⚠ **An option that fits only by crushing its neighbour has not been checked.**
- MUST: ⚠ **When an option turns out not to work after it was chosen, say so plainly with the
  number, and offer the nearest one that does.** ⚠ **Never quietly substitute.**

---

## 4. Capturing

- MUST: ⚠ **Identical conditions across every artefact**, and ⚠ **state them** —
  what size, what theme, what data, what was open.
- MUST: ⚠ **Capture both ends of every range that could flip the answer.**
  ⚠ Themes, widths, empty and full, cold and warm.
  ⚠ **Looking at one end and deciding is how the other end breaks.**
- MUST: ⚠ **Crop to the smallest container in which the decision is visible.**
  ⚠ **A whole-screen capture hides the difference being decided.**
- MUST NOT: ⚠ **Never modify the product to take the picture.**
  ⚠ **Apply the candidate at capture time**, and say so.
- MUST: ⚠ **Keep labels plain.** ⚠ **They become file names and column headings.**

---

## 5. Handing it over

```text
The artefacts themselves   ⚠ in the conversation. ⚠ Fastest
One composed page          ⚠ when it has to be shared onward
```

- MUST NOT: ⚠ **Never put a local absolute path or a session URL into anything public**
  ([`rules/git.md`](../../rules/git.md)).
- SHOULD: ⚠ **Numbers and words in the issue; artefacts in the conversation.**
  ⚠ **Do not commit captures to the repository.**

---

## 6. Asking

⚠ **`AskUserQuestion` and "never bury it in prose" are owned by `owner-decisions.md`.**
⚠ **What this skill adds is what the question has to contain.**

```text
⚠ Problem       one line
⚠ Now           the artefact
⚠ Candidates    the artefacts. ⚠ two or three
⚠ Recommended   which, why, ⚠ what it loses
⚠ Options       answerable in one word
```

- MUST: ⚠ **Say what was not checked.** ⚠ **A silent gap reads as "checked and fine".**
- MUST: ⚠ **With a still artefact, say which parts respond to being used.**
  ⚠ **Otherwise something interactive reads as decoration.**
- MAY: ⚠ **Put the same artefacts to stand-in readers first.**
  ⚠ **Say they are stand-ins, give the count, and give how each one read it.**
  ⚠ **A split reading is the result.** ⚠ **Never resolve it by majority.**

---

## 7. After it is decided

- MUST: ⚠ **Record the decision and its reason** (`owner-decisions.md` owns where it goes).
  ⚠ **Say that the owner settled it by looking.**
- MUST: ⚠ **Re-capture under the same conditions and show the change landed.**
- MUST NOT: ⚠ **Never report "verified" because an artefact looked right.**
  ⚠ **What defends the claim afterwards is a check** (`rules/verification.md`).

---

## ⚠ Not in scope

```text
Diffing artefacts automatically     ⚠ a human looks. ⚠ No automatic verdict
Forcing this onto every decision    ⚠ only the ones settled by looking
Deciding for the owner              ⚠ the owner decides
Committing captures                 ⚠ they belong in the conversation
```
