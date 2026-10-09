// Check that the Slack setup is correct, before relying on it.
//
// ⚠ Posts nothing. Read-only (only --post sends a single message).
// ⚠ Never prints a token value. First few characters and the length, nothing else.
// ⚠ Zero dependencies — node's built-in fetch and WebSocket only (Bolt is not needed).
//
//   node .claude/hooks/slack-doctor.mjs            inspect the setup only
//   node .claude/hooks/slack-doctor.mjs --post     post one sample with buttons and wait for a press
//   node .claude/hooks/slack-doctor.mjs --modal    post one sample and check free text via a modal
//
// Read (env vars; failing that, **only that one line** of this repo's .envrc / .env):
//   SLACK_APP_TOKEN    xapp-…  Socket Mode (connections:write)
//   SLACK_BOT_TOKEN    xoxb-…  posting (chat:write)
//   SLACK_CHANNEL_ID   C…      where to post
// ⚠ SLACK_WEBHOOK_URL is no longer used. If it is still there, say so and tell them to delete it.
import { readFileSync, existsSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = process.env.CLAUDE_PROJECT_DIR
  ?? join(dirname(fileURLToPath(import.meta.url)), "..", "..");

// ⚠ Never source it. Read only the line for this one variable (.envrc is arbitrary shell code)
const fromEnvFile = (name) => {
  for (const f of [join(ROOT, ".envrc"), join(ROOT, ".env")]) {
    if (!existsSync(f)) continue;
    // ⚠ Horizontal whitespace only ([ \t], not \s). ⚠ \s crosses newlines, so an empty
    //   assignment (NAME=) would take the next non-empty line as the value — reading
    //   "missing" as "set, to the wrong thing", and the terminal fallback never fires.
    const m = new RegExp(`^[ \\t]*(?:export[ \\t]+)?${name}[ \\t]*=[ \\t]*([^\\n]*)$`, "m")
      .exec(readFileSync(f, "utf8"));
    if (!m) continue;
    let v = m[1].trim();
    if (/^".*"$/.test(v) || /^'.*'$/.test(v)) v = v.slice(1, -1);
    else v = v.replace(/\s+#.*$/, "");
    if (v) return v;
  }
  return null;
};
const get = (name) => process.env[name] || fromEnvFile(name);

// ⚠ Never print the value. Shape only
const shape = (v) => (v ? `${v.slice(0, 5)}… (${v.length} chars)` : "missing");

const call = async (method, token, body) => {
  const r = await fetch(`https://slack.com/api/${method}`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": body ? "application/json; charset=utf-8" : "application/x-www-form-urlencoded",
    },
    body: body ? JSON.stringify(body) : "",
  });
  return r.json();
};

// The ways this goes wrong are few and known. Translate Slack's terse errors into what to do
const HINT = {
  invalid_auth: "wrong token (mis-pasted, or an old one after a reinstall)",
  not_allowed_token_type: "⚠ wrong token type. apps.connections.open takes xapp- (not xoxb-)",
  missing_scope: "missing permission. add the scope and **reinstall** (adding alone does nothing)",
  not_in_channel: "⚠ the bot was never invited to the channel. `/invite @app-name`",
  channel_not_found: "wrong channel id (the C… at the bottom of the channel details, verbatim)",
  account_inactive: "the app has been uninstalled",
};

const line = (ok, label, detail) =>
  console.log(`  ${ok ? "\x1b[32m✓\x1b[0m" : "\x1b[31m✗\x1b[0m"} ${label}${detail ? " — " + detail : ""}`);

const APP = get("SLACK_APP_TOKEN"), BOT = get("SLACK_BOT_TOKEN"), CH = get("SLACK_CHANNEL_ID");
const HOOK = get("SLACK_WEBHOOK_URL");

console.log("\n\x1b[1m1. What is present locally\x1b[0m (⚠ values are never printed)");
// ⚠ Webhooks are **no longer used** (chat.postMessage is a superset).
//   If one is still around, it means an unused secret is lying there. Say so.
if (HOOK) line(false, "SLACK_WEBHOOK_URL", "⚠ still present. No longer used — delete it (never keep an unused secret)");
line(!!APP, "SLACK_APP_TOKEN", shape(APP));
line(!!BOT, "SLACK_BOT_TOKEN", shape(BOT));
line(!!CH, "SLACK_CHANNEL_ID", CH ?? "missing");
if (APP && !APP.startsWith("xapp-")) line(false, "SLACK_APP_TOKEN shape", "⚠ does not start with xapp-");
if (BOT && !BOT.startsWith("xoxb-")) line(false, "SLACK_BOT_TOKEN shape", "⚠ does not start with xoxb-");

let fatal = 0;
const need = (v, what) => { if (!v) { console.log(`\n  stopping here: ${what}`); fatal++; } return !!v; };

if (need(BOT, "no SLACK_BOT_TOKEN, so nothing further can be checked")) {
  console.log("\n\x1b[1m2. Can it identify itself as the bot\x1b[0m (auth.test)");
  const a = await call("auth.test", BOT).catch((e) => ({ ok: false, error: String(e) }));
  line(a.ok, "auth.test", a.ok ? `${a.team} / ${a.user}` : `${a.error} — ${HINT[a.error] ?? ""}`);
  if (!a.ok) fatal++;
}

if (APP) {
  console.log("\n\x1b[1m3. Is Socket Mode enabled\x1b[0m (apps.connections.open)");
  const c = await call("apps.connections.open", APP).catch((e) => ({ ok: false, error: String(e) }));
  line(c.ok, "apps.connections.open", c.ok ? "got a WebSocket URL" : `${c.error} — ${HINT[c.error] ?? "Socket Mode may be OFF"}`);
  if (c.ok) {
    // ⚠ Actually connect and wait for hello. Getting a URL does not prove the connection works
    const ws = new WebSocket(c.url);
    const hello = await new Promise((res) => {
      const t = setTimeout(() => res(null), 10000);
      ws.onmessage = (e) => { const m = JSON.parse(e.data);
        if (m.type === "hello") { clearTimeout(t); res(m); } };
      ws.onerror = () => { clearTimeout(t); res(null); };
    });
    line(!!hello, "WebSocket connects", hello ? `received hello (${hello.num_connections} connections)` : "⚠ no hello after 10 seconds");
    if (!hello) fatal++;
    ws.close();
  } else fatal++;
} else console.log("\n\x1b[1m3. Socket Mode\x1b[0m — not checked (no SLACK_APP_TOKEN)");

if (process.argv.includes("--post") && BOT && CH && APP) {
  console.log("\n\x1b[1m4. Show buttons and wait for a press\x1b[0m (⚠ really posts one message)");
  const OPTS = ["Go with A", "Go with B", "Neither"];
  const p = await call("chat.postMessage", BOT, {
    channel: CH,
    text: "(sample) checking whether an answer can come back from Slack",
    blocks: [
      { type: "section", text: { type: "mrkdwn", text: "*(sample) Can an answer come back from Slack?*" } },
      { type: "actions", elements: OPTS.map((o, i) => ({
        type: "button", action_id: `doctor_${i}`, value: o, text: { type: "plain_text", text: o } })) },
    ],
  });
  line(p.ok, "chat.postMessage", p.ok ? `posted (ts=${p.ts})` : `${p.error} — ${HINT[p.error] ?? ""}`);
  if (!p.ok) fatal++;
  else {
    const c = await call("apps.connections.open", APP);
    const ws = new WebSocket(c.url);
    console.log("     press a button in Slack (waiting 60 seconds)…");
    const got = await new Promise((res) => {
      const t = setTimeout(() => res(null), 60000);
      ws.onmessage = (e) => {
        const m = JSON.parse(e.data);
        if (!m.envelope_id) return;
        // ⚠ Slack redelivers if not acked within 3 seconds
        ws.send(JSON.stringify({ envelope_id: m.envelope_id }));
        const act = m.payload?.actions?.[0];
        // ⚠ Only react to the message this run posted
        // ⚠ Who pressed it is not inspected (no names in the output)
        if (act && m.payload?.message?.ts === p.ts) { clearTimeout(t); res({ act }); }
      };
      ws.onerror = () => { clearTimeout(t); res(null); };
    });
    ws.close();
    line(!!got, "button events arrive",
      got ? `“${got.act.value}” was pressed` : "⚠ nothing after 60 seconds (Interactivity may be OFF)");
    if (!got) fatal++;
    // ⚠ Only a value matching an option we offered counts as an answer
    if (got) line(OPTS.includes(got.act.value), "the pressed value matches an offered option",
      OPTS.includes(got.act.value) ? "matches" : "⚠ does not match (not accepted as an answer)");
  }
} else if (process.argv.includes("--post")) {
  console.log("\n\x1b[1m4.\x1b[0m — not attempted (the three values are not all present)");
}

// ⚠ How free text is received. **Measure whether a modal works, rather than reading thread replies.**
//   Reading a thread needs channels:history = **every message in that channel reaches the bot**.
//   With a modal, only what that person wrote, in answer to our question, arrives.
//   ⚠ Whether extra scopes are required is not assumed — **it is called and observed**.
if (process.argv.includes("--modal") && BOT && CH && APP) {
  console.log("\n\x1b[1m5. Can free text be received through a modal\x1b[0m (⚠ really posts one message)");
  const p2 = await call("chat.postMessage", BOT, {
    channel: CH,
    text: "(sample) checking free-text answers",
    blocks: [
      { type: "section", text: { type: "mrkdwn", text: "*(sample) Can you write freely?*" } },
      { type: "actions", elements: [
        { type: "button", action_id: "doctor_free", value: "__free__",
          text: { type: "plain_text", text: "Write freely" } }] },
    ],
  });
  line(p2.ok, "chat.postMessage", p2.ok ? `posted (ts=${p2.ts})` : `${p2.error} — ${HINT[p2.error] ?? ""}`);
  if (!p2.ok) fatal++;
  else {
    const c2 = await call("apps.connections.open", APP);
    const ws2 = new WebSocket(c2.url);
    console.log("     press “Write freely”, type something into the box and send (waiting 90 seconds)…");
    const out = await new Promise((res) => {
      const t = setTimeout(() => res({ err: "timed out" }), 90000);
      ws2.onmessage = async (e) => {
        const m = JSON.parse(e.data);
        if (!m.envelope_id) return;
        const pl = m.payload;
        // (1) button pressed -> open the modal right away (trigger_id expires in 3 seconds)
        if (pl?.type === "block_actions" && pl.message?.ts === p2.ts) {
          ws2.send(JSON.stringify({ envelope_id: m.envelope_id }));
          const v = await call("views.open", BOT, {
            trigger_id: pl.trigger_id,
            view: { type: "modal", callback_id: "doctor_modal",
              title: { type: "plain_text", text: "Write freely" },
              submit: { type: "plain_text", text: "Send" },
              blocks: [{ type: "input", block_id: "b", label: { type: "plain_text", text: "Answer" },
                element: { type: "plain_text_input", action_id: "a", multiline: true } }] },
          });
          if (!v.ok) { clearTimeout(t); res({ err: `views.open: ${v.error}` }); }
          return;
        }
        // (2) modal submitted
        if (pl?.type === "view_submission" && pl.view?.callback_id === "doctor_modal") {
          ws2.send(JSON.stringify({ envelope_id: m.envelope_id, payload: { response_action: "clear" } }));
          clearTimeout(t);
          res({ text: pl.view.state.values.b.a.value });
          return;
        }
        ws2.send(JSON.stringify({ envelope_id: m.envelope_id }));
      };
      ws2.onerror = () => { clearTimeout(t); res({ err: "WebSocket dropped" }); };
    });
    ws2.close();
    line(!out.err, "free text arrives through the modal",
      out.err ? `⚠ ${out.err}` : `${out.text.length} chars written: “${out.text.slice(0, 40)}”`);
    if (out.err) fatal++;
    else line(true, "⚠ no extra scope was required", "the modal opened with chat:write alone");
  }
}

console.log(fatal ? `\n\x1b[31m${fatal} problem(s)\x1b[0m\n` : "\n\x1b[32mall good\x1b[0m\n");
process.exit(0);
