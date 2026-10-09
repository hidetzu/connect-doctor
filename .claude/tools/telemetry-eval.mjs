#!/usr/bin/env node
// Lay the recorded tasks out side by side, grouped by kind (Eval Phase 1).
//
// ⚠ **This is not scoring.** ⚠ **It arranges observed facts so they can be compared, nothing more.**
//
// ⚠ **Read-only.** ⚠ **Not one byte is written to `events.jsonl` or `tasks.jsonl`.**
//   ⚠ **No compaction either.** ⚠ The raw record is touched only by the writer
//   ⚠ (`../hooks/telemetry.mjs`).
//
// ## Usage
//
//   node .claude/tools/telemetry-eval.mjs
//   node .claude/tools/telemetry-eval.mjs --json
//
//   # ⚠ Read a different record (⚠ how a test exercises this without touching the real one)
//   CLAUDE_DEV_TELEMETRY_DIR=/tmp/xxx node .claude/tools/telemetry-eval.mjs
//
// ## ⚠ What is printed, and what is not
//
// ⚠ **Only values present in the record, and arithmetic over them.**
//
//     task count / count by kind / count by grouping
//     duration (median / p90)   ⚠ **only for tasks whose end was observed**
//     turns (average / median) / share that finished in one turn
//     share that spanned several sessions / count whose end was never observed
//
// ⚠ **Not printed** (⚠ **because it was never observed**):
//
//     success rate / failure rate / quality score / agent score
//     productivity / autonomy / PASS-FAIL / GOOD-BAD / overall score
//
// ⚠ **"One turn, therefore good" and "fast, therefore good" are not said either.**
//   ⚠ **Fast may only mean the job was small.** ⚠ **That is not distinguished here.**
//
// ## ⚠ Never mix fact with guess
//
// ⚠ **Three of the recorded fields are guesses** (the contract in `telemetry.mjs`):
//
//     grouping / task_type / task_type_source
//
// ⚠ **The per-kind table splits its rows on those guesses.** ⚠ **So the table is labelled.**
//   ⚠ **Printed silently, a guess wears the face of a measurement** (`.claude/rules/evidence.md`).
//
// ⚠ **In particular, a `grouping=turn` task has `turns=1` and exactly one session by definition**
//   (⚠ because one prompt was defined to be one task).
//   ⚠ **Comparing turn counts across kinds is meaningless.** ⚠ **It follows from how they were
//   ⚠ bundled, not from anything observed.**
//
// ## ⚠ The same task_id appears many times
//
// ⚠ **`tasks.jsonl` is append-only** (the contract in `telemetry.mjs`).
// ⚠ **Take the last line for that task_id as its current state.** ⚠ **Never rewrite it into one line.**
import { readFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
// ⚠ **The location lives in `../telemetry-dir.mjs`, in one place** (⚠ borrowed from the writer).
//   ⚠ **Held separately, the write target and the read target drift apart silently** (⚠ demonstrated).
import { telemetryDir } from "../telemetry-dir.mjs";

// ⚠ **Fixed ordering.** ⚠ **Unknown kinds are appended after it**
//   (⚠ never dropped — ⚠ **so that a new kind appearing is noticeable**).
export const KNOWN_TYPES = ["prompt", "issue_refine", "issue_execute"];

// ---------- reading ----------
// ⚠ **Never discard a broken line silently.** ⚠ **Return how many could not be read**
//   (⚠ discarded, ⚠ **nobody can tell the denominator shrank**).
export const parseJsonl = (text) => {
  const rows = [], unreadable = [];
  (text ?? "").split("\n").forEach((line, i) => {
    if (!line.trim()) return;
    try { rows.push(JSON.parse(line)); } catch { unreadable.push(i + 1); }
  });
  return { rows, unreadable };
};

// ⚠ **Take the last line for each task_id** (⚠ the contract above). ⚠ **Order follows first appearance.**
//
// ⚠ **Dropped rows are returned too.** ⚠ **A row can parse as JSON and still have no `task_id`.**
//   ⚠ **Dropped silently, nobody can tell the denominator shrank** (⚠ same story as unreadable lines).
//   ⚠ **Counted separately from unreadable lines.** ⚠ **The cause is different.**
export const snapshot = (rows) => {
  const seen = new Map();
  let invalid = 0;
  for (const r of rows) {
    const id = r?.task_id;
    if (typeof id !== "string" || !id) { invalid += 1; continue; }
    seen.set(id, r);                               // ⚠ a later row replaces an earlier one
  }
  return { tasks: [...seen.values()], invalid };
};

// ---------- counting ----------
// ⚠ **Sort a copy.** ⚠ **Never mutate the caller's array** (`[...xs]`).
// ⚠ **median averages the middle two when the count is even.** ⚠ **p90 is the measured value
//   sitting at the 90% position** (⚠ no interpolation. ⚠ **Only values actually measured**, `.claude/rules/evidence.md`).
export const median = (xs) => {
  if (!xs.length) return null;
  const s = [...xs].sort((a, b) => a - b), m = s.length >> 1;
  return s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2;
};
export const p90 = (xs) => {
  if (!xs.length) return null;
  const s = [...xs].sort((a, b) => a - b);
  return s[Math.min(s.length - 1, Math.ceil(s.length * 0.9) - 1)];
};
export const mean = (xs) => (xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : null);

// ⚠ **Seconds.** ⚠ **`null` when the end was never observed** (⚠ **not 0**).
//   ⚠ **A 0 would make an unfinished task look like it finished instantly.**
export const durationSec = (t) => {
  if (!t?.started_at || !t?.ended_at) return null;
  const a = Date.parse(t.started_at), b = Date.parse(t.ended_at);
  if (!Number.isFinite(a) || !Number.isFinite(b) || b < a) return null;
  return (b - a) / 1000;
};

const turnsOf = (t) => (Number.isInteger(t?.turns) && t.turns > 0 ? t.turns : null);
const sessionsOf = (t) => (Array.isArray(t?.session_ids) ? t.session_ids.length : null);

// ⚠ **For one set, count only what can be counted.**
//   ⚠ **Each figure carries its own denominator** (⚠ **they are not the same**, `.claude/rules/evidence.md`).
const statsOf = (tasks) => {
  const durations = tasks.map(durationSec).filter((v) => v !== null);
  // ⚠ **"no end recorded"** and ⚠ **"an end exists but is unreadable or inverted"** are separate
  const noEnd = tasks.filter((t) => !t?.ended_at).length;
  const brokenTime = tasks.length - durations.length - noEnd;
  const turns = tasks.map(turnsOf).filter((v) => v !== null);
  const sessions = tasks.map(sessionsOf).filter((v) => v !== null);
  return {
    tasks: tasks.length,
    duration: {
      samples: durations.length, unfinished: noEnd, unusable: brokenTime,
      median_sec: median(durations), p90_sec: p90(durations),
    },
    turns: {
      samples: turns.length, mean: mean(turns), median: median(turns),
      // ⚠ **Denominator is "those whose turn count was readable"** (⚠ not the total task count)
      one_turn: turns.length ? turns.filter((v) => v === 1).length / turns.length : null,
    },
    sessions: {
      samples: sessions.length,
      multi: sessions.length ? sessions.filter((v) => v > 1).length / sessions.length : null,
    },
  };
};

export const summarize = (rows, unreadableLines = []) => {
  const { tasks, invalid } = snapshot(rows);
  const byType = new Map();
  for (const t of tasks) {
    // ⚠ **An unknown kind must not break anything.** ⚠ **Absent becomes `(none)`, so it stays visible**
    const k = typeof t?.task_type === "string" && t.task_type ? t.task_type : "(none)";
    if (!byType.has(k)) byType.set(k, []);
    byType.get(k).push(t);
  }
  const order = [...KNOWN_TYPES.filter((k) => byType.has(k)),
    ...[...byType.keys()].filter((k) => !KNOWN_TYPES.includes(k)).sort()];
  const byGrouping = {};
  for (const t of tasks) {
    const g = typeof t?.grouping === "string" && t.grouping ? t.grouping : "(none)";
    byGrouping[g] = (byGrouping[g] ?? 0) + 1;
  }
  // ⚠ **The window being reported on.** ⚠ **Nothing is filtered out** (⚠ the whole record is in scope).
  //
  // ⚠ **Starts and ends are pooled, and both extremes taken.**
  //   ⚠ **The last end must not be used as the upper bound** (⚠ **this actually skewed**):
  //     ⚠ with a task from 20:00-20:10 and ⚠ **one that began at 21:00 and has not ended**,
  //     ⚠ **the 21:00 task is in the aggregate, yet Period read "20:00 - 20:10".**
  //   ⚠ **The meaning is "oldest and newest observation included in this aggregate".**
  //
  // ⚠ **Never sort these as strings** (⚠ **the same instant can be written differently**).
  //   ⚠ `2026-08-24T01:01:00Z` and `2026-08-24T10:00:00+09:00` ⚠ **order backwards on text alone.**
  //   ⚠ **Compare actual instants** (`Date.parse`).
  const observed = tasks
    .flatMap((t) => [t?.started_at, t?.ended_at])
    .filter((v) => typeof v === "string" && Number.isFinite(Date.parse(v)))
    .sort((x, y) => Date.parse(x) - Date.parse(y));
  return {
    schema: 1,
    // ⚠ **This is an aggregate of observed facts, ⚠ not a verdict on quality**
    kind: "observation",
    unreadable_lines: unreadableLines.length,
    // ⚠ **Broken-as-JSON rows** and ⚠ **readable-but-not-a-task rows** are separate (different causes)
    invalid_task_rows: invalid,
    period: { from: observed[0] ?? null, to: observed[observed.length - 1] ?? null },
    overall: statsOf(tasks),
    by_grouping: byGrouping,
    by_type: order.map((k) => ({ task_type: k, ...statsOf(byType.get(k)) })),
    // ⚠ **The output itself carries which fields are guesses** (⚠ so a reader need not read prose)
    estimated_fields: ["task_type", "grouping", "task_type_source"],
  };
};

// ---------- presenting ----------
const fmtDur = (sec) => {
  if (sec === null || sec === undefined) return "-";
  if (sec < 90) return `${sec.toFixed(0)}s`;
  if (sec < 5400) return `${(sec / 60).toFixed(1)}m`;
  return `${(sec / 3600).toFixed(1)}h`;
};
const fmtPct = (v) => (v === null || v === undefined ? "-" : `${Math.round(v * 100)}%`);
const fmtNum = (v) => (v === null || v === undefined ? "-" : String(Math.round(v * 10) / 10));
const fmtWhen = (s) => (s ? s.slice(0, 16).replace("T", " ") : "-");

// ⚠ **Table headings stay ASCII.** ⚠ **Mixed-width characters break the column alignment.**
export const format = (s) => {
  const L = [];
  L.push("AI Task Eval");
  L.push(`Period: ${fmtWhen(s.period.from)} - ${fmtWhen(s.period.to)}   (whole record, unfiltered)`);
  L.push("");
  L.push(`Tasks: ${s.overall.tasks}`);
  L.push(`  end observed:      ${s.overall.duration.samples}`);
  L.push(`  not finished:      ${s.overall.duration.unfinished}`);
  if (s.overall.duration.unusable) L.push(`  ! unreadable time: ${s.overall.duration.unusable}`);
  if (s.unreadable_lines) L.push(`  ! unreadable lines: ${s.unreadable_lines}`);
  if (s.invalid_task_rows) L.push(`  ! not a task:      ${s.invalid_task_rows} (no task_id)`);
  L.push("");
  L.push(`Duration (only the ${s.overall.duration.samples} whose end was observed)`);
  L.push(`  median ${fmtDur(s.overall.duration.median_sec)} / p90 ${fmtDur(s.overall.duration.p90_sec)}`);
  L.push("");
  L.push("By kind (task_type is a guess)");
  // ⚠ **Headings and values come from the same place** (⚠ two width lists drift apart).
  //   ⚠ **They actually did drift** (⚠ the Tasks column announced itself one column early).
  const COLS = [
    ["Type",          14, (r) => r.task_type],
    ["Tasks",          7, (r) => String(r.tasks)],
    ["Finished",      10, (r) => String(r.duration.samples)],
    ["Median",         9, (r) => fmtDur(r.duration.median_sec)],
    ["p90",            9, (r) => fmtDur(r.duration.p90_sec)],
    ["Turns(med)",    12, (r) => fmtNum(r.turns.median)],
    ["1-turn",         8, (r) => fmtPct(r.turns.one_turn)],
    ["Multi-session", 15, (r) => fmtPct(r.sessions.multi)],
  ];
  const line = (cells) => "  " + cells
    .map((v, i) => (i === 0 ? v.padEnd(COLS[i][1]) : v.padStart(COLS[i][1]))).join("");
  L.push(line(COLS.map(([h]) => h)));
  for (const r of s.by_type) L.push(line(COLS.map(([, , get]) => get(r))));
  if (!s.by_type.length) L.push("  (nothing recorded yet)");
  L.push("");
  L.push(`By grouping (also a guess): `
    + (Object.entries(s.by_grouping).map(([k, v]) => `${k} ${v}`).join(" / ") || "-"));
  L.push("");
  L.push("How to read this");
  L.push("  ! task_type / grouping / task_type_source are guesses, not observations");
  L.push("  ! a grouping=turn task has turns=1 and one session by definition.");
  L.push("    ! comparing turn counts across kinds is meaningless (it follows from the bundling)");
  L.push("  ! duration is wall-clock until the turn ended, not time spent working");
  L.push("  ! quality is not measured (no success rate, no quality, no autonomy)");
  return L.join("\n");
};

// ---------- entry ----------
export const readTasks = (dir) => {
  const f = join(dir, "tasks.jsonl");
  if (!existsSync(f)) return { rows: [], unreadable: [], missing: true };
  return { ...parseJsonl(readFileSync(f, "utf8")), missing: false };
};

const main = () => {
  const dir = telemetryDir();
  const { rows, unreadable, missing } = readTasks(dir);
  if (missing) {
    // ⚠ **Absent is not broken** (`.claude/rules/evidence.md`: not captured is not the same as did not happen)
    process.stdout.write(`AI Task Eval\n\n${join(dir, "tasks.jsonl")} does not exist yet.\n`
      + "! Either the telemetry hook has never run, or the record lives somewhere else.\n");
    return;
  }
  const s = summarize(rows, unreadable);
  process.stdout.write(process.argv.includes("--json")
    ? `${JSON.stringify(s, null, 2)}\n` : `${format(s)}\n`);
};

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) main();
