#!/usr/bin/env node
// When a human decision is needed (AskUserQuestion), ask in Slack and bring the answer back.
//
// ⚠ **This is a round trip, not a notification.** Where the line between "decide yourself" and
//   "ask" sits is in `.claude/rules/owner-decisions.md`. ⚠ It is not repeated here (never hold one spec in two places).
//
// ⚠ **Becoming unable to ask a human is the one outcome to avoid.**
//   A PreToolUse hook that exits non-zero cancels the tool call itself. A missing token,
//   Slack being down, a timeout, or a bug in here must never turn into "cannot ask".
//   → **Exit 0 whatever happens.** When no answer comes back, emit nothing, which falls
//   back to asking in the terminal exactly as usual.
//
// ⚠ **The wait is bounded.** 3 minutes by default, inside the hook's own timeout.
//
// Two ways to answer. **Both are reachable only by people already in the channel**, so who may
// answer is decided by Slack's channel settings — no allow-list is kept here.
//
//   buttons        … accepted **only when they match an option we ourselves offered**
//   ✎ free text    … received through a modal (the same thing the terminal's "Other" does)
//
// ⚠ **Replies typed directly into the thread are not read.** Reading them needs
//   channels:history / groups:history, which delivers *every* message in that channel to the bot.
//   We do not read everything in order to receive one answer.
//   ⚠ Since they are not read, they cannot be noticed either. ⚠ **So on timeout, post one line
//     back into the thread** — never leave someone who wrote there thinking it arrived.
//
// Required (env vars; failing that, **only that one line** of this repo's .envrc / .env):
//   SLACK_APP_TOKEN   xapp-…  Socket Mode (connections:write)
//   SLACK_BOT_TOKEN   xoxb-…  posting and modals (chat:write)
//   SLACK_CHANNEL_ID  C…      where to post
//
// ⚠ Exactly two scopes are needed: chat:write and connections:write (measured in konjaku,
//   2026-08-18). No history, no channel info. Confirmed to work in a private channel.
// ⚠ Incoming Webhooks are not used. chat.postMessage is a superset, and keeping both would
//   mean two ways out to Slack (rule: never two implementations of the same question).
//
// Trying it by hand:
//   node .claude/hooks/slack-doctor.mjs                   inspect the setup (posts nothing)
//   node .claude/hooks/slack-doctor.mjs --post --modal    exercise buttons and the modal for real
//
//   # ⚠ End to end as a hook (⚠ this really posts once, and really waits 3 minutes)
//   echo '{"tool_name":"AskUserQuestion","cwd":"'"$PWD"'","session_id":"t","tool_input":{"questions":[
//     {"question":"Which way?","header":"Approach","options":[{"label":"Plan A"},{"label":"Plan B"}]}]}}' \
//     | node .claude/hooks/ask-slack.mjs; echo "exit=$?"
//
//   # ⚠ Confirm it never dams up the question (all exit 0, emitting nothing)
//   echo 'this is not json'              | node .claude/hooks/ask-slack.mjs; echo "exit=$?"
//   echo '{"tool_name":"Bash"}'          | node .claude/hooks/ask-slack.mjs; echo "exit=$?"
//   printf ''                            | node .claude/hooks/ask-slack.mjs; echo "exit=$?"
import { readFileSync, existsSync } from "node:fs";
import { join, dirname, basename } from "node:path";
import { fileURLToPath } from "node:url";
import { execFileSync } from "node:child_process";

const WAIT_MS = 180_000;          // ⚠ the ceiling. Never wait past it
const FREE = "__free__";          // marker for "✎ write freely"
const bail = (why) => { if (why) process.stderr.write(`ask-slack: ${why}\n`); process.exit(0); };

try {
  const INPUT = JSON.parse(readFileSync(0, "utf8") || "{}");
  // ⚠ AskUserQuestion only. Never stall any other tool
  if (INPUT.tool_name && INPUT.tool_name !== "AskUserQuestion") bail();
  const questions = INPUT.tool_input?.questions ?? [];
  if (!questions.length) bail("no questions");

  const ROOT = process.env.CLAUDE_PROJECT_DIR
    ?? join(dirname(fileURLToPath(import.meta.url)), "..", "..");
  // ⚠ Never source it. .envrc is arbitrary shell code — reading it that way runs anything
  const fromFile = (name) => {
    for (const f of [join(ROOT, ".envrc"), join(ROOT, ".env")]) {
      if (!existsSync(f)) continue;
      // ⚠ Horizontal whitespace only ([ \t], not \s). ⚠ \s crosses newlines, so an empty
      //   assignment (NAME=) would take the next non-empty line as the value — reading
      //   "missing" as "set, to the wrong thing", and the terminal fallback never fires.
      const m = new RegExp(`^[ \\t]*(?:export[ \\t]+)?${name}[ \\t]*=[ \\t]*([^\\n]*)$`, "m").exec(readFileSync(f, "utf8"));
      if (!m) continue;
      let v = m[1].trim();
      if (/^".*"$/.test(v) || /^'.*'$/.test(v)) v = v.slice(1, -1); else v = v.replace(/\s+#.*$/, "");
      if (v) return v;
    }
    return null;
  };
  const env = (n) => process.env[n] || fromFile(n);
  const APP = env("SLACK_APP_TOKEN"), BOT = env("SLACK_BOT_TOKEN"), CH = env("SLACK_CHANNEL_ID");
  if (!APP || !BOT || !CH) bail("no Slack setup; asking in the terminal");

  // Where the work is happening. ⚠ Derived from cwd every time (a fixed string lies in another clone)
  const CWD = INPUT.cwd ?? "";
  let PROJECT = basename(CWD || "unknown");
  try {
    const top = execFileSync("git", ["-C", CWD, "rev-parse", "--show-toplevel"],
      { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
    if (top) { const rel = CWD.slice(top.length).replace(/^\//, ""); PROJECT = basename(top) + (rel ? ` / ${rel}` : ""); }
  } catch { /* no git, or not a repo. keep the basename */ }

  const head = `🤖 *Claude Code is waiting on a decision*  \`${PROJECT}\``;
  const plain = questions.map((q, i) => `${i + 1}. ${q.question ?? ""}`).join("\n");

  const api = async (method, token, payload) => {
    const r = await fetch(`https://slack.com/api/${method}`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json; charset=utf-8" },
      body: JSON.stringify(payload ?? {}),
      signal: AbortSignal.timeout(10_000),
    });
    return r.json();
  };

  // ---- One row of option buttons per question, plus "✎ write freely" ----
  // ⚠ Button text caps at 75 chars. It is truncated for display, but **the untruncated label is
  //   what gets adopted as the answer**
  const optionsOf = (q) => (q.options ?? [])
    .map((o) => (typeof o === "string" ? o : o?.label)).filter(Boolean);
  const blocks = [{ type: "section", text: { type: "mrkdwn", text: head } }];
  questions.forEach((q, qi) => {
    blocks.push({ type: "section", text: { type: "mrkdwn",
      text: `*${q.header ? `[${q.header}] ` : ""}${q.question ?? ""}*` } });
    const els = optionsOf(q).slice(0, 4).map((label, oi) => ({
      type: "button", action_id: `q${qi}_o${oi}`, value: `${qi}:${oi}`,
      text: { type: "plain_text", text: label.slice(0, 75) },
    }));
    els.push({ type: "button", action_id: `q${qi}_free`, value: `${qi}:${FREE}`,
      text: { type: "plain_text", text: "✎ write freely" } });
    blocks.push({ type: "actions", block_id: `a${qi}`, elements: els });
  });

  const post = await api("chat.postMessage", BOT, { channel: CH, text: `${head}\n${plain}`, blocks });
  if (!post.ok) bail(`could not post (${post.error}); asking in the terminal`);

  const conn = await api("apps.connections.open", APP);
  if (!conn.ok) bail(`could not open Socket Mode (${conn.error}); asking in the terminal`);

  // ---- Wait, bounded ----
  const answers = new Array(questions.length).fill(null);
  const ws = new WebSocket(conn.url);
  const result = await new Promise((done) => {
    const timer = setTimeout(() => done(null), WAIT_MS);
    const finish = (v) => { clearTimeout(timer); done(v); };
    ws.onerror = () => finish(null);
    ws.onclose = () => finish(null);
    ws.onmessage = async (ev) => {
      let m; try { m = JSON.parse(ev.data); } catch { return; }
      if (!m.envelope_id) return;
      const pl = m.payload;
      // ⚠ Slack redelivers if not acked within 3 seconds. Ack before deciding anything
      const ack = (payload) => ws.send(JSON.stringify({ envelope_id: m.envelope_id, ...(payload ? { payload } : {}) }));

      if (pl?.type === "block_actions" && pl.message?.ts === post.ts) {
        ack();
        const act = pl.actions?.[0];
        const [qi, oi] = String(act?.value ?? "").split(":");
        const q = questions[Number(qi)];
        if (!q) return;
        if (oi === FREE) {
          // ⚠ trigger_id expires in 3 seconds. Open the modal immediately after the ack
          const v = await api("views.open", BOT, {
            trigger_id: pl.trigger_id,
            view: { type: "modal", callback_id: `ask_${qi}`,
              private_metadata: String(qi),
              title: { type: "plain_text", text: "Write freely" },
              submit: { type: "plain_text", text: "Send" },
              blocks: [{ type: "input", block_id: "b",
                label: { type: "plain_text", text: (q.question ?? "").slice(0, 2000) },
                element: { type: "plain_text_input", action_id: "a", multiline: true } }] },
          });
          if (!v.ok) process.stderr.write(`ask-slack: could not open the modal (${v.error})\n`);
          return;
        }
        // ⚠ Adopted only when it matches an option we ourselves offered
        const label = optionsOf(q)[Number(oi)];
        if (label == null) return;
        answers[Number(qi)] = label;
        if (answers.every((a) => a !== null)) finish(answers);
        return;
      }

      if (pl?.type === "view_submission" && /^ask_\d+$/.test(pl.view?.callback_id ?? "")) {
        ack({ response_action: "clear" });
        const qi = Number(pl.view.private_metadata);
        const text = pl.view.state?.values?.b?.a?.value ?? "";
        if (!questions[qi] || !text.trim()) return;
        answers[qi] = text.trim();
        if (answers.every((a) => a !== null)) finish(answers);
        return;
      }
      ack();
    };
  });
  try { ws.close(); } catch { /* cannot close; no longer relevant */ }

  // ---- Timed out. ⚠ Do not go quiet. Tell whoever wrote there that it did not arrive ----
  if (!result) {
    await api("chat.postMessage", BOT, { channel: CH, thread_ts: post.ts,
      text: "⏱ No answer came back, so the question moved to the terminal. "
          + "(⚠ Replies typed directly into this thread are not read — please use a button or “✎ write freely”.)",
    }).catch(() => {});
    bail("timed out; asking in the terminal");
  }

  // ---- Hand the answer back to Claude ----
  // ⚠ Do not leave the message still looking clickable (prevents double answers and confusion)
  await api("chat.update", BOT, { channel: CH, ts: post.ts,
    text: `${head}\n${plain}`,
    blocks: [{ type: "section", text: { type: "mrkdwn", text: head } },
      ...questions.map((q, i) => ({ type: "section", text: { type: "mrkdwn",
        text: `*${q.question ?? ""}*\n→ ${result[i]}` } })),
      { type: "context", elements: [{ type: "mrkdwn", text: "✅ answered" }] }],
  }).catch(() => {});

  const said = questions.map((q, i) => `• ${q.question ?? ""} → ${result[i]}`).join("\n");
  // ⚠ **Never let the same question be asked again.** Without saying so, the tool gets called
  //   again and it loops forever.
  // ⚠ **Who answered is not included.** Only the answer itself.
  //   Names and ids would scatter through the record (transcript, logs, PR bodies).
  //   ⚠ Who pressed the button has no bearing on whether the answer is right. Slack has it if needed.
  const reason = `An answer came back from Slack.\n${said}\n`
    + `Treat this as the user's answer and continue without calling AskUserQuestion again.`;
  process.stdout.write(JSON.stringify({
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "deny",       // ⚠ The tool does not run. Only the reason (= the answer) comes back
      permissionDecisionReason: reason,
    },
  }));
  process.exit(0);
} catch (e) {
  // ⚠ Whatever happens, never dam up the question
  bail(`crashed; asking in the terminal (${e?.message ?? e})`);
}
