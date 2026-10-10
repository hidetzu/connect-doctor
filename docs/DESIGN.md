# DESIGN — what each layer can show, what can go wrong, and the shape of the code

⚠ **This is the design the MVP is built against.** ⚠ **Why each boundary sits where it does is
in [`adr/`](adr/); the binding security constraints are [`.claude/rules/security.md`](../.claude/rules/security.md).**
⚠ **What is implemented is [`SPEC.md`](SPEC.md).** ⚠ **This file is a plan until SPEC says otherwise.**

---

## 1. Steps and outcomes

Each step ends in exactly one **status**, and a failed or refused step carries one **outcome
code**. ⚠ **The code is stable and machine-readable; the sentence is for people.**

| Status | Meaning | ⚠ Must not be confused with |
|---|---|---|
| `ok` | The step completed | — |
| `failed` | ⚠ **The target, or the path to it, did not let the step complete** | `refused` |
| `refused` | ⚠ **ConnectDoctor declined to perform the step** (policy) | ⚠ **Never phrased as the target's fault** (`CLAUDE.md` § 4-1) |
| `skipped` | Not attempted, because an earlier step did not succeed | `failed` |
| `not_applicable` | Does not exist for this URL (TLS on `http://`) | `skipped` |
| `not_implemented` | ⚠ **We have not built this step yet** | ⚠ **Never shown as `—` alone; never as a failure** |

⚠ **`evidence.md` § Outcomes** — which of its six can occur here:

| evidence.md outcome | Here |
|---|---|
| Accepted and handled | `ok` |
| Malformed | `input.malformed` — the URL itself cannot be parsed |
| Well-formed but unsupported | `refused` — scheme, port, credentials, non-public address |
| Not implemented yet | `not_implemented` |
| Nothing arrived | `tcp.timeout`, `tls.timeout`, `http.timeout` — ⚠ **"no answer", never "it is down"** |
| A timer expired | ⚠ **Same as above here: every wait is bounded by a timer, so these two collapse into `*.timeout`** |

### Input refusals (before any step)

⚠ **Nothing is attempted: every step is `skipped`, `conclusion.failed_step` is absent, and `url`
is not echoed** (the input may carry credentials, T9).

| Code | When |
|---|---|
| `input.missing` | Empty |
| `input.malformed` | Not parseable; no scheme; an IP in a non-canonical spelling (T2); over the length cap |
| `input.unsupported_scheme` | Not `http` / `https` (T13) |
| `input.unsupported_port` | Not exactly `80` / `443` (T8) |
| `input.credentials` | Userinfo present (T9) |
| `input.local_name` | Single-label, `localhost`, `.local`, `.home.arpa`, `.internal` (T3) |
| `input.refused_address` | An IP literal the policy refuses (T1) |
| `input.idn_not_implemented` | ⚠ **Non-ASCII host. Our gap, worded as ours** — punycode needs a module we do not take |

An IP literal the policy permits has no DNS step: DNS is `not_applicable`, with the address in
its detail.

## 2. What each layer can actually observe (Go standard library)

⚠ **"Observable" means: distinguishable from our server with the standard library, without raw
sockets.** ⚠ **Anything else is not claimed.**

### DNS (`net.Resolver`, pure-Go resolver)

| Outcome code | Observed as | Conclusion points at |
|---|---|---|
| `ok` | One or more A / AAAA addresses | — |
| `dns.not_found` | `*net.DNSError` with `IsNotFound` — ⚠ **NXDOMAIN or NODATA, which the standard library does not tell apart** | ⚠ **The name does not exist, or has no A/AAAA record** — spelling, or the record was never created |
| `dns.timeout` | `IsTimeout` | ⚠ **No answer from the resolver** — not "the name is wrong" |
| `dns.server_failure` | Any other `*net.DNSError` (SERVFAIL, refused, …) | The name's DNS is misconfigured or its servers are failing |
| `dns.refused_address` | ⚠ **Resolved, and at least one address is not public** | ⚠ **Refused by us**. ⚠ **The address itself is never shown** (§ 4, T5) |

⚠ **Measured, 2026-10-10, go1.26.2 linux/amd64, pure-Go resolver against `internal/dnstest`:**
⚠ **an NXDOMAIN answer and a NOERROR answer with zero records both return `*net.DNSError{Err: "no
such host", IsNotFound: true}`.** ⚠ **So "the name does not exist" and "the name has no address"
are one outcome here**, and the wording says both. ⚠ **Telling them apart needs our own DNS
client — an ADR first (`adr/0006`).**

⚠ **Names are resolved as absolute (`host + "."`).** ⚠ **Otherwise the resolver tries the server's
own search domains, and `intranet.corp` becomes `intranet.corp.<our-domain>` — a lookup into our
network chosen by a stranger** (§ 4, T3).

⚠ **Not observable, so not claimed:** which authoritative server failed, DNSSEC validation
state, whether the user's own resolver agrees (§ 4 of [`PRODUCT.md`](PRODUCT.md) — vantage).

### TCP (`net.Dialer` to the validated address)

| Outcome code | Observed as |
|---|---|
| `ok` | Three-way handshake completed |
| `tcp.refused` | `ECONNREFUSED` — ⚠ **something answered, with a reset**: the host is reachable, nothing listens on that port |
| `tcp.timeout` | No answer before the deadline — ⚠ **filtered, dropped, or unroutable; these cannot be told apart from here** |
| `tcp.unreachable` | `EHOSTUNREACH` / `ENETUNREACH` — ⚠ **reported by our own side or a router** |
| `tcp.no_route_family` | ⚠ **Every permitted address is IPv6 and our server has no IPv6 route** — ⚠ **our gap, not theirs** |
| `tcp.refused_address` | ⚠ **`internal/dial`'s `Control` hook refused the address at the socket.** Status `refused`. ⚠ **Unreachable while the DNS step and the hook share one policy**; it exists so that, if they ever disagree, the answer is a refusal and never a connection |
| `tcp.failed` | Any other dial error, ⚠ **with its raw text in the attempt's `error`** (`.claude/rules/go.md`) |

⚠ **Multiple addresses**: tried in order (IPv4 first), ⚠ **each attempt recorded**. ⚠ **The step is
`ok` if any attempt connects**, and the detail says which ones did not.
⚠ **When every attempt fails, the first attempted address's outcome is the step's code**
(owner decision, hidetzu/connect-doctor#2, 2026-10-10) — except `tcp.no_route_family`, which needs
every address to be IPv6 and each to have failed for lack of a route on our side.

⚠ **Measured, 2026-10-10, linux, in an unprivileged network namespace: `ENETUNREACH` (101) with no
IPv6 route; `EHOSTUNREACH` (113) from an `unreachable` route; a route into a dummy interface
produces silence and so a timeout; a `blackhole` route fails at once with `EINVAL` and is therefore
not a way to produce `tcp.timeout`.**

### TLS (`crypto/tls` over the same connection, `https` only)

| Outcome code | Observed as |
|---|---|
| `ok` | Handshake completed and the chain verified for the host name |
| `tls.cert_expired` | `x509.CertificateInvalidError{Reason: Expired}` (also not-yet-valid) |
| `tls.cert_untrusted` | `x509.UnknownAuthorityError` — self-signed, or a missing intermediate |
| `tls.cert_name_mismatch` | `x509.HostnameError` |
| `tls.handshake_failed` | An alert from the server, or no common version / cipher |
| `tls.timeout` | No handshake before the deadline |
| `tls.not_tls` | ⚠ **Bytes came back that are not TLS** — typically plain HTTP on 443 |

⚠ **An error that matches none of these is `tls.handshake_failed` with its raw text in
`detail.error`** (`.claude/rules/go.md`) — e.g. a certificate not authorised to sign.
⚠ **`tls.cert_expired` also covers "not yet valid"**: `crypto/x509` reports both as `Expired`;
the dates in detail say which.

⚠ **Trust is the system root store, and nothing else.** ⚠ **There is no way to tell the step to
trust another CA, and no `InsecureSkipVerify`.** ⚠ **Tests give trust the way a deployment does:
`SSL_CERT_FILE` / `SSL_CERT_DIR` (read in go1.26.2 `crypto/x509` `loadSystemRoots`).**

⚠ **Detail shows**: negotiated version, cipher suite, ALPN, leaf subject / SANs / issuer / validity. ⚠ **For a certificate that failed verification, the leaf is still shown**
(`tls.CertificateVerificationError.UnverifiedCertificates`) — the expiry date is the useful part.
⚠ **Not graded.** ⚠ **Not scored** (`PRODUCT.md` § 5).

### HTTP (`net/http` request written on the same connection)

| Outcome code | Observed as |
|---|---|
| `ok` | A status line was read. ⚠ **Any status, including 4xx / 5xx, is `ok` for connectivity** — the detail and conclusion still say what the status means |
| `http.timeout` | No response before the deadline |
| `http.malformed_response` | Bytes that are not an HTTP/1.x response, ⚠ **including response headers over the 64 KiB cap** |
| `http.no_response` | ⚠ **The connection closed (or reset) after the request with not one response byte** — owner decision, hidetzu/connect-doctor#4: ⚠ **nothing arrived ≠ malformed** ([`evidence.md`](../.claude/rules/evidence.md)) |
| `http.too_many_redirects` | ⚠ **Conclusion-level**: more than `limits.RedirectHops` redirects. No `failed_step` |
| `http.redirect_refused` | ⚠ **Conclusion-level**: hop N ≥ 2 was refused — at input (`hops[N-1].code` holds the `input.*` reason, every step skipped), at DNS, or at the socket. No `failed_step`; ⚠ **the summary names the hop and its reason** (owner decision, hidetzu/connect-doctor#5) |

⚠ **Redirects** (hidetzu/connect-doctor#5): only 301, 302, 303, 307 and 308 with a `Location` are
followed (RFC 9110 § 15.4); 300, 304 and a 3xx without `Location` are the answer. A relative
`Location` is resolved against the hop's URL. ⚠ **Each next URL goes through `target.Parse`, DNS,
the policy, TCP, TLS and HTTP from the start; one context bounds every hop (`limits.Check`).**
⚠ **A refused hop's URL is not echoed when it carries credentials or does not parse.**

⚠ **Why 5xx is `ok` at the HTTP layer**: the question is "why does it not connect". A 503 means
it connected, and the server said it is unavailable. ⚠ **The conclusion says exactly that**, and
the ladder shows HTTP ✅ with the status beside it. ⚠ **Confirmed by the owner, 2026-10-10
(hidetzu/connect-doctor#4).** ⚠ **An `ok` conclusion also says: if it does not connect from your
machine, look at your side of the network** (`PRODUCT.md` § 6).

⚠ **The request**: `GET`, `Host`, `User-Agent: ConnectDoctor/<version> (+repository URL)`,
`Connection: close`, written with `(*http.Request).Write`; the response read with
`http.ReadResponse` through one `io.LimitedReader` whose budget is the header cap, then the body cap
(+1 byte, to know it was cut). ⚠ **That reader is the only cap.**

## 3. Timeouts and limits

⚠ **All of these are budgets we chose, not measurements.** ⚠ **Tightened 2026-10-10 as part of the
abuse limits ([`adr/0010`](adr/0010-abuse-limits-protect-targets-first-with-one-in-memory-limiter-per-question.md)).** ⚠ **They live in one place in the code
(`internal/limits`) and nowhere else.**

| Limit | Value | Why this value |
|---|---|---|
| DNS | 2 s | Pure-Go resolver retries within it |
| TCP per attempt / total | 3 s / 4 s | Two attempts fit |
| TLS | 3 s | |
| HTTP (headers) | 4 s | |
| Whole check, all hops | 10 s | ⚠ **Hard ceiling, enforced by one context** |
| Redirect hops | 3 | |
| Response headers | 64 KiB | |
| Response body read | 64 KiB, then closed | ⚠ **Read only to observe that a body arrived** |
| URL length | 2048 bytes | |
| Concurrent checks per server | 8 | ⚠ **Bounds what one instance can be made to do at once** |

## 4. Threat model — SSRF first

⚠ **The server fetches a URL chosen by a stranger.** ⚠ **That is SSRF by construction**, and the
design starts from it, not adds it later.

**Assets**: the server's internal network, cloud metadata endpoints (`169.254.169.254`,
`fd00:ec2::254`, ⚠ **`metadata.google.internal` on Cloud Run, which issues our service account's token** — [`adr/0008`](adr/0008-public-exposure-is-cloud-run-in-tokyo.md)), services on loopback, the server's reputation and egress, the users' privacy.

**Attacker**: anyone who can send a URL — page or API, no authentication.

| # | Threat | Defence | Where |
|---|---|---|---|
| T1 | Literal internal address (`http://127.0.0.1/`, `http://[::1]/`, `http://169.254.169.254/`) | ⚠ **Every address is checked against one policy before any dial** | `internal/policy` |
| T2 | Alternate IP spellings (`2130706433`, `0x7f.1`, `0177.0.0.1`, `[::ffff:127.0.0.1]`) | ⚠ **Hosts that look numeric but are not a canonical address are refused as malformed**; IPv4-mapped IPv6 is unmapped before the check | `internal/target` |
| T3 | A name that resolves internally (`localhost`, `*.localhost`, `*.internal`, `metadata.google.internal`, single-label names) | Refused **before** resolving (RFC 6761, RFC 6762, RFC 8375), ⚠ **and** every resolved address is checked (T1) | `internal/target`, `internal/policy` |
| T4 | ⚠ **DNS rebinding**: resolves public at check time, private at dial time | ⚠ **Resolve once; dial the validated `netip.Addr`, never the name.** ⚠ **The dialer's `Control` hook re-checks the address actually being connected** ([`adr/0003`](adr/0003-dial-the-address-that-was-validated-never-the-name-again.md)) | `internal/dial` |
| T5 | ⚠ **Internal DNS disclosure**: our resolver answers for an internal zone, and we print the answer | ⚠ **A refused address is never shown, in the page or the API** — only that it was refused | `internal/diag` |
| T6 | Mixed answer: one public and one private address | ⚠ **The whole name is refused** ([`adr/0004`](adr/0004-a-name-with-any-non-public-address-is-refused-whole.md)) | `internal/diag` |
| T7 | Redirect to an internal address | ⚠ **Every hop goes through T1–T6 again.** `net/http`'s redirect follower is not used | `internal/diag` |
| T8 | Port scanning | ⚠ **Only ports 80 and 443 are ever dialled** ([`adr/0002`](adr/0002-only-ports-80-and-443-are-ever-dialed.md)). ⚠ **One target per request** | `internal/target` |
| T9 | Credentials in the URL (`https://user:pass@host/`) | ⚠ **Refused.** We never send them, and never echo them | `internal/target` |
| T10 | Resource exhaustion: slow-loris targets, huge headers, huge bodies | Timeouts and caps (§ 3); one context bounds the whole check | `internal/limits` |
| T11 | Using us as an anonymous request cannon | Concurrency cap; ⚠ **per-client limits and a global breaker** (hidetzu/connect-doctor#6), ⚠ **per-target limits** (hidetzu/connect-doctor#27) — [`adr/0010`](adr/0010-abuse-limits-protect-targets-first-with-one-in-memory-limiter-per-question.md) | `internal/ratelimit`, `internal/server` |
| T12 | Our responses leaking the target's secrets (URL query tokens) | ⚠ **Query strings are never logged.** The response echoes the URL only to the caller who sent it | `internal/server` |
| T13 | Non-HTTP schemes (`file:`, `gopher:`, `ftp:`) | ⚠ **Only `http` and `https`** | `internal/target` |

⚠ **The policy is an allow-decision on `netip.Addr`, with one implementation.** It refuses:
unspecified, loopback, private (RFC 1918, RFC 4193), link-local (incl. metadata), multicast,
CGNAT `100.64.0.0/10` (RFC 6598), and every range in the IANA IPv4 and IPv6 Special-Purpose
Address Registries that is not globally reachable (RFC 6890 § 2.2.2 "Global" = false), including
documentation, benchmarking, `0.0.0.0/8`, `240.0.0.0/4`, NAT64 `64:ff9b::/96`, 6to4
`2002::/16`, Teredo `2001::/32`. ⚠ **Where the registry is silent, it refuses** (fail closed).

⚠ **Defence in depth that is not code**: ⚠ **on Cloud Run's free tier there is no network-level egress
filter; the code is the only egress defence, by decision** ([`adr/0008`](adr/0008-public-exposure-is-cloud-run-in-tokyo.md)).
⚠ **The service account has no roles, so a leaked metadata token is worth nothing.**

## 5. Minimal architecture

⚠ **One Go binary. Standard library only. No JavaScript build. No database.**

```text
cmd/connect-doctor/       main: flags, listen
internal/target/          parse + normalise one URL; refuse what is not checkable (T2,T3,T8,T9,T13)
internal/policy/          ⚠ the one address policy (T1). pure function of netip.Addr
internal/dial/            dialer whose Control hook calls policy on the real address (T4)
internal/limits/          every timeout and cap, in one place (§ 3)
internal/diag/            the steps: dns.go tcp.go tls.go http.go, and Check() that runs them
                          and follows redirects (T5,T6,T7). returns diag.Result
internal/server/          GET /            page (html/template, no JS needed)
                          GET /api/check   JSON
                          ⚠ both call diag.Check and render the same diag.Result
```

```text
          URL
           |
     target.Parse ──refuse──> Result{input refused}
           |
     dns.Resolve ──policy on every address──> refuse whole name (T6)
           |
     tcp.Dial(validated addr) ──Control hook: policy again (T4)
           |
     tls.Handshake (same conn)
           |
     http.Do (same conn) ──3xx──> back to target.Parse with the Location (T7)
           |
        Result
```

⚠ **Why the steps share one connection** — [`adr/0005`](adr/0005-tcp-tls-and-http-share-one-connection.md).

## 6. The result data model

⚠ **This is the JSON API, and the page renders exactly this.** ⚠ **Field names are a contract
once the API ships**; renaming one is a breaking change.

```json
{
  "url": "https://example.com/",
  "observed_from": "server",
  "observed_from_note": "ConnectDoctorのサーバから観測した結果です。あなたのPCからの接続結果ではありません。",
  "checked_at": "2026-10-10T09:00:00Z",
  "duration_ms": 312,
  "conclusion": {
    "status": "failed",
    "failed_step": "tls",
    "code": "tls.cert_expired",
    "summary": "TLSハンドシェイクに失敗しています。サーバ証明書の有効期限が切れています。"
  },
  "hops": [
    {
      "url": "https://example.com/",
      "steps": [
        { "step": "dns",  "status": "ok",     "duration_ms": 12, "detail": { "addresses": ["93.184.215.14"] } },
        { "step": "tcp",  "status": "ok",     "duration_ms": 98, "detail": { "address": "93.184.215.14:443", "attempts": [] } },
        { "step": "tls",  "status": "failed", "duration_ms": 201, "code": "tls.cert_expired",
          "message": "サーバ証明書の有効期限が切れています。", "detail": { "not_after": "2026-09-01T00:00:00Z" } },
        { "step": "http", "status": "skipped" }
      ]
    }
  ]
}
```

- `conclusion.status` is one of `ok`, `failed`, `refused`, `incomplete`.
  ⚠ **`incomplete` means every step that ran succeeded and at least one step is
  `not_implemented`.** ⚠ **Since hidetzu/connect-doctor#4 every step is implemented, so no result
  carries either today; both stay in the vocabulary for a step added later.** ⚠ **Never `ok` while a layer was not checked** — ⚠ **"not checked" is not
  "connected"** ([`evidence.md`](../.claude/rules/evidence.md)).
- `conclusion.failed_step` is the **first** step, across all hops, whose status is `failed` or
  `refused`; absent on `ok`, ⚠ **and absent for the conclusion-level codes** (`input.*`,
  `http.redirect_refused`, `http.too_many_redirects`).
- ⚠ **`hops[]` has one entry per URL visited, in order**; the page draws one ladder per hop.
- ⚠ **`code` is the stable contract for programs. `summary` / `message` are for people and may be
  reworded** without a version bump.
- ⚠ **A refused address never appears in `detail`** (T5).
- `duration_ms` is wall time measured on our server, ⚠ **for that step only**.
- ⚠ **HTTP status of the API**: `400` for `input.*`, `200` for every other result (a failed
  target is a successful diagnosis), `503` with `{"error": {"code": "server.busy", ...}}` when the
  concurrency cap is reached — ⚠ **never a `Result`, so "busy" cannot be read as the target failing**.

## 7. Delivery order

⚠ **One layer at a time, each a vertical slice that runs end to end** (the issues say so):

```text
URL input -> DNS -> result      (the first slice)
   + TCP
   + TLS
   + HTTP
   + redirects
   + rate limit
```
