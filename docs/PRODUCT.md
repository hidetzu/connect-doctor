# PRODUCT — what ConnectDoctor is, and what it refuses to become

⚠ **This file is the single source for what the product is.** ⚠ **`CLAUDE.md`, the README and
the issues link here; they do not restate it.**
⚠ **What is implemented today is [`SPEC.md`](SPEC.md), not this file.** ⚠ **Planned is not
implemented.**

---

## 1. One sentence

> **Give ConnectDoctor one URL, and it tells you the first layer — DNS, TCP, TLS or HTTP — at
> which that URL stops connecting from our server, and what that means in one sentence.**

⚠ **The question it answers is "why does this URL not connect?"** ⚠ **Every feature is judged
against that question, and a feature that answers a different one is out.**

## 2. Who it is for

A developer who is **not** a network specialist, holding a URL that does not work, who wants to
know **which layer to go and look at** before opening `dig`, `curl -v` and `openssl s_client` in
three terminals.

⚠ **A second audience is a program**: a CLI or an AI agent that calls the JSON API and branches on
the failing layer. ⚠ **The API and the page return the same diagnosis** (§ 4, clause 10).

## 3. The user flow (MVP)

```text
1. open the page             one input box. ⚠ a line above it: "checked from ConnectDoctor's server"
2. paste a URL, submit
3. read the ladder           DNS ✅  TCP ✅  TLS ❌  HTTP —
4. read the conclusion       one sentence: which layer, what happened, what to look at next
5. (optional) open details   resolved addresses, timings, the certificate error, the status line
```

⚠ **Step 3 is the product.** ⚠ **If a user has to open details to know which layer failed, the
page has failed.**

The same flow without a browser:

```text
curl 'https://<host>/api/check?url=https://example.com'
```

## 4. What the MVP does

1. Accept one URL (`http` or `https`).
2. DNS: resolve the host from our server.
3. TCP: connect to a resolved, permitted address on the URL's port.
4. TLS: handshake and verify the certificate (`https` only).
5. HTTP: send one request and read the status line and headers.
6. Report how long each step took.
7. Name the first step that failed.
8. Say, in one short sentence, what that means.
9. Expandable detail per step.
10. ⚠ **A JSON API that returns the same result the page renders** — ⚠ **one diagnosis, two
    renderings, never two implementations** (`CLAUDE.md` § 3).

Redirects are followed, ⚠ **and every hop is a fresh URL that goes through every check again**
([`adr/0003`](adr/0003-dial-the-address-that-was-validated-never-the-name-again.md)).

## 5. What it deliberately does not do

⚠ **A gap named here is a decision.** ⚠ **Each one was cut because it answers a different
question, or because it would make the server a tool for something else.**

| Not doing | Why |
|---|---|
| Port scanning, arbitrary ports | ⚠ **Turns the server into a scanner of other people's hosts.** Only 80 and 443 ([`adr/0002`](adr/0002-only-ports-80-and-443-are-ever-dialed.md)) |
| traceroute / mtr | Answers "where on the path", not "which layer". Also needs raw sockets |
| WHOIS | Answers "who owns it" |
| Vulnerability scanning, TLS cipher grading | Answers "is it secure". SSL Labs and testssl.sh already do it well |
| Security-header audit | Answers "is it hardened" |
| Web performance | Answers "is it fast". ⚠ **Per-step timing is shown only to say where the time went before a failure** |
| Uptime monitoring | Answers "is it up over time". ⚠ **This is one observation, now** |
| Multi-location probing | ⚠ **Not in MVP.** One vantage, stated plainly (§ 6) |
| Accounts, billing | Nothing to remember about anyone |
| AI-generated diagnosis | ⚠ **The conclusion is a fixed sentence per observed outcome.** A guess dressed as a diagnosis is exactly what [`evidence.md`](../.claude/rules/evidence.md) forbids |

## 6. ⚠ The vantage point is part of every answer

⚠ **Every result is observed from ConnectDoctor's server, not from the user's machine.**

- ⚠ **"It connects from ConnectDoctor" does not mean "it connects from you"** — a corporate proxy,
  a local DNS override, a VPN or a firewall on the user's side is invisible to us.
- ⚠ **"It fails from ConnectDoctor" does not mean "it is down"** — the target may block our
  network, or route differently.
- ⚠ **"Our server" is a Cloud Run instance in Tokyo** ([`adr/0008`](adr/0008-public-exposure-is-cloud-run-in-tokyo.md)).
- ⚠ **The page and the API both say this, every time**, not in a footnote
  ([`adr/0001`](adr/0001-every-result-says-it-was-observed-from-our-server.md)).

## 7. The MVP is done when

⚠ **Each line is judged by a check, not by looking** ([`verification.md`](../.claude/rules/verification.md)).

- [ ] A URL entered on the page produces a four-row ladder and one conclusion, for each of:
      success, DNS failure, TCP failure, TLS failure, HTTP failure.
- [ ] `GET /api/check?url=` returns the same steps and conclusion as the page, for the same input.
- [ ] Every SSRF refusal in [`DESIGN.md` § 4](DESIGN.md#4-threat-model--ssrf-first) is exercised
      by a check that ⚠ **also proves nothing was dialled**.
- [ ] Redirects are followed with every hop re-validated, and a redirect to a refused address is
      refused.
- [ ] The vantage statement appears in the page and in the API response.
- [ ] `docs/SPEC.md` has a row, naming the check, for every line above.
