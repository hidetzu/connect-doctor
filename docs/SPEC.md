# SPEC — what may be claimed

⚠ **This file holds what this project may claim about itself.** ⚠ Nothing else.
**How to work** is [`CLAUDE.md`](../CLAUDE.md); **how to write it** is
[`.claude/rules/`](../.claude/rules/); **why** is [`adr/`](adr/).

⚠ **Never write a count in here** ([`evidence.md`](../.claude/rules/evidence.md)).
⚠ **Counts are announced by whatever produced them**, at the moment it runs.
⚠ **A count written down here is stale from the moment it is written, and it makes every
parallel change conflict.**

---

## 1. What this implements

⚠ **A row goes in here only once the behaviour exists and a check asserts it.**
⚠ **Planned is not implemented.**

⚠ **"What asserts it" names a case, not a file.** ⚠ **A file name says where to look; it does not
say that anything in there asserts this claim** — and a reader takes the column at its word.
⚠ **Run the case before filling the row in, and read whether it covers every clause of the claim.**

⚠ **Grounds: this went wrong in the project this template was extracted from.** A row named two
files as asserting a claim; ⚠ **neither did**, ⚠ **the gap had already been written down in a
completion report**, and the row was left standing anyway.
⚠ **Saying a gap in one place is not permission to claim it in another.**

⚠ **A static check can stop a row that names a file and no case, a row that names nothing, and a
case name that does not exist.** ⚠ **Nothing mechanical can stop a case that exists and does not
cover the clause** — ⚠ **reading the case is the reviewer's job, and that is the half that gets
skipped.** ⚠ **Write that check early.**

| Layer | What is supported | Which authority, which section | What asserts it |
|---|---|---|---|
| Input | `http`/`https` only; ports exactly 80/443; userinfo, local-only names, non-canonical numeric hosts and policy-refused IP literals refused, each with its code | `rules/security.md` § 2; RFC 6761 § 6.3, RFC 6762, RFC 8375; WHATWG URL § host parsing | `internal/target` `TestParseRefuses`, `TestParseAccepts` |
| Policy | Only global unicast outside the IANA special-purpose "not globally reachable" ranges; IPv6 only within 2000::/3; IPv4-mapped unmapped first | RFC 6890 § 2.2.2; IANA special-purpose registries | `internal/policy` `TestAllowed` |
| DNS | Absolute-name resolution; `dns.not_found` (NXDOMAIN or NODATA), `dns.timeout`, `dns.server_failure`; IPv4 first, deduplicated | `docs/DESIGN.md` § 2 | `internal/diag` `TestDNSOutcomes`, `TestDNSOkIsIncomplete`; final gate `TestDNSOutcomesThroughTheBinary` |
| DNS | ⚠ **Any non-public address refuses the whole name, and no refused address appears in the result** | `docs/adr/0004` | `internal/diag` `TestRefusedAddressNeverShown`; final gate `TestDNSOutcomesThroughTheBinary` |
| Input → DNS | ⚠ **A refused URL causes zero DNS queries**, against a control that causes some | `rules/security.md` § 6 | final gate `TestRefusedURLsReachNothing`; `internal/diag` `TestInputRefusalResolvesNothing` |
| TCP | ⚠ **Every target connection goes through `internal/dial`, whose `Control` hook applies the policy at the socket**; a loopback listener the refused dial would have reached records nothing | `rules/security.md` § 1, § 6; `docs/adr/0003` | `internal/dial` `TestControlRefusesWhatPolicyRefuses`, `TestTCPToLoopbackReachesNoListener`; `internal/conformance` `TestOnlyInternalDialBuildsADialer`; final gate `TestRefusedURLsReachNothing` |
| TCP | `tcp.refused`, `tcp.timeout` (bounded by `limits.TCPAttempt`), `tcp.unreachable`, `tcp.no_route_family`, `tcp.failed` with raw text; IPv4 tried first, every attempt recorded, the first attempt's outcome concludes | `docs/DESIGN.md` § 2 (TCP); owner decision on hidetzu/connect-doctor#2 | `internal/diag` `TestTCPOutcomes`, `TestTCPSecondAddressConnects`, `TestUnknownErrorKeepsItsText`; final gate `TestTCPOutcomesThroughTheBinary`, `TestTCPTimeoutIsBounded`, `TestTCPFallsBackToTheNextAddress` |
| Result | While TLS/HTTP are not built, a clean TCP result concludes `incomplete`, never `ok`; those steps are `not_implemented` (TLS `not_applicable` for `http`) | `docs/DESIGN.md` § 6 | `internal/diag` `TestDNSOkIsIncomplete`, `TestLiteralAddressSkipsDNS`; final gate `TestTCPOutcomesThroughTheBinary` |
| Final gate | ⚠ **Runs only inside an empty network namespace with no route out**, and refuses otherwise | `rules/verification.md` § An exercise must not change the world | final gate `TestHarnessIsIsolated` and `TestMain`'s refusal |
| Page / API | The page shows the API's conclusion verbatim, the ladder, and the vantage sentence; `/` shows the form and the vantage sentence | `docs/adr/0001`, `0006` | `internal/server` `TestAPIAndPageRenderTheSameResult`, `TestPageWithoutURLShowsFormAndVantage`; final gate `TestPage` |
| API | `400` for input refusals | `docs/DESIGN.md` § 6 | `internal/server` `TestAPIInputRefusalIs400`; final gate `TestRefusedURLsReachNothing` |
| Server | ⚠ **Over the concurrency cap: `503 server.busy`, counted, never a DNS outcome** | `rules/security.md` § 4 | `internal/server` `TestBusyIsNotATargetFailure` |
| Server | ⚠ **Request logs carry the path only; the checked URL never reaches a log** | `rules/security.md` § 5 | `internal/server` `TestLogsCarryNoQueryString` |
| Source | No self-dialling HTTP/net calls, no `InsecureSkipVerify`, no `require` in `go.mod` | `rules/security.md` § 1, `rules/go.md` | `internal/conformance` `TestNoSelfDialingCalls`, `TestGoModHasNoRequire` |

## 2. What this deliberately does not implement

⚠ **A gap named here is a decision.** ⚠ **An unnamed gap is just something not done yet** —
they are different things and the difference is stated, not implied.

| Not implemented | Deliberate? | Why |
|---|---|---|
| Ports other than 80 and 443 | ⚠ **yes** | [`adr/0002`](adr/0002-only-ports-80-and-443-are-ever-dialed.md) |
| Connecting to a name that has any non-public address | ⚠ **yes** | [`adr/0004`](adr/0004-a-name-with-any-non-public-address-is-refused-whole.md) |
| Telling NXDOMAIN from NODATA | ⚠ **not observable with the standard library** (measured, `DESIGN.md` § 2) | Needs our own DNS client: ADR first |
| Internationalised domain names | ⚠ **not implemented yet** | Punycode needs a module we do not take (`adr/0006`); refused as `input.idn_not_implemented` |
| TLS, HTTP, redirects | ⚠ **not implemented yet** | hidetzu/connect-doctor#3, #4, #5 |
| HTTP/2 | ⚠ **yes, for the MVP** | [`adr/0005`](adr/0005-tcp-tls-and-http-share-one-connection.md). ⚠ **An h2-only server would be misdiagnosed** |
| A second vantage point | ⚠ **yes** | [`PRODUCT.md`](PRODUCT.md) § 5, § 6 |
| Everything in [`PRODUCT.md`](PRODUCT.md) § 5 | ⚠ **yes** | Listed there, with reasons |

## 3. Measured numbers

⚠ **Every number here carries the denominator of its claim, the date, and the conditions**
([`evidence.md`](../.claude/rules/evidence.md)): the versions that matter, how the environment was
built, how many runs, which percentile.

⚠ **A number without those is deleted, not corrected.**

⚠ **Conditions for the row below**: 2026-10-10, go1.26.2 linux/amd64, pure-Go resolver
(`PreferGo: true`), one run each, against the developer machine's configured recursive resolver.

| What was measured | Value | When | Under what conditions |
|---|---|---|---|
| External tier: our DNS step vs `getent ahosts` for `example.com`, `www.cloudflare.com`, `github.com`, `does-not-exist.example.com` | Same address set for all four names; the last `dns.not_found` on both | 2026-10-10 | Above. ⚠ **Four names, one machine, one run — a sanity record, not a claim about resolvers in general** |
