#!/usr/bin/env node
// Hold the documents to what they say about themselves.
//
// ⚠ **These are the cases this template can assert about any project that copies it.**
//   ⚠ **They are not this project's verification** — that is `.claude/skills/verify/SKILL.md`,
//   ⚠ **and every project writes its own** (README §3).
//
// ⚠ **Read-only.** Nothing here writes, and nothing here reaches the network.
//
// ## Usage
//
//   node .claude/tools/docs-check.mjs                 run every case
//   node .claude/tools/docs-check.mjs --list          name the cases without running them
//   node .claude/tools/docs-check.mjs --only=links    run one case
//
// ⚠ **The count is announced here, at the moment it runs** (`rules/evidence.md`).
// ⚠ **Never copy it into a document.**
//
// Exit: 0 when every case that ran passed, 1 otherwise.

import { readFileSync, existsSync } from "node:fs";
import { join, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { execFileSync } from "node:child_process";

const ROOT = process.env.CLAUDE_PROJECT_DIR
  ?? join(dirname(fileURLToPath(import.meta.url)), "..", "..");

const read = (p) => readFileSync(join(ROOT, p), "utf8");

// ⚠ Strip before reading a document, or a check picks up the very words written to describe it
//   (`CLAUDE.md` §5). ⚠ Fenced blocks go too: an example of a bad shape is not the bad shape.
const stripMarkdown = (s) => s
  .replace(/```[\s\S]*?```/g, "")
  .replace(/<!--[\s\S]*?-->/g, "");

const markdownFiles = () =>
  execFileSync("git", ["ls-files", "*.md"], { cwd: ROOT, encoding: "utf8" })
    .trim().split("\n").filter(Boolean);

// ── the cases ──────────────────────────────────────────────────────────────
// Each returns { ok, said } — `said` is what gets printed either way, so a pass
// and a failure are read the same way.

const CASES = [
  {
    name: "links",
    // ⚠ A link that does not resolve sends the reader to a file that is not there. The rules
    //   cross-reference constantly ("owned by X"), so a rename quietly breaks the ownership map.
    run() {
      const broken = [];
      let checked = 0;
      for (const f of markdownFiles()) {
        for (const m of stripMarkdown(read(f)).matchAll(/\[[^\]]*\]\(([^)\s]+)\)/g)) {
          const target = m[1];
          if (/^(https?:|mailto:|#)/.test(target)) continue;
          const path = target.split("#")[0];
          if (!path) continue;
          checked++;
          if (!existsSync(resolve(join(ROOT, dirname(f)), path))) broken.push(`${f} -> ${target}`);
        }
      }
      return broken.length
        ? { ok: false, said: `${broken.length} of ${checked} relative links do not resolve:\n      ` + broken.join("\n      ") }
        : { ok: true, said: `every relative link resolves (${checked} checked, in ${markdownFiles().length} files)` };
    },
  },
  {
    name: "telemetry-ignore-line",
    // ⚠ Grounds: `.gitignore` and `telemetry-dir.mjs` hold two copies of one string, and the
    //   README says so. ⚠ Two copies of one decision is exactly what `CLAUDE.md` §3 forbids
    //   keeping unchecked. ⚠ If they drift, the telemetry stops being ignored and enters git —
    //   ⚠ and nothing announces it.
    run() {
      const declared = /TELEMETRY_DIR_NAME\s*=\s*["'`]([^"'`]+)["'`]/.exec(read(".claude/telemetry-dir.mjs"));
      if (!declared) return { ok: false, said: "telemetry-dir.mjs no longer declares TELEMETRY_DIR_NAME" };
      const line = `.claude/${declared[1]}/`;
      const ignored = read(".gitignore").split("\n").some((l) => l.trim() === line);
      return ignored
        ? { ok: true, said: `.gitignore ignores what telemetry-dir.mjs builds (both say ${line})` }
        : { ok: false, said: `telemetry-dir.mjs builds ${line}, and .gitignore has no such line — the telemetry would enter git` };
    },
  },
  {
    name: "no-count-in-spec",
    // ⚠ Grounds: `rules/evidence.md` — a count written into a document is stale from the moment
    //   it is written, and it makes every parallel change conflict. ⚠ Without this, that rule is
    //   a promise and not a wall (evidence.md says so itself).
    // ⚠ Add this project's own shapes here. ⚠ Never remove one to make a document pass.
    run() {
      const COUNT = [
        /\b\d+\s+(?:cases?|checks?|tests?|rows?|entry\s+points?|files?)\b/i,
        /\d+\s*(?:件|本|か所)/,
      ];
      const hits = [];
      for (const f of ["docs/SPEC.md"]) {
        if (!existsSync(join(ROOT, f))) continue;
        stripMarkdown(read(f)).split("\n").forEach((l, i) => {
          if (COUNT.some((re) => re.test(l))) hits.push(`${f}:${i + 1}: ${l.trim().slice(0, 70)}`);
        });
      }
      return hits.length
        ? { ok: false, said: `a count is written into the spec (${hits.length}). ⚠ Whatever produced it announces it:\n      ` + hits.join("\n      ") }
        : { ok: true, said: "no count is written into docs/SPEC.md (counts are announced by the runner)" };
    },
  },
];

// ── the runner ─────────────────────────────────────────────────────────────

const args = process.argv.slice(2);
const only = (args.find((a) => a.startsWith("--only=")) ?? "").slice("--only=".length);

// ⚠ Counting must not load anything heavy (`rules/verification.md`).
if (args.includes("--list")) {
  console.log(`docs-check: ${CASES.length} cases, none run`);
  for (const c of CASES) console.log(`  ${c.name}`);
  process.exit(0);
}

const chosen = only ? CASES.filter((c) => c.name === only) : CASES;
if (only && !chosen.length) {
  console.error(`docs-check: no case named "${only}". ⚠ --list names them.`);
  process.exit(1);
}

// ⚠ Announce the subset on the first line, before anything else (`rules/verification.md`).
console.log(only
  ? `docs-check: running 1 of ${CASES.length} cases (--only=${only})`
  : `docs-check: running ${chosen.length} of ${CASES.length} cases`);

let failed = 0;
for (const c of chosen) {
  let r;
  try { r = c.run(); }
  catch (e) { r = { ok: false, said: `the case could not run: ${e.message}` }; }
  if (!r.ok) failed++;
  console.log(`  ${r.ok ? "\x1b[32mok\x1b[0m  " : "\x1b[31mFAIL\x1b[0m"} ${c.name} — ${r.said}`);
}

console.log(failed
  ? `\ndocs-check: ${failed} of ${chosen.length} cases failed`
  : `\ndocs-check: ${chosen.length} of ${chosen.length} cases passed`);
process.exit(failed ? 1 : 0);
