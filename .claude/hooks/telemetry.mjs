#!/usr/bin/env node
// Record when a task was handed to the AI, and how it ended, so it can be traced later (Phase 1).
//
// ⚠ **This is not scoring.** ⚠ **Observation only.** It never judges success or failure.
//
// ⚠ **Session ≠ Task.** One task spans several turns and sometimes several sessions.
//
//     Task
//      └─ Claude Code session
//           ├─ UserPromptSubmit   … a turn begins
//           └─ Stop               … a turn ends
//
// ⚠ **This is the record of development, not of the product.** ⚠ **Not one byte leaves this
//   machine.** ⚠ **It never enters git** (see `.gitignore`).
//
// ⚠ **Never blocking work is the top priority.** `UserPromptSubmit` and `Stop` are hooks that
//   halt things when they exit non-zero (the former swallows the prompt entirely; the latter
//   makes the turn unable to finish). ⚠ **Whatever happens, exit 0.** Losing a measurement is
//   strictly better than losing the ability to work.
//
// ⚠ **Never write a single character to stdout.** A `UserPromptSubmit` hook's stdout is read
//   as extra context for Claude — the measurement would leak into the conversation.
//   ⚠ Complaints go to stderr.
//
// ## What is recorded, and what is not
//
// ⚠ **No content.** None of the following is needed to identify a task, so none is written:
//
//     prompt body            final reply body      transcript contents or path
//     cwd / env vars         tool inputs/outputs   token counts / cost
//     hashes of the body     personal names        secrets (API keys / tokens / passwords)
//
// ⚠ **`prompt_id` stands in for the body** (a UUID assigned by Claude Code).
//   ⚠ **It links the start and the end of the same prompt while holding zero characters of it.**
//   ⚠ **Only the length is kept** — so that "do longer requests take more turns?" stays answerable.
//
// ## Location
//
//     .claude/telemetry/events.jsonl   one event per line (raw)
//     .claude/telemetry/tasks.jsonl    one line appended per task, on every Stop
//     .claude/telemetry/state.json     index of open tasks (used to link the next turn)
//
// ⚠ **`tasks.jsonl` is append-only.** ⚠ **The same task_id appears many times.**
//   ⚠ **When reading, take the last line for that task_id** (append-only cannot corrupt; never rewrite).
//
// ⚠ **`CLAUDE_DEV_TELEMETRY_DIR` redirects the write target**, so a test can exercise the whole
//   path without polluting the real records.
//
// ## Task ID
//
// ⚠ **Never ask a human to supply it.** ⚠ **Decide only from what the prompt itself carries.**
//
//     an issue could be read   →  that issue is the task (⚠ **same task across sessions**)
//     nothing could be read    →  ⚠ **one prompt = one task** (⚠ **do not bundle**)
//
// ⚠ **Bundle only when there is a reason to bundle** (fixed in konjaku on 2026-08-24).
//   ⚠ **It used to treat consecutive turns in one session as one task.** ⚠ **That is a guess,
//   ⚠ and an irreversible one:**
//     "tidy up the build flags" → "fix the README" → "what do you think of this SQL?"
//     ⚠ **becomes one task, three turns, thirty minutes.**
//   ⚠ **With no body retained, it can never be split apart afterwards.**
//   ⚠ **The other direction still works** — T001 and T002 can be declared the same job later.
//
// ⚠ **Always record what the decision was based on** (⚠ **all of these are guesses, not observations**):
//     grouping           issue / turn
//     task_type_source   prompt_pattern (a skill name appeared in the text)
//                        issue_ref      (an issue was present, so execution was assumed)
//                        default        (nothing could be read)
//   ⚠ **Drop that field and the guess starts wearing the face of a measurement** (`.claude/rules/evidence.md`).
//
// ## Trying it by hand (⚠ without touching the real records)
//
//   D=$(mktemp -d)
//   echo '{"hook_event_name":"UserPromptSubmit","session_id":"s1","prompt_id":"p1","prompt":"hello"}' \
//     | CLAUDE_DEV_TELEMETRY_DIR=$D node .claude/hooks/telemetry.mjs; echo "exit=$?"
//   echo '{"hook_event_name":"Stop","session_id":"s1","prompt_id":"p1","last_assistant_message":"ok"}' \
//     | CLAUDE_DEV_TELEMETRY_DIR=$D node .claude/hooks/telemetry.mjs; echo "exit=$?"
//   cat $D/events.jsonl $D/tasks.jsonl
//
//   # ⚠ Confirm it never dams up the work (all exit 0, stdout empty)
//   echo 'this is not json'   | node .claude/hooks/telemetry.mjs; echo "exit=$?"
//   printf ''                 | node .claude/hooks/telemetry.mjs; echo "exit=$?"
import { appendFileSync, mkdirSync, rmdirSync, readFileSync, writeFileSync, renameSync, statSync } from "node:fs";
import { join } from "node:path";
import { execFileSync } from "node:child_process";
// ⚠ **The location lives in `.claude/telemetry-dir.mjs`, in one place.**
//   ⚠ **It used to be spelled out separately by the writer and the reader.**
//   ⚠ **Changing one side alone broke no test** (⚠ demonstrated).
import { telemetryDir, projectRoot } from "../telemetry-dir.mjs";

const LOCK_MS = 300;      // ⚠ how long to wait for the index lock. ⚠ **Past that, proceed unlocked** (never block)
const KEEP_TASKS = 500;   // ⚠ tasks retained in the index. ⚠ Oldest fall off (do not grow without bound)
const STALE_MS = 10_000;  // ⚠ a lock older than this is assumed to belong to a killed process, and is removed

// ⚠ **Exit 0 no matter what.** ⚠ Never write to stdout (it would leak into the conversation)
const bail = (why) => { if (why) process.stderr.write(`telemetry: ${why}\n`); process.exit(0); };

try {
  const IN = JSON.parse(readFileSync(0, "utf8") || "{}");
  const event = IN.hook_event_name;
  // ⚠ Only two events matter. Riding along on any other hook does nothing
  if (event !== "UserPromptSubmit" && event !== "Stop") bail();
  // ⚠ **Do not count subagents.** Their unit of work is different (inside one turn of the parent)
  if (IN.agent_id) bail();
  const sid = String(IN.session_id ?? "").trim();
  if (!sid) bail("no session_id");

  const ROOT = projectRoot();
  const DIR = telemetryDir();
  mkdirSync(DIR, { recursive: true });

  // ---- Timestamp (local time with offset — not normalised to UTC, so "when did I work" stays readable) ----
  const stamp = () => {
    const d = new Date(), off = -d.getTimezoneOffset(), s = off < 0 ? "-" : "+";
    const p = (n) => String(Math.floor(Math.abs(n))).padStart(2, "0");
    return new Date(d.getTime() + off * 60000).toISOString().slice(0, 19)
      + `${s}${p(off / 60)}:${p(off % 60)}`;
  };
  const ts = stamp();

  // ---- Identifying the task (⚠ **only from what the prompt carries**) ----
  // ⚠ **A bare number points at a different issue once the repo is migrated.**
  //   ⚠ **So the record always carries the repo name.** Only when it cannot be resolved
  //   ⚠ is the bare number kept.
  const repoOf = () => {
    try {
      const url = execFileSync("git", ["config", "--get", "remote.origin.url"],
        { cwd: ROOT, encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
      const m = /(?:[:/])([^/:]+)\/([^/]+?)(?:\.git)?$/.exec(url);
      return m ? `${m[1]}/${m[2]}` : null;
    } catch { return null; }
  };
  const issueOf = (text) => {
    const q = /([\w.-]+\/[\w.-]+)#(\d+)/.exec(text);       // owner/repo#N — take as is
    if (q) return `${q[1]}#${q[2]}`;
    const b = /(?:^|[^\w#])#(\d+)\b/.exec(text);           // bare #N — attach the repo to make it named
    if (!b) return null;
    const repo = repoOf();
    return repo ? `${repo}#${b[1]}` : `#${b[1]}`;
  };
  // ⚠ **If a skill name appears, take it.** ⚠ Otherwise decide by whether an issue is present.
  //   ⚠ **This judges the text only.** ⚠ Whether that skill actually ran is not observed.
  //
  // ⚠ **So record what the decision was based on** (`task_type_source`).
  //   ⚠ **Same principle as `grouping`.** ⚠ **Both are guesses, not observations** (`.claude/rules/evidence.md`).
  //   ⚠ **It does get this wrong**: "look into #<n> for me" is really issue_refine,
  //   ⚠ but an issue is present, so it is recorded as issue_execute.
  //   ⚠ **The reader has to be able to see that.**
  const typeOf = (text, issue) => {
    if (/issue-ready/.test(text)) return { task_type: "issue_refine", source: "prompt_pattern" };
    if (/loop-controller|issue-work/.test(text)) return { task_type: "issue_execute", source: "prompt_pattern" };
    if (issue) return { task_type: "issue_execute", source: "issue_ref" };
    return { task_type: "prompt", source: "default" };
  };

  // ---- The index (⚠ **contended, so take the lock before reading or writing**) ----
  const STATE = join(DIR, "state.json");
  const LOCK = join(DIR, ".lock");
  const nap = (ms) => { try { Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms); } catch {} };
  const withLock = (fn) => {
    let held = false;
    for (const until = Date.now() + LOCK_MS; Date.now() < until;) {
      try { mkdirSync(LOCK); held = true; break; } catch { nap(20); }
      // ⚠ **Never inherit an abandoned lock forever.** ⚠ This hook can be killed by its timeout
      //   (⚠ 5 seconds, in `settings.json`). ⚠ **A lock left behind by a killed run makes every
      //   ⚠ later run wait to the limit and then write unlocked** (= contention becomes the norm).
      try { if (Date.now() - statSync(LOCK).mtimeMs > STALE_MS) rmdirSync(LOCK); } catch {}
    }
    // ⚠ **Proceed even without the lock.** ⚠ Losing one turn to a race is better than stopping
    try { return fn(); } finally { if (held) { try { rmdirSync(LOCK); } catch {} } }
  };
  const readState = () => {
    try {
      const j = JSON.parse(readFileSync(STATE, "utf8"));
      return { version: 1, sessions: j.sessions ?? {}, tasks: j.tasks ?? {} };
    } catch { return { version: 1, sessions: {}, tasks: {} }; }   // ⚠ corrupt index must not stop the work
  };
  const writeState = (st) => {
    // ⚠ Do not grow without bound. ⚠ **Keep the newest KEEP_TASKS entries**
    const keys = Object.keys(st.tasks)
      .sort((a, b) => String(st.tasks[b].started_at).localeCompare(String(st.tasks[a].started_at)))
      .slice(0, KEEP_TASKS);
    const tasks = {};
    for (const k of keys) tasks[k] = st.tasks[k];
    const sessions = {};
    for (const [s, k] of Object.entries(st.sessions)) if (tasks[k]) sessions[s] = k;
    // ⚠ **Never let a half-written file be read** (another session reads concurrently).
    //   ⚠ Write first, then swap into place
    const tmp = `${STATE}.${process.pid}.tmp`;
    writeFileSync(tmp, JSON.stringify({ version: 1, sessions, tasks }));
    renameSync(tmp, STATE);
  };
  const put = (file, rec) => appendFileSync(join(DIR, file), `${JSON.stringify(rec)}\n`);

  const newTaskId = () => `T-${ts.slice(0, 19).replace(/[-:T]/g, "")}-`
    + Math.floor(Math.random() * 0xffff).toString(16).padStart(4, "0");

  if (event === "UserPromptSubmit") {
    // ⚠ **The body is touched only here.** ⚠ Read what is needed, then drop it. ⚠ It never reaches the record
    const text = String(IN.prompt ?? "");
    const chars = text.length;
    const issue = issueOf(text);
    const { task_type, source: task_type_source } = typeOf(text, issue);

    // ⚠ **Bundle only when there is a reason to bundle.**
    //   ⚠ **If an issue was read, it is the same task across sessions** (⚠ **there is a reason**).
    //   ⚠ **If not, one prompt is one task** (⚠ **no reason, so no bundling**).
    //
    // ⚠ **See the header**: bundling consecutive turns was a guess, and an irreversible one.
    const grouping = issue ? "issue" : "turn";
    const turnId = IN.prompt_id ?? `${ts}-${Math.random().toString(16).slice(2, 10)}`;
    const key = issue ? `${task_type}:${issue}` : `${task_type}:turn:${sid}:${turnId}`;

    const rec = withLock(() => {
      const st = readState();
      const t = st.tasks[key] ?? {
        task_id: newTaskId(), task_type, task_type_source, grouping, issue,
        started_at: ts, ended_at: null, session_ids: [], turns: 0, result: "unknown",
      };
      t.turns += 1;
      if (!t.session_ids.includes(sid)) t.session_ids.push(sid);
      st.tasks[key] = t;
      st.sessions[sid] = key;
      writeState(st);
      return t;
    });

    put("events.jsonl", {
      ts, event, session_id: sid, prompt_id: IN.prompt_id ?? null,
      task_id: rec.task_id, task_type, task_type_source, grouping, issue, turn: rec.turns,
      permission_mode: IN.permission_mode ?? null,
      prompt_chars: chars,     // ⚠ **length only.** ⚠ Neither the body nor a hash of it
    });
    bail();
  }

  // ---- Stop ----
  // ⚠ **All that was observed is that a turn ended.** ⚠ **Whether it succeeded was not observed.**
  //   ⚠ So `result` stays `unknown` (Phase 1 does not score).
  const reply = String(IN.last_assistant_message ?? "");
  const snap = withLock(() => {
    const st = readState();
    const key = st.sessions[sid];
    const t = key ? st.tasks[key] : null;
    // ⚠ **A Stop from an unknown session is not linked to a task** (happens when telemetry is
    //   ⚠ switched on mid-flight). ⚠ **Forcing a link would claim a start that was never seen**
    if (!t) return null;
    t.ended_at = ts;
    writeState(st);
    return t;
  });

  put("events.jsonl", {
    ts, event, session_id: sid, prompt_id: IN.prompt_id ?? null,
    task_id: snap?.task_id ?? null, turn: snap?.turns ?? null,
    permission_mode: IN.permission_mode ?? null,
    effort: IN.effort?.level ?? null,
    stop_hook_active: IN.stop_hook_active ?? null,
    reply_chars: reply.length,   // ⚠ **length only.** ⚠ The reply body is not kept
  });
  // ⚠ **Append only.** ⚠ The same task_id appears repeatedly. ⚠ **Readers take the last line**
  if (snap) put("tasks.jsonl", {
    ts, task_id: snap.task_id, task_type: snap.task_type, grouping: snap.grouping,
    task_type_source: snap.task_type_source ?? null,
    issue: snap.issue, started_at: snap.started_at, ended_at: snap.ended_at,
    session_ids: snap.session_ids, turns: snap.turns,
    result: snap.result,   // ⚠ **always unknown.** ⚠ Nothing is scored, so nothing else may be written
  });
} catch (e) {
  bail(e?.message ?? String(e));
}
process.exit(0);
