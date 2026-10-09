# Git

⚠ **`MUST` = required, `SHOULD` = default, `MAY` = optional.**

⚠ **This file is the owner of `CLAUDE.md` §8.** ⚠ **Never restate it there.**
⚠ **How to review a change is owned by [`change-review`](../skills/change-review/SKILL.md).**

## Commits

- MUST: Conventional Commits (`<type>(<scope>): <subject>`).
- MUST: ⚠ **Never sweep unrelated work in with `git add -A`.**
  ⚠ **One reason for a change, one commit.**
- MUST NOT: ⚠ **Never commit directly to the default branch.** Branch (`<type>/<short-name>`).
- SHOULD: Put **why** and **the measured numbers** in the body.

## Permission

- MUST: ⚠ **Take permission for `git push` every single time.**
  ⚠ **Permission granted before does not carry forward.**
  - ⚠ **One exception.** Via the Loop Controller, for **one issue that passed the quality gate**,
    pushing to that issue's branch and **opening the PR** count as approved
    (`.claude/skills/loop-controller/SKILL.md`).
  - ⚠ **Merge is never included.** ⚠ **The AI enters the loop on its own** — ⚠ **it applies
    `ready-for-ai` itself** (`rules/owner-decisions.md`) — ⚠ **so the merge is where the human
    enters, and permission for it is taken every single time.**
    ⚠ **Auto-merge (`--auto`), merge queues, and merges that bypass protection (`--admin`) are
    forbidden outright.** ⚠ **Never merge on red or in-progress CI.**
  - ⚠ **Used standalone, every skill still takes permission every time.** Nothing is weakened.

## ⚠ Never do these without being told

```text
git push --force
git reset --hard
git clean -fd
git checkout -- .
git restore .
```

- MUST: ⚠ **When one is genuinely needed, say what will be lost, first.**
- MUST: ⚠ **Never discard or overwrite someone's uncommitted work.**
- MUST: ⚠ **Never push to a branch someone else is working on without asking.**

## Before starting

- MUST: ⚠ **Look at the current branch and at uncommitted changes.**
- MUST: ⚠ **Keep our changes and theirs separate** (⚠ **a separate worktree makes this certain**).

## Conflicts

- MUST: ⚠ **Never discard the other side's change without understanding what it meant.**
- MUST: ⚠ **Re-run the checks after a merge or rebase.**

## ⚠ Never put the working environment into anything public

⚠ **Grounds: a public repository's commit bodies, PR bodies, issues and comments are readable by
anyone.** ⚠ **This is the mirror image of the evidence rules** ([`evidence.md`](evidence.md)):
⚠ **there, do not claim what was not observed; here, do not emit what was not meant to be emitted.**

⚠ **Apply it in a private repository too.** ⚠ **A repository's visibility can change; its history
does not get re-reviewed when it does.**

- MUST NOT: ⚠ **Never write an AI working-session URL.** ⚠ **Even when the contents are behind
  authentication, the session ids and how many there are can still be read.**
  ⚠ **Where a tool writes one by default, this rule wins.**
- MUST NOT: ⚠ **Never write a local absolute path** (`/home/<name>/…`), a hostname, or an internal URL.
- MUST NOT: ⚠ **Never write a token, a webhook, or an API key.**
- MUST: ⚠ **Write an issue with its repository name** (`owner/repo#N`).
  ⚠ **A bare number points at a different issue once anything is migrated.**

### ⚠ If it is already written, there are two places to erase, and one is not enough

```text
GitHub side    PR body, issue body, comments      ⚠ fixable with gh
git history    commit bodies                      ⚠ needs a rewrite and a force push
```

- MUST: ⚠ **Fixing the PR body does not fix the squash-merged commit body.** ⚠ They are separate.
- MUST: ⚠ **After a force push, an old commit can still be read by anyone who knows its SHA**,
  until the host collects it. ⚠ **Erasing it completely means asking the host.**
- MUST: ⚠ **Confirm it is genuinely unreadable before saying it was erased**
  ([`evidence.md`](evidence.md)).
