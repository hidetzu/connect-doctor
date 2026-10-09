# 0005 — TCP, TLS and HTTP share one connection, and HTTP is HTTP/1.1 only

Status: accepted (2026-10-10)

## Decision

- ⚠ **The TCP step's connection is the one the TLS step wraps, and the one the HTTP step writes
  its request on.** Each step is its own function with its own result, ⚠ **but there is one
  socket.**
- ⚠ **ALPN offers `http/1.1` only.** The request is written with `(*http.Request).Write` and the
  response read with `http.ReadResponse`, under the limits in DESIGN § 3.

## Why

- ⚠ **A failure is then attributable to exactly one step**, because each step starts from the
  previous step's object. No inference.
- ⚠ **Dialling again for HTTP would be a second dial** — a second chance for the address to
  differ, and a second set of timings that do not describe the first.

## Rejected

- **HTTP/2.** ⚠ **Not in the MVP.** "Does it connect" is answered by HTTP/1.1 on every server we
  are aware of that also speaks h2. ⚠ **A server that speaks only h2 would be misdiagnosed as
  `http.malformed_response`** — ⚠ **recorded as a deliberate gap in SPEC**, to revisit if seen.
- **`http.Transport` per step.** Rejected in [`0003`](0003-dial-the-address-that-was-validated-never-the-name-again.md).
