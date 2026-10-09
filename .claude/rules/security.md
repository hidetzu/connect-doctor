# Security — the server fetches what a stranger names

⚠ **`MUST` = required, `SHOULD` = default, `MAY` = optional.**

⚠ **This file is an engineering constraint, not a learned pitfall** ([`README.md`](README.md)).
⚠ **It binds from the first line of code.** ⚠ **It does not wait for an incident here.**

## Grounds

⚠ **Two facts, both true before any code exists.**

1. ⚠ **The input is hostile by construction.** ⚠ **The product takes a URL from anyone and
   connects to it from our server.** That is Server-Side Request Forgery unless every step says
   otherwise (OWASP, *Server Side Request Forgery Prevention Cheat Sheet*).
2. ⚠ **The repository is public.** ⚠ **The defences are readable by the attacker**, so they must
   hold when known ([`git.md`](git.md) owns what never goes public).

⚠ **The threat list is [`docs/DESIGN.md` § 4](../../docs/DESIGN.md#4-threat-model--ssrf-first).
⚠ The why of each boundary is [`docs/adr/`](../../docs/adr/). ⚠ This file is the rules.**
⚠ **Never weaken a clause here on your own judgement** — ⚠ **an owner decision, every time**
([`owner-decisions.md`](owner-decisions.md)).

---

## 1. One policy, enforced at the dial

- MUST: ⚠ **Exactly one function decides whether an address may be dialled**
  (`internal/policy`). ⚠ **Never a second list, a regexp on the host string, or a copy in a test.**
- MUST: ⚠ **Fail closed.** ⚠ **An address the policy does not positively recognise as globally
  reachable is refused.** ⚠ **Grounds: IANA Special-Purpose Address Registries (RFC 6890 § 2.2.2);
  where the registry is silent, refuse.**
- MUST: ⚠ **Unmap IPv4-mapped IPv6 before deciding.** ⚠ **`::ffff:127.0.0.1` is loopback.**
- MUST: ⚠ **Every dial goes through a `net.Dialer` whose `Control` hook calls that policy on the
  address actually being connected** ([`adr/0003`](../../docs/adr/0003-dial-the-address-that-was-validated-never-the-name-again.md)).
- MUST NOT: ⚠ **Never dial a host name.** ⚠ **Dial the `netip.Addr` that was validated.**
- MUST NOT: ⚠ **Never use `http.Get`, `http.DefaultClient`, `http.DefaultTransport`, or
  `net.Dial` in product code.** ⚠ **Each of them resolves and dials on its own.**
  ⚠ **Hold this with a check, not with care.**

## 2. What is refused before anything leaves

- MUST: ⚠ **Only `http` and `https`.**
- MUST: ⚠ **Only ports 80 and 443** ([`adr/0002`](../../docs/adr/0002-only-ports-80-and-443-are-ever-dialed.md)).
- MUST: ⚠ **Refuse userinfo** (`user:pass@`). ⚠ **Never send it, never echo it.**
- MUST: ⚠ **Refuse host names that only mean something locally**: `localhost` and `*.localhost`
  (RFC 6761 § 6.3), `*.local` (RFC 6762), `*.home.arpa` (RFC 8375), `*.internal`, single-label
  names. ⚠ **And still check every resolved address** — ⚠ **the name list is an early answer,
  not the defence.**
- MUST: ⚠ **Refuse a host that looks numeric and is not a canonical IP literal**
  (`2130706433`, `0x7f.1`, `0177.0.0.1`). ⚠ **Grounds: WHATWG URL § host parsing treats a final
  numeric label as IPv4; resolvers disagree on these forms; refusing is the only reading that does
  not depend on which one runs.**

## 3. Every hop is a new URL

- MUST: ⚠ **A redirect `Location` goes through § 2 and § 1 from the start.**
  ⚠ **Never through `net/http`'s redirect follower.**
- MUST: ⚠ **Bound the number of hops, and the time for all hops together** (`internal/limits`).

## 4. Bounded, always

- MUST: ⚠ **Every wait has a deadline, and the whole check has one ceiling, enforced by one
  `context.Context`.** ⚠ **A step without its own deadline is a bug.**
- MUST: ⚠ **Bound what is read**: response headers, response body, URL length.
- MUST: ⚠ **Bound concurrent checks per process.** ⚠ **Reaching the bound is counted, and answered
  as "busy", ⚠ never as a failure of the target.**
- MUST: ⚠ **Every limit lives in `internal/limits`.** ⚠ **A number written inline elsewhere is a
  second implementation of the same question.**

## 5. What is shown, and what is written down

- MUST NOT: ⚠ **Never show an address that policy refused**, in the page or the API
  ([`adr/0004`](../../docs/adr/0004-a-name-with-any-non-public-address-is-refused-whole.md)).
  ⚠ **Say that it was refused, not what it was.**
- MUST NOT: ⚠ **Never log a URL's query string, fragment, or userinfo.**
  ⚠ **URLs carry tokens.** ⚠ **Redact at the one place that builds the log line.**
- MUST: ⚠ **A refusal is our policy, and the wording says so** (`CLAUDE.md` § 4-1).
  ⚠ **Never "the site is unreachable" for something we declined to reach.**

## 6. Proving it

- MUST: ⚠ **Every refusal in § 1–§ 3 has a check that also proves nothing was dialled**
  ([`verification.md`](verification.md) § An exercise must not change the world):
  ⚠ **a listener the refused request would have reached, and did not**,
  ⚠ **paired with a control that does reach a listener** — ⚠ **otherwise the check passes when
  nothing starts at all.**
- MUST NOT: ⚠ **Never give the product a switch that widens the policy for tests.**
  ⚠ **A test may touch less, never more.** ⚠ **To test against a local listener, put the listener
  on an address the policy permits** (a network namespace), ⚠ **not the policy around the
  listener.**
