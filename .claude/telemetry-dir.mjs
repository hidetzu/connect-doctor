// Where the development telemetry is written and read (⚠ **this one place**).
//
// ⚠ **The writer and the reader used to each carry their own copy of this string**
//   (fixed in konjaku on 2026-08-24. ⚠ **The grounds is that incident; it is not a pitfall of
//   ⚠ this repository** — see `.claude/rules/README.md`).
//
//     .claude/hooks/telemetry.mjs        writes
//     .claude/tools/telemetry-eval.mjs   reads
//     the static check                   excludes it from scans / asserts git does not track it
//
// ⚠ **Demonstrated in konjaku (2026-08-24, before the two were merged):**
//   ⚠ **changing the location on the writer side alone broke no test at all.**
//   ⚠ **The reader kept reading an empty directory and nobody noticed.**
//   ⚠ **Worse, `.gitignore` still excluded the *old* name, so the records started
//   ⚠ landing in git** — the promise "this never enters git" broke silently.
//
// ⚠ **Rule: never keep two implementations that answer the same question.**
//
// ⚠ **`.gitignore` alone cannot import from here** (it is git syntax, not code).
//   ⚠ **So it must be cross-checked mechanically**: a static check has to assert that
//   ⚠ `TELEMETRY_IGNORE_LINE` below actually appears in `.gitignore`.
//   ⚠ **That check does not exist in this repo yet.** Until it does, this is a promise,
//   ⚠ not an enforced invariant. Do not describe it as enforced.
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

// ⚠ **The directory name under `.claude/`** (⚠ the same string `.gitignore` excludes).
export const TELEMETRY_DIR_NAME = "telemetry";

// ⚠ **The exact line `.gitignore` must contain** (⚠ the check compares against this string).
export const TELEMETRY_IGNORE_LINE = `.claude/${TELEMETRY_DIR_NAME}/`;

// ⚠ **Project root.** `CLAUDE_PROJECT_DIR` is handed to hooks by Claude Code
//   (⚠ **the directory the session started in**). ⚠ If absent, walk up from this file.
export const projectRoot = () => process.env.CLAUDE_PROJECT_DIR
  ?? join(dirname(fileURLToPath(import.meta.url)), "..");

// ⚠ **Both the write target and the read target.**
//   ⚠ **`CLAUDE_DEV_TELEMETRY_DIR` overrides it**, so a test can run the whole path
//   ⚠ end to end without polluting the real records.
export const telemetryDir = () => process.env.CLAUDE_DEV_TELEMETRY_DIR
  ?? join(projectRoot(), ".claude", TELEMETRY_DIR_NAME);
