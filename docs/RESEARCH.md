# RESEARCH — what already exists, and what ConnectDoctor cuts

⚠ **Surveyed on 2026-10-10, from public documentation and READMEs.** ⚠ **Nothing below was run
by us; it is what each project says about itself.** Items marked *(unconfirmed)* came from a
secondary source or a search snippet and were not confirmed against the primary one.

⚠ **The question this comparison asks is not "what features do they have".**
⚠ **It is "to answer *why does this URL not connect?* fastest, what should we cut?"**

---

## 1. The landscape

| Tool | Vantage | Layers | First failing layer + one-line conclusion? | JSON API | What it is really for |
|---|---|---|---|---|---|
| downforeveryoneorjustme / isitdownrightnow | their server | HTTP only | No — Up / Down | No official one *(unconfirmed)* | "Is it just me?" |
| [check-host.net](https://check-host.net/about/api) | many nodes | ping, TCP, HTTP, DNS — each separate | No | Yes, polled | Multi-location reachability |
| [SSL Labs](https://github.com/ssllabs/ssllabs-scan/blob/master/ssllabs-api-docs-v3.md) | their server | TLS | No — a grade. "usually at least 60 seconds" | Yes, polled, rate-limited | "Is my TLS configured well?" |
| [Uptrends](https://www.uptrends.com/blog/test-your-website-from-the-most-locations-with-uptrends-free-tools/) / GTmetrix | many / browser | resolve, connect, download / page load | No | Paid | Monitoring, performance |
| [dnschecker.org](https://dnschecker.org/) | many resolvers | DNS | No | *(unconfirmed)* | Propagation |
| [MXToolbox](https://mxtoolbox.com/restapi.aspx) | their server | dns, mx, http, tcp, … separate | Pass / Warn / Fail per test | Yes, key, network tests paid | Mail and DNS health |
| [curl `-w`](https://curl.se/docs/manpage.html) | your machine | DNS, connect, TLS, transfer timings | Only via the exit code, by hand | `%{json}` | Everything |
| [httpstat](https://github.com/reorx/httpstat) | your machine | DNS, TCP, TLS, server, transfer | Not stated | `--format json` | "Where did the time go?" |
| [Go `net/http/httptrace`](https://pkg.go.dev/net/http/httptrace) | — (library) | hooks per phase | — | — | Building block |
| mtr / tcping | your machine | path / TCP | No | tcping (Rust): yes | Path diagnosis / port ping |
| [Cloudflare 1.1.1.1/help](https://developers.cloudflare.com/1.1.1.1/check/) | your browser | resolver only | — | — | "Am I using 1.1.1.1?" |
| [testssl.sh](https://github.com/testssl/testssl.sh) | your machine | TLS in depth | No | Yes | TLS audit |
| [hurl](https://hurl.dev/docs/manual.html) | your machine | HTTP scenarios | No | Yes | API tests in CI |
| [Globalping](https://github.com/jsdelivr/globalping) | community probes | ping, traceroute, mtr, dns, http (timings incl. dns/tcp/tls) | No | Yes, 250/h unauthenticated | Multi-location measurement |
| RIPE Atlas | physical probes | many | No | Yes, credits | Research infrastructure |
| [net-diagnostics](https://github.com/hashesgenb65-dev/net-diagnostics) and similar small CLIs | your machine | DNS → TCP → HTTP → TLS, skipping dependents | Yes, plain text | `--json` | ⚠ **Closest in logic** |

## 2. The gap

⚠ **Among the hosted services surveyed, none returns "the first layer that failed" with a
one-sentence conclusion.** They return Up / Down, or raw per-test data the user has to read.

⚠ **The tools that do return a conclusion are local CLIs.** That is a different vantage
(§ 4), and they have to be installed.

⚠ **This is a claim about what was surveyed, not about the whole market**
([`evidence.md`](../.claude/rules/evidence.md): not observed ≠ does not exist).

## 3. What we deliberately do not copy

⚠ **These are recorded in [`PRODUCT.md` § 5](PRODUCT.md#5-what-it-deliberately-does-not-do).
This table is the evidence behind them; that table is the decision.**

| Seen in | Feature | Why it is cut |
|---|---|---|
| check-host, Globalping, RIPE Atlas, dnschecker | Many vantage points | Probes, credits, operations. ⚠ **One vantage, stated honestly, already answers "does it connect from a server on the internet"** |
| SSL Labs, testssl.sh | Grades, cipher enumeration, vulnerability tests | ⚠ **Turns "does it connect" into "is it secure", and takes a minute.** We keep: handshake succeeded or not, the certificate error, validity dates |
| GTmetrix, Uptrends | Performance scores, waterfalls | Measures loading, not reaching |
| mtr, traceroute | Hop-by-hop path | ICMP reachability ≠ HTTP reachability; reading it needs expertise |
| isitdownrightnow, Uptrends | Outage history, user reports, monitoring | Needs state and accounts |
| SSL Labs | Public result boards, shared caches | ⚠ **Leaks which URLs people are checking** |
| hurl | Scenarios, assertions | A test runner's job |
| MXToolbox, Globalping | API keys, credits | ⚠ **No accounts in the MVP.** Abuse is handled by limits, not identity |

## 4. Lessons taken

- ⚠ **SSRF**: of the hosted services, only Globalping states it publicly ("No local network
  tests are allowed, only public endpoints"). ⚠ **The others are silent in their documentation —
  which is not evidence that they lack protection.** Our design: [`DESIGN.md` § 4](DESIGN.md#4-threat-model--ssrf-first).
- ⚠ **Vantage wording**: "It's just you" turns a difference in vantage into a verdict about the
  user. ⚠ **Too strong, and often wrong.** net-diagnostics' note that "unreachable" may be
  "specific to that network" is the better precedent. ⚠ **We put the subject in the sentence**:
  *"from ConnectDoctor's server"* — [`adr/0001`](adr/0001-every-result-says-it-was-observed-from-our-server.md).
- **Building blocks**: curl's timing variables and Go's `httptrace` phases are the same
  decomposition we use. ⚠ **We do not use `httptrace` for the steps themselves**, because it
  observes a client that also resolves and dials on its own — ⚠ **we need to own the dial**
  ([`adr/0003`](adr/0003-dial-the-address-that-was-validated-never-the-name-again.md),
  [`adr/0005`](adr/0005-tcp-tls-and-http-share-one-connection.md)).
