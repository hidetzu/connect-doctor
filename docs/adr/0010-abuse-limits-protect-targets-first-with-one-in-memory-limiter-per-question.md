# 0010 — Abuse limits protect the targets first, with one in-memory limiter per question

Status: accepted (2026-10-10). Owner decisions on hidetzu/connect-doctor#6, from a reviewed
proposal with the AI's amendments.

## Decision

Four layers, built in this order (one issue each):

| | What | Values | Issue |
|---|---|---|---|
| 1 | ⚠ **Per-check budgets** | whole check 10 s (DNS 2, TCP 3/4, TLS 3, HTTP 4); 3 redirects; 8 checks at once per instance | hidetzu/connect-doctor#26 |
| 2 | **Per client** (IPv4 address, IPv6 /64; the last `X-Forwarded-For` entry) | burst 3 then 5/min; 30/hour; 100/day (⚠ best effort); 1 at a time. Global breaker 3,000/hour per instance | hidetzu/connect-doctor#6 |
| 3 | ⚠ **Per target** | hostname 6/min (burst 2); destination IP + port 30/min (burst 5), ⚠ **counted per TCP attempt** | hidetzu/connect-doctor#27 |
| 4 | **Cache** | whole result per normalised URL, 30 s; served instead of 429 when the target limit refuses | hidetzu/connect-doctor#28 |

Over a limit: `429` + `Retry-After`, a `server.*` code, neutral, worded as ConnectDoctor's limit.
⚠ **Logged only when a limit fires**: the target hostname and the client prefix (IPv4 /24,
IPv6 /48). ⚠ **Never path, query, or a full address.** Ordinary use is not logged.

## Why

- ⚠ **The target limits are the ones that matter.** Client limits fail against many attacker
  addresses aimed at one victim; the target limits hold however many clients there are.
- ⚠ **Counting per TCP attempt**: a check can try several addresses; counting per check would let
  a name with many addresses dodge the IP limit.
- ⚠ **The global breaker**: client limits do not bound total cost when every request comes from a
  fresh address; one instance at 8 concurrent short checks could still run hundreds a minute.

## ⚠ What these limits do not do

- ⚠ **They live in one process's memory.** A replaced instance starts from zero, and Cloud Run may
  briefly run more than one instance despite `--max-instances 1`. ⚠ **So "100 per day" is best
  effort**, and every limit is per instance. ⚠ **Enforcing them exactly needs shared state — a cost
  and an ADR of its own.**
- ⚠ **They are not the SSRF defence.** The policy, the dial hook and the per-hop re-validation
  (`docs/adr/0003`, `.claude/rules/security.md`) do that, with or without limits.

## Rejected

- **Per-layer caches** (DNS/TCP/TLS 60 s, HTTP 30 s, as proposed): two caches answering one
  question drift apart (`CLAUDE.md` § 3). One cache of the whole result instead.
- **Client limits alone**: rejected above.
- **No logging at all**: a report of abuse could not be investigated. The logging is limited to
  the moment a limit fires.
- **A shared store (Memorystore, Firestore) for exact limits**: a running cost the owner has not
  accepted (`docs/adr/0008`).
