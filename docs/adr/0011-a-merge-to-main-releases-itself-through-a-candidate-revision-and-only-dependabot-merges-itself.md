# 0011 — A merge to main releases itself through a candidate revision; for now only Dependabot merges itself

Status: accepted (2026-10-17). Owner decisions, 2026-10-17. ⚠ **Amends the release procedure of
[`0008`](0008-public-exposure-is-cloud-run-in-tokyo.md)** (where it runs, not what runs).
⚠ **Amends [`.claude/rules/git.md`](../../.claude/rules/git.md) § Permission and
[`.claude/rules/owner-decisions.md`](../../.claude/rules/owner-decisions.md) § `ready-for-ai`**
for Dependabot pull requests only.

## Decision

```text
PR ──> CI: fast, final, Docker image build + start, Claude review ──> merge
                                                                        │
main ──> build + push image :<sha> ──> deploy it with no traffic, tagged candidate
                                                                        │
         DEPLOY.md § 3 against the candidate's own URL ──> all pass ──> move 100 % of traffic to it
                                                      └──> any fail ──> stop; traffic stays where it was
```

1. ⚠ **Every merge to `main` is released by GitHub Actions**, not from the owner's machine.
2. ⚠ **A release reaches users only through a candidate revision**: deployed with `--no-traffic`
   under a tag, checked with [`DEPLOY.md`](../DEPLOY.md) § 3 at the tag's URL, ⚠ **then promoted
   with no human step**. ⚠ **A failed check leaves the serving revision untouched.**
3. ⚠ **The pull-request checks gain a Docker image build that also starts the binary**, and an
   automated review by Claude (`change-review`'s questions, on the diff).
4. ⚠ **Dependabot pull requests merge themselves** when every required check, the review
   included, passes. ⚠ **Every other pull request is still merged by the owner.**
   ⚠ **The review runs Claude Opus 5.5 at effort `medium`** (owner, 2026-10-17). ⚠ **Moving it
   to Sonnet is measured first**: once Opus has a quality baseline on real pull requests, the
   same pull requests are reviewed by Sonnet and the verdicts compared; ⚠ **not switched on
   price alone.**
   ⚠ **The Dependabot merge was taken in unverified end to end** (owner, 2026-10-17): a
   `workflow_run` workflow runs only from the default branch, so it cannot be exercised before
   merge. ⚠ **The first Dependabot pull request after it is the verification**, reported on
   hidetzu/connect-doctor#62. ⚠ **If the merge itself fails, the pull request stays open for
   the owner. If the merge succeeds and a dispatch fails, `main` is merged but not released**:
   the automerge run fails visibly, and the next release (any merge, or
   `gh workflow run release.yml`) carries it. ⚠ **After the merge it dispatches `check.yml` as
   well as `release.yml`**, so the README badge reports the merged commit (owner, 2026-10-17).
   ⚠ **A Dependabot change under `.github/workflows/` is not merged by it**: `GITHUB_TOKEN`
   cannot hold the `workflows` permission, so those (the github-actions updates) are the owner's.
5. ⚠ **The target is that every pull request merges itself once the review has earned it.**
   ⚠ **That is a later decision, made by amending this ADR — not by widening the Dependabot rule.**

| Built by | |
|---|---|
| hidetzu/connect-doctor#59 | Docker image build + start on every pull request |
| hidetzu/connect-doctor#60 | Claude review on every pull request |
| hidetzu/connect-doctor#61 | Release through a candidate revision |
| hidetzu/connect-doctor#62 | Dependabot merges itself, then starts the release |

## Why

- ⚠ **Measured gap, 2026-10-17 (hidetzu/connect-doctor#58):** CI installs Go from `go.mod`, so a
  `Dockerfile` that moved to Go 1.27 was never built by any check before merge. ⚠ **The image the
  service runs was built only on the owner's machine.** Building it in CI closes that.
- ⚠ **The release ran from one machine with three hand-offs** (merge, build and push, deploy), and
  ⚠ **the agent's permission classifier blocked `docker push` on one day and allowed it on another**.
  A pipeline does the same thing every time.
- ⚠ **A candidate revision makes "deploy, and if nothing is wrong, deploy" literal**: the checks run
  against the very revision that will serve, ⚠ **before** any user reaches it.

## ⚠ What this gives up, said plainly

- ⚠ **For a Dependabot pull request there is no human step between an upstream release and
  production.** The owner accepted this. ⚠ **What stands in for the human is: the required checks,
  the review, and § 3 against the candidate.** ⚠ **None of them reads the upstream project's
  changes**; a malicious base image or action that still builds, starts and passes § 3 ships.
  ⚠ **This is the gap `owner-decisions.md` "never end up with zero" exists for; it is opened here
  on purpose, for Dependabot only.**
- ⚠ **GitHub now holds a credential that can deploy.** It is obtained through Workload Identity
  Federation (⚠ **no key file exists**), ⚠ **only for workflows of `hidetzu/connect-doctor` on
  `refs/heads/main`**, as a deployer account separate from the runtime account. ⚠ **The runtime
  account keeps no roles** ([`0008`](0008-public-exposure-is-cloud-run-in-tokyo.md)); § 3 keeps
  checking it.
- ⚠ **The review costs money per pull request** (Anthropic API, a repository secret, and a
  Dependabot secret, because Dependabot runs cannot read Actions secrets).

## ⚠ Constraints the implementation must keep

- ⚠ **A merge made with `GITHUB_TOKEN` does not start `push` workflows** (GitHub Docs, *Triggering a workflow from a workflow*: "events triggered
  by the `GITHUB_TOKEN` … will not create a new workflow run", except `workflow_dispatch` and
  `repository_dispatch`). ⚠ **So the Dependabot merge must start the release with
  `workflow_dispatch` itself**, ⚠ **not** with `gh pr merge --auto` and ⚠ **not** with a personal
  access token that can merge.
- ⚠ **No workflow that holds the deploy credential or the API key runs code from a pull request's
  head.**
- ⚠ **Promotion is by revision name, never `--to-latest`**: a second release racing the first must
  not promote a revision that § 3 did not check.
- ⚠ **§ 3 runs from GitHub's runners against the live internet** (`example.com`); ⚠ **a failure
  there is an external failure that still blocks promotion** ([`verification.md`](../../.claude/rules/verification.md)).

## Rejected

- **Owner approves each promotion (GitHub Environment reviewer).** Kept the human in, ⚠ **but the
  owner chose fully automatic promotion**; the candidate checks are the gate.
- **Every pull request merges itself now.** The target, ⚠ **not yet**: the review has no record
  here yet. Dependabot first.
- **A personal access token or GitHub App so `--auto` merges trigger `push`.** ⚠ **A stored
  credential that can merge is a bigger thing to lose than one `workflow_dispatch` call.**
- **Keeping the release on the owner's machine.** It is what failed today.
