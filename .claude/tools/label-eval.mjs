#!/usr/bin/env node
// Count who applied `ready-for-ai`, and how often the gate said no.
//
// ⚠ **This is not scoring.** ⚠ **Observation only** (same contract as `telemetry-eval.mjs`).
// ⚠ **Read-only.** ⚠ **It writes nothing, and it never applies or removes a label.**
//
// ## ⚠ What changed, and why it had to
//
// ⚠ **This used to subtract.** ⚠ **Total applications, minus what a local file said the AI did,
//   ⚠ and the remainder was called the owner's.** ⚠ **That was an inference wearing a number.**
//
// ⚠ **The reason it had to be: `gh` acts as the repository owner, so the timeline's actor is the
//   ⚠ owner whoever pressed the button.** ⚠ **And a file on one laptop cannot know about an
//   ⚠ application made anywhere else** — ⚠ **its silence about one is indistinguishable from that
//   ⚠ one not happening** (`.claude/rules/evidence.md`).
//
// ⚠ **So the AI now leaves its own mark in the same timeline the question is asked of**
//   (`APPLIED_BY_AI_LABEL`).
//
// ⚠ **The mark is a convention this project operates.** ⚠ **It is NOT evidence that GitHub
//   ⚠ identified an AI** — ⚠ **GitHub identifies nobody here, and nothing stops a person applying
//   ⚠ the mark by hand.**
// ⚠ **What improved is smaller than "measured", and worth saying exactly:**
//   ⚠ **the attribution travels with the issue rather than living on one laptop, and it is per
//   ⚠ application rather than a subtraction over a total.**
//
// ## ⚠ What is read, and what is still not
//
//     read from the timeline   every `ready-for-ai` application
//     read from the timeline   which of them carry the mark, and which do not
//     ⚠ by convention           that a marked one is the AI's and an unmarked one is the owner's
//     ⚠ NOT known               applications made before the mark existed  ⚠ their own row
//     ⚠ NOT known               refusals   ⚠ they leave no trace on GitHub; ⚠ local record only
//
// ## Usage
//
//   node .claude/tools/label-eval.mjs
//   node .claude/tools/label-eval.mjs --json
//
//   # ⚠ Read a different record (⚠ how a test exercises this without touching the real one)
//   CLAUDE_DEV_TELEMETRY_DIR=/tmp/xxx node .claude/tools/label-eval.mjs
import { existsSync, readFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { APPLIED_BY_AI_LABEL, READY_FOR_AI_LABEL, labelLogPath } from "../ready-for-ai-label.mjs";

/**
 * ⚠ **The whole judgement, as one pure function.**
 *
 * ⚠ **Pure so it can be run against fixtures rather than against whatever the repository happens
 * to contain today** — ⚠ **`tools/docs-check.mjs` case `label-attribution` does exactly that.**
 * ⚠ **Attribution is the part that was wrong before; it is the part that gets a check.**
 *
 * @param events ⚠ **`labeled` events only**, as `{ issue, label, at }`. ⚠ Order does not matter.
 */
export const attribute = (events) => {
  const applications = events.filter((e) => e.label === READY_FOR_AI_LABEL);
  const marks = events.filter((e) => e.label === APPLIED_BY_AI_LABEL);
  const markedIssues = new Set(marks.map((e) => e.issue));

  // ⚠ The mark did not always exist. ⚠ Anything before the first one cannot be attributed by it,
  //   ⚠ and calling those the owner's would be the same mistake this rewrite is fixing.
  //   ⚠ With no marks at all, nothing is attributable — ⚠ which is the honest answer, not zero.
  const markEra = marks.length === 0 ? null : marks.map((e) => e.at).sort()[0];

  const byAi = [];
  const byOwner = [];
  const predatesTheMark = [];
  for (const a of applications) {
    if (markedIssues.has(a.issue)) byAi.push(a);
    else if (markEra === null || a.at < markEra) predatesTheMark.push(a);
    else byOwner.push(a);
  }
  return { applications, byAi, byOwner, predatesTheMark, markEra };
};

/** ⚠ **Never discard a broken line silently.** ⚠ **A discarded line shrinks the denominator unseen.** */
export const readRecord = (text) => {
  const applied = [];
  const refused = [];
  const unreadable = [];
  (text ?? "").split("\n").forEach((line, i) => {
    if (!line.trim()) return;
    try {
      const r = JSON.parse(line);
      if (r.label !== READY_FOR_AI_LABEL) return;
      if (r.event === "label_applied") applied.push(r);
      else if (r.event === "label_refused") refused.push(r);
    } catch {
      unreadable.push(i + 1);
    }
  });
  return { applied, refused, unreadable };
};

// ── reading the world ───────────────────────────────────────────────────────

const gh = (a) => execFileSync("gh", a, { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });

const timelineEvents = () => {
  const repo = gh(["repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner"]).trim();
  // ⚠ Every issue, not only the ones currently labelled — ⚠ a label that was later removed was
  //   ⚠ still applied once, and the question is about applications.
  const numbers = JSON.parse(
    gh(["issue", "list", "--state", "all", "--limit", "500", "--json", "number"]),
  ).map((i) => i.number);

  const events = [];
  for (const n of numbers) {
    let timeline;
    try {
      timeline = JSON.parse(gh(["api", `repos/${repo}/issues/${n}/timeline`, "--paginate"]));
    } catch {
      // ⚠ Could not be read ≠ nothing there. ⚠ Reported below, never folded into a count.
      events.push({ issue: n, label: null, at: null, unreadable: true });
      continue;
    }
    for (const e of timeline) {
      if (e.event !== "labeled") continue;
      events.push({ issue: n, label: e.label?.name ?? "", at: e.created_at ?? "" });
    }
  }
  return { repo, events };
};

// ── the report ──────────────────────────────────────────────────────────────

// ⚠ **Importing this file must do nothing.** ⚠ **`docs-check.mjs` imports `attribute` to run it
//   ⚠ against fixtures; ⚠ without this guard that import printed the whole report and reached
//   ⚠ the network.** ⚠ **A check that has side effects is not a check.**
const RUN_AS_MAIN = process.argv[1] !== undefined && import.meta.filename === process.argv[1];
if (!RUN_AS_MAIN) {
  // ⚠ Nothing below runs. ⚠ The exports above are the whole of what an importer gets.
} else {
  main();
}

function main() {
const JSON_OUT = process.argv.includes("--json");

const log = labelLogPath();
const record = readRecord(existsSync(log) ? readFileSync(log, "utf8") : "");

let world = null;
let why = null;
try {
  world = timelineEvents();
} catch (e) {
  why = e?.message?.split("\n")[0] ?? String(e);
}

const readable = world === null ? [] : world.events.filter((e) => !e.unreadable);
const unreadableIssues = world === null ? [] : world.events.filter((e) => e.unreadable).map((e) => e.issue);
const seen = world === null ? null : attribute(readable);

// ⚠ The local record is the cross-check now, not the source of truth. ⚠ Where the two disagree,
//   ⚠ that disagreement is the interesting thing, so it is printed rather than resolved.
const recordedIssues = new Set(record.applied.map((r) => r.issue));
const markedIssues = new Set(seen === null ? [] : seen.byAi.map((a) => a.issue));
const recordedButUnmarked = [...recordedIssues].filter((n) => !markedIssues.has(n)).sort((a, b) => a - b);
const markedButUnrecorded = [...markedIssues].filter((n) => !recordedIssues.has(n)).sort((a, b) => a - b);

const refusalsByReason = {};
for (const r of record.refused) refusalsByReason[r.why ?? "unsaid"] = (refusalsByReason[r.why ?? "unsaid"] ?? 0) + 1;

if (JSON_OUT) {
  console.log(
    JSON.stringify(
      {
        label: READY_FOR_AI_LABEL,
        marker: APPLIED_BY_AI_LABEL,
        timeline_reachable: world !== null,
        timeline_error: why,
        observed: seen === null
          ? null
          : {
              applications_total: seen.applications.length,
              applied_by_ai: seen.byAi.length,
              applied_by_owner: seen.byOwner.length,
              predates_the_mark: seen.predatesTheMark.length,
              mark_first_seen: seen.markEra,
            },
        not_observed_on_github: { refusals_by_reason: refusalsByReason },
        cross_check: { recorded_but_unmarked: recordedButUnmarked, marked_but_unrecorded: markedButUnrecorded },
        unreadable_timelines: unreadableIssues,
        unreadable_record_lines: record.unreadable,
      },
      null,
      2,
    ),
  );
  process.exit(0);
}

console.log(`ready-for-ai — who applied it`);
console.log(`Label: ${READY_FOR_AI_LABEL}   Mark: ${APPLIED_BY_AI_LABEL}`);
console.log("");

if (seen === null) {
  // ⚠ Say it could not be obtained. ⚠ Never print 0 and let it read as "none happened".
  console.log(`⚠ GitHub's timeline could not be read: ${why}`);
  console.log(`⚠ That is not the same as "the label was never applied."`);
  console.log(`⚠ Nothing below is a count of applications.`);
} else {
  console.log(`Read from the timeline (⚠ attributed by a mark this project operates)`);
  console.log(`⚠ GitHub identifies nobody here. ⚠ The mark is ours, and nothing enforces it.`);
  console.log(`  applications, total:              ${seen.applications.length}`);
  console.log(`  ⚠ marked ${APPLIED_BY_AI_LABEL}:          ${seen.byAi.length}   ⚠ the AI, if the mark was only ever applied by the tool`);
  console.log(`  ⚠ unmarked:                       ${seen.byOwner.length}   ⚠ the owner, on the same condition`);
  console.log(`  ⚠ before the mark existed:        ${seen.predatesTheMark.length}   ⚠ not attributable`);
  if (seen.markEra !== null) console.log(`  (the mark is first seen at ${seen.markEra})`);
  if (unreadableIssues.length) {
    console.log(`  ⚠ timelines that could not be read: ${unreadableIssues.join(", ")}`);
  }
}
console.log("");

console.log(`Refusals (⚠ local record only — ⚠ a refusal leaves no trace on GitHub)`);
const reasons = Object.keys(refusalsByReason).sort();
if (reasons.length === 0) console.log(`  none recorded on this machine`);
for (const r of reasons) console.log(`  ${r.padEnd(18)} ${refusalsByReason[r]}`);
console.log("");

console.log(`Cross-check (⚠ the local record against the timeline)`);
console.log(`  applications the AI recorded:     ${record.applied.length}`);
console.log(`  ⚠ recorded here, unmarked there:  ${recordedButUnmarked.length ? recordedButUnmarked.join(", ") : "none"}`);
console.log(`  ⚠ marked there, not recorded here: ${markedButUnrecorded.length ? markedButUnrecorded.join(", ") : "none"}`);
if (record.unreadable.length) console.log(`  ⚠ unreadable lines in the record: ${record.unreadable.join(", ")}`);
console.log("");

console.log(`How to read this`);
console.log(`  ! GitHub does not identify an AI. ! gh acts as the owner whoever pressed the`);
console.log(`    button, and the timeline's actor says so. ! the mark is this project's own.`);
console.log(`  ! nothing enforces the mark. ! applied by hand, it would look identical, and`);
console.log(`    every figure below it would be wrong with nothing announcing it.`);
console.log(`  ! what improved over the subtraction it replaced: the attribution travels with`);
console.log(`    the issue instead of one laptop, and it is per application, not a remainder.`);
console.log(`  ! that is smaller than "measured". ! it is said that way on purpose.`);
console.log(`  ! applications made before the mark existed are their own row. ! they are not`);
console.log(`    attributable by this mechanism, and calling them the owner's would be a guess.`);
console.log(`  ! refusals are local-only. ! a refusal on another machine is not counted here,`);
console.log(`    and this file's silence about one is not evidence that it did not happen.`);
console.log(`  ! the mark is applied by the tool, never by hand. ! a hand-applied mark would`);
console.log(`    make an owner application read as the AI's, and nothing could tell afterwards.`);
console.log(`  ! nothing here says whether applying the label was the right call.`);
}
