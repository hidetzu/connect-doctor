# 0003 — Dial the address that was validated, never the name again

Status: accepted (2026-10-10)

## Decision

1. ⚠ **The host is resolved exactly once per hop**, by our DNS step.
2. ⚠ **Every resolved address is checked by `internal/policy`.**
3. ⚠ **TCP dials the validated `netip.Addr`**, never the host name.
4. ⚠ **The dialer's `Control` hook calls the same `internal/policy` on the address the kernel is
   actually about to connect to.** ⚠ **One policy, two call sites — not two policies**
   (`CLAUDE.md` § 3).
5. ⚠ **Redirects are not followed by `net/http`.** ⚠ **Each `Location` becomes a new hop that goes
   through URL parsing, resolution and policy from the start.**

## Why

⚠ **Check-then-resolve-again is the DNS rebinding hole**: a name can answer public at check time
and `127.0.0.1` at dial time (TTL 0). ⚠ **Resolving once and dialling the address closes it.**
⚠ **The `Control` hook closes whatever we got wrong in 1–3** — it sees the real socket address.

`net/http`'s redirect follower resolves the next host itself and can only be vetoed after it has
already decided where to go. ⚠ **We want the decision before the dial, every hop.**

## Rejected

- **`http.Client` with a `CheckRedirect` and a custom `DialContext`.** It works, but the steps
  then happen inside one opaque call, and attributing a failure to DNS / TCP / TLS / HTTP needs
  `httptrace` inference. ⚠ **We would be guessing which layer failed** — the one thing the
  product must not guess. See also [`0005`](0005-tcp-tls-and-http-share-one-connection.md).
- **An HTTP proxy that enforces egress (e.g. smokescreen).** Good defence in depth, ⚠ **but it
  hides the TCP and TLS layers from us.** ⚠ **Network-level egress filtering is recommended for
  deployment in addition** (owner decision).
