// The label names, and where an application of `ready-for-ai` is recorded (⚠ **this one place**).
//
// ⚠ **Two tools and four documents talk about the same string.**
//
//     .claude/tools/ready-for-ai.mjs   applies the label, and appends the record
//     .claude/tools/label-eval.mjs     reads the record back, and the timeline it is compared to
//     .claude/rules/owner-decisions.md states when the label may be applied
//     .claude/skills/{issue-ready,loop-controller}/SKILL.md   run the gate around it
//
// ⚠ **The two tools import from here.** ⚠ **The documents cannot import anything.**
//   ⚠ **So `docs-check.mjs` case `ready-for-ai-label-line` asserts that the rule file actually
//   ⚠ names the string declared below** — ⚠ **renaming the label in one place alone fails.**
//   ⚠ **Same shape as `telemetry-dir.mjs` and the `.gitignore` line**, and for the same reason:
//   ⚠ **two copies of one decision drift silently unless something mechanical holds them together.**
//
// ⚠ **The record lives in the telemetry directory**, so it inherits that directory's properties:
//   ⚠ **it never enters git, and not one byte of it leaves this machine** (see `.gitignore`).
//
// ⚠ **The record is what makes "who applied this label" answerable at all.**
//   ⚠ **`gh` acts as the repository owner's account, so GitHub's own timeline shows the same
//   ⚠ actor whether a human clicked it or the AI called the API.**
//   ⚠ **Without a local record there is nothing to subtract, and the question is unanswerable.**
//   ⚠ **See `label-eval.mjs` for what that still cannot show.**
import { join } from "node:path";
import { telemetryDir } from "./telemetry-dir.mjs";

// ⚠ **The label that marks "an AI can carry this to the end"**
//   (⚠ the exact string `.claude/rules/owner-decisions.md` must name).
export const READY_FOR_AI_LABEL = "ready-for-ai";

// ⚠ **The label that means the owner has not decided.** ⚠ **Its presence forbids the other one.**
export const NEEDS_DECISION_LABEL = "needs-decision";

/**
 * ⚠ **The mark that says the AI applied `ready-for-ai`, and not the owner.**
 *
 * ⚠ **Why this is a label and not a line in the local record.**
 *
 * ⚠ **The question is "who applied it", and it spans machines and people.**
 * ⚠ **A file on this laptop cannot answer it** — ⚠ **it cannot know about an application made
 * anywhere else, and its silence about one is indistinguishable from that one not happening**
 * (`.claude/rules/evidence.md`: ⚠ **not observed ≠ did not happen**).
 *
 * ⚠ **`gh` acts as the repository owner, so the timeline's actor is the owner either way.**
 * ⚠ **So the AI leaves its own mark in the same timeline the question is asked of.**
 *
 * ⚠ **What this is, exactly.**
 *
 * ⚠ **An operational attribution marker.** ⚠ **NOT evidence that GitHub identified an AI.**
 * ⚠ **GitHub identifies nobody here** — ⚠ **it records that a label was applied by the owner's
 * credentials, and that is all it will ever record.**
 *
 * ⚠ **Nothing enforces the mark.** ⚠ **A person could apply it by hand and the timeline would
 * look identical.** ⚠ **It holds because this project operates it, and for no other reason.**
 *
 * ⚠ **What it buys is not proof. ⚠ It is two smaller things:**
 * ⚠ **the attribution travels with the issue instead of living on one laptop,**
 * ⚠ **and it is per application instead of a subtraction over a total.**
 *
 * ⚠ **MUST NOT be applied by hand.** ⚠ **Doing so makes an owner application read as the AI's,
 * and nothing can tell afterwards.**
 */
export const APPLIED_BY_AI_LABEL = "applied-by-ai";

// ⚠ **Append-only.** ⚠ **One line per application. ⚠ Never rewritten, never compacted.**
export const LABEL_LOG_NAME = "labels.jsonl";

export const labelLogPath = () => join(telemetryDir(), LABEL_LOG_NAME);

// ⚠ **The verdict must say this before the label may be applied.**
//   ⚠ **Checked mechanically, so the tool cannot be pointed at a verdict that said anything else.**
export const YES_MARKER = /^\s*(?:⚠\s*)?\**\s*Ready for AI\s*:\s*\**\s*YES\b/im;
