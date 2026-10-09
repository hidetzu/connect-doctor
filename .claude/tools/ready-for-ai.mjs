#!/usr/bin/env node
// Post an issue-ready verdict, apply `ready-for-ai`, and record that the AI did it — ⚠ in one step.
//
// ⚠ **This is the only way the label may be applied by the AI**
//   (`.claude/rules/owner-decisions.md` § `ready-for-ai`).
//   ⚠ **`gh issue edit --add-label` skips the record, ⚠ and the record is the only thing that
//   ⚠ lets anyone tell the AI's labels from the owner's** (`label-eval.mjs`).
//
// ⚠ **Grounds for doing all three in one step: a rule every call site has to remember is not a
//   ⚠ rule, it is a hope** (`.claude/rules/security.md` says the same about redaction).
//   ⚠ **Post-then-label-then-record, as three commands, would drift apart on the first busy day.**
//
// ## Usage
//
//   node .claude/tools/ready-for-ai.mjs --issue 1 --verdict-file <path>
//   node .claude/tools/ready-for-ai.mjs --issue 1 --verdict-file <path> --dry-run
//
// ⚠ **`--dry-run` writes nothing, posts nothing and labels nothing.** ⚠ **It runs every refusal
//   check and says what it would have done** — ⚠ **so the checks can be exercised without
//   touching a public repository.**
//
// ## ⚠ What it refuses, and why a refusal is an answer
//
//     the issue is CLOSED             ⚠ never touch something already finished
//     needs-decision is present       ⚠ the owner has not decided
//     the verdict does not say YES    ⚠ the gate already said no
//     a named dependency is open      ⚠ the work could be started and not finished
//     the label is already there      ⚠ nothing to do (⚠ and nothing to record)
//
// ⚠ **A refusal is not an obstacle to work around.** ⚠ **Never re-run it with the check removed,
//   ⚠ and never fall back to `gh issue edit`.**
//
// ⚠ **It never removes a label.** ⚠ **Applying and removing are not symmetric** — ⚠ **removing one
//   the owner applied would override the owner.**
//
// Exit: 0 when the label is present at the end (applied now, or already there), 1 on a refusal
// or an error.
import { readFileSync, appendFileSync, mkdirSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { dirname } from "node:path";
import {
  APPLIED_BY_AI_LABEL, READY_FOR_AI_LABEL, NEEDS_DECISION_LABEL, YES_MARKER, labelLogPath,
} from "../ready-for-ai-label.mjs";

const args = process.argv.slice(2);
const arg = (name) => {
  const i = args.indexOf(name);
  return i >= 0 ? args[i + 1] : undefined;
};
const DRY = args.includes("--dry-run");

// ⚠ **Timestamps read the way "when did I work" reads** — local time with its offset,
//   ⚠ the same shape `hooks/telemetry.mjs` writes.
const stamp = () => {
  const d = new Date(), off = -d.getTimezoneOffset(), s = off < 0 ? "-" : "+";
  const p = (n) => String(Math.floor(Math.abs(n))).padStart(2, "0");
  return new Date(d.getTime() + off * 60000).toISOString().slice(0, 19)
    + `${s}${p(off / 60)}:${p(off % 60)}`;
};

// ⚠ **Append-only, and it never leaves this machine** — same contract as the rest of
//   ⚠ `.claude/telemetry/` (`hooks/telemetry.mjs`). ⚠ **Nothing here is scored.**
const record = (rec) => {
  try {
    const log = labelLogPath();
    mkdirSync(dirname(log), { recursive: true });
    appendFileSync(log, `${JSON.stringify({ ts: stamp(), ...rec })}\n`);
  } catch (e) {
    // ⚠ Losing a measurement is strictly better than losing the ability to work
    //   (`hooks/telemetry.mjs` says the same thing about itself).
    process.stderr.write(`ready-for-ai: the record could not be written: ${e.message}\n`);
  }
};

// ⚠ **Every refusal is recorded, not only every application.**
//   ⚠ Without this, "how often did the gate say no" is unanswerable — ⚠ and a refusal leaves no
//   ⚠ trace on GitHub at all, so this file is the only place it can be counted.
const die = (why, refusal) => {
  if (refusal !== undefined) record({ event: "label_refused", label: READY_FOR_AI_LABEL, by: "ai", ...refusal });
  console.error(`ready-for-ai: ${why}`);
  process.exit(1);
};
const gh = (a) => execFileSync("gh", a, { encoding: "utf8" });

// ⚠ **A redirected record means this is an exercise, and an exercise must not change the world.**
//
// ⚠ **Grounds: `.claude/rules/verification.md` § An exercise must not change the world.**
//   ⚠ **`CLAUDE_DEV_TELEMETRY_DIR` redirects the record; ⚠ it never redirects GitHub**, and its
//   ⚠ name reads as though it might. ⚠ (Ported from hidetzu/kagima, where that reading did real
//   ⚠ damage; ⚠ the incident stays there, the constraint travels.)
// ⚠ **So the tool now refuses.** ⚠ **`--dry-run` is the way to exercise it** — ⚠ **it says so.**
if (process.env["CLAUDE_DEV_TELEMETRY_DIR"] !== undefined && !DRY) {
  console.error(
    "ready-for-ai: CLAUDE_DEV_TELEMETRY_DIR is set, so this is an exercise — ⚠ and an exercise must not label a real issue.\n" +
      "⚠ That variable redirects the local record only. ⚠ GitHub is always real.\n" +
      "⚠ Add --dry-run to exercise this, or unset the variable to mean it.",
  );
  process.exit(1);
}

const issue = arg("--issue")?.replace(/^#/, "");
const verdictFile = arg("--verdict-file");
if (!issue || !/^\d+$/.test(issue)) die("--issue <N> is required");
if (!verdictFile) die("--verdict-file <path> is required (⚠ the verdict goes on the issue before the label does)");

const verdict = readFileSync(verdictFile, "utf8");
if (!verdict.trim()) die("the verdict file is empty");

// ⚠ **The gate's own words decide this, not the caller's intent.**
if (!YES_MARKER.test(verdict)) {
  die(
    `the verdict does not say "Ready for AI: YES" — ⚠ the gate said no, so the label does not go on`,
    { issue: Number(issue), why: "verdict-not-yes" },
  );
}

const repo = gh(["repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner"]).trim();
const view = JSON.parse(gh(["issue", "view", issue, "--json", "number,title,state,labels,body"]));
const labels = view.labels.map((l) => l.name);

console.log(`ready-for-ai: ${repo}#${view.number} — ${view.title}${DRY ? "   ⚠ DRY RUN" : ""}`);

if (view.state === "CLOSED") {
  die(`${repo}#${issue} is CLOSED — ⚠ never touch something already finished`,
    { repo, issue: Number(issue), why: "issue-closed" });
}
if (labels.includes(NEEDS_DECISION_LABEL)) {
  die(`${repo}#${issue} carries "${NEEDS_DECISION_LABEL}" — ⚠ the owner has not decided. ⚠ Not a label to work around`,
    { repo, issue: Number(issue), why: "needs-decision" });
}
if (labels.includes(READY_FOR_AI_LABEL)) {
  // ⚠ **Not an error, and not a record either.** ⚠ **Recording it would double-count an
  //   ⚠ application that already happened, possibly the owner's.**
  console.log(`  ok   already labelled "${READY_FOR_AI_LABEL}" — ⚠ nothing applied, nothing recorded`);
  process.exit(0);
}

// ---- dependencies (⚠ read, never guessed) ----------------------------------
// ⚠ **Only what the issue names**, on a line beginning `Depends on`, in the body or a comment.
//   ⚠ **Never inferred from a hunch about implementation order** (`issue-ready` clause 12).
const comments = JSON.parse(gh(["issue", "view", issue, "--json", "comments"])).comments ?? [];
const texts = [view.body ?? "", ...comments.map((c) => c.body ?? "")];
const deps = new Set();
for (const t of texts) {
  for (const line of t.split("\n")) {
    if (!/^\s*(?:[-*]\s*)?(?:⚠\s*)?\**\s*depends on\b/i.test(line)) continue;
    for (const m of line.matchAll(/(?:([\w.-]+\/[\w.-]+))?#(\d+)/g)) deps.add(`${m[1] ?? repo}#${m[2]}`);
  }
}

const open = [];
for (const d of deps) {
  const [r, n] = d.split("#");
  try {
    const st = gh(["issue", "view", n, "--repo", r, "--json", "state", "--jq", ".state"]).trim();
    if (st !== "CLOSED") open.push(`${d} (${st})`);
  } catch {
    // ⚠ **Could not be read ≠ closed** (`.claude/rules/evidence.md`). ⚠ **Treat as blocking.**
    open.push(`${d} (⚠ could not be read — ⚠ that is not the same as closed)`);
  }
}
console.log(`  deps ${deps.size ? [...deps].join(", ") : "none named"}`);
if (open.length) {
  die(`a named dependency is still open: ${open.join(", ")} — ⚠ the work could be started and not finished`,
    { repo, issue: Number(issue), why: "dependency-open", open });
}

if (DRY) {
  console.log(`  would post the verdict, apply "${READY_FOR_AI_LABEL}", and append one line to the record`);
  console.log(`\nready-for-ai: dry run — ⚠ nothing was posted, labelled or recorded`);
  process.exit(0);
}

// ---- ⚠ the verdict goes on the issue BEFORE the label does -----------------
// ⚠ **A label with no verdict behind it is indistinguishable from one applied by mistake.**
gh(["issue", "comment", issue, "--body-file", verdictFile]);
console.log(`  ok   verdict posted`);

// ⚠ Both labels, in one call. ⚠ Two calls could leave the mark off after a crash, and an
//   ⚠ unmarked application reads as the owner's forever after.
gh(["issue", "edit", issue, "--add-label", `${READY_FOR_AI_LABEL},${APPLIED_BY_AI_LABEL}`]);
console.log(`  ok   "${READY_FOR_AI_LABEL}" applied, marked "${APPLIED_BY_AI_LABEL}"`);

// ---- the record ------------------------------------------------------------
// ⚠ **Append-only. ⚠ Never rewritten.** ⚠ **It never enters git and never leaves this machine.**
// ⚠ **Written last on purpose**: ⚠ **a record of something that did not happen is worse than a
//   ⚠ missing record**, and a missing one is visible as an unexplained owner application in
//   ⚠ `label-eval.mjs` rather than silently vanishing.
// ⚠ **The cross-check, not the source of truth.** ⚠ **Attribution is read off the timeline now**
//   ⚠ (`tools/label-eval.mjs`); ⚠ **this file is what catches the two disagreeing.**
record({
  event: "label_applied",
  label: READY_FOR_AI_LABEL,
  marker: APPLIED_BY_AI_LABEL,
  repo,
  issue: Number(view.number),
  by: "ai",                     // ⚠ **the only value this tool ever writes**
  tool: "ready-for-ai.mjs",
  // ⚠ **Length only.** ⚠ **The verdict itself is on the issue; ⚠ it is not duplicated here**
  verdict_chars: verdict.length,
  dependencies: [...deps],
});
console.log(`  ok   recorded (by=ai)`);

console.log(`\nready-for-ai: ${repo}#${view.number} labelled. ⚠ The label is an entry condition, not a guarantee — ⚠ re-run the gate before starting work.`);
