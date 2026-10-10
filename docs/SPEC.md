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
| Input | ⚠ **Input without a scheme is checked as `https://`** (`example.com`, `example.com:443/x`); every refusal applies exactly as for the `https://` form; a scheme without `//` (`mailto:`, `javascript:`, `data:` …) is still a scheme and refused | owner decision on hidetzu/connect-doctor#42 | `internal/target` `TestBareHost`; final gate `TestRefusedURLsReachNothing` |
| Input | `http`/`https` only; ports exactly 80/443; userinfo, local-only names, non-canonical numeric hosts and policy-refused IP literals refused, each with its code | `rules/security.md` § 2; RFC 6761 § 6.3, RFC 6762, RFC 8375; WHATWG URL § host parsing | `internal/target` `TestParseRefuses`, `TestParseAccepts` |
| Policy | Only global unicast outside the IANA special-purpose "not globally reachable" ranges; IPv6 only within 2000::/3; IPv4-mapped unmapped first | RFC 6890 § 2.2.2; IANA special-purpose registries | `internal/policy` `TestAllowed` |
| DNS | Absolute-name resolution; `dns.not_found` (NXDOMAIN or NODATA), `dns.timeout`, `dns.server_failure`; IPv4 first, deduplicated | `docs/DESIGN.md` § 2 | `internal/diag` `TestDNSOutcomes`, `TestDNSOkIsIncomplete`; final gate `TestDNSOutcomesThroughTheBinary` |
| DNS | ⚠ **Any non-public address refuses the whole name, and no refused address appears in the result** | `docs/adr/0004` | `internal/diag` `TestRefusedAddressNeverShown`; final gate `TestDNSOutcomesThroughTheBinary` |
| Input → DNS | ⚠ **A refused URL causes zero DNS queries**, against a control that causes some | `rules/security.md` § 6 | final gate `TestRefusedURLsReachNothing`; `internal/diag` `TestInputRefusalResolvesNothing` |
| TCP | ⚠ **Every target connection goes through `internal/dial`, whose `Control` hook applies the policy at the socket**; a loopback listener the refused dial would have reached records nothing | `rules/security.md` § 1, § 6; `docs/adr/0003` | `internal/dial` `TestControlRefusesWhatPolicyRefuses`, `TestTCPToLoopbackReachesNoListener`; `internal/conformance` `TestOnlyInternalDialBuildsADialer`; final gate `TestRefusedURLsReachNothing` |
| TCP | `tcp.refused`, `tcp.timeout` (bounded by `limits.TCPAttempt`), `tcp.unreachable`, `tcp.no_route_family`, `tcp.failed` with raw text; IPv4 tried first, every attempt recorded, the first attempt's outcome concludes | `docs/DESIGN.md` § 2 (TCP); owner decision on hidetzu/connect-doctor#2 | `internal/diag` `TestTCPOutcomes`, `TestTCPSecondAddressConnects`, `TestUnknownErrorKeepsItsText`; final gate `TestTCPOutcomesThroughTheBinary`, `TestTCPTimeoutIsBounded`, `TestTCPFallsBackToTheNextAddress` |
| TLS | Handshake over the TCP connection, chain verified for the host name against the system roots only, ALPN `http/1.1`; `tls.cert_expired` (incl. not yet valid), `tls.cert_untrusted`, `tls.cert_name_mismatch`, `tls.not_tls`, `tls.timeout` (bounded by `limits.TLS`), `tls.handshake_failed` with raw text | `docs/DESIGN.md` § 2 (TLS); `docs/adr/0005` | `internal/diag` `TestTLSOutcomes`, `TestTLSErrorClassification`, `TestTLSFailureConcludes`; final gate `TestTLSOutcomesThroughTheBinary`, `TestTLSTimeoutIsBounded` |
| TLS | Version, cipher suite, ALPN and the leaf (subject, names, issuer, validity) are shown — ⚠ **also for a certificate that failed verification**; ⚠ **not graded** | `docs/PRODUCT.md` § 5 | `internal/diag` `TestTLSDetail`; final gate `TestTLSDetailThroughTheBinary` |
| TLS | ⚠ **`http://` URLs report TLS `not_applicable`, never `skipped`**, whatever TCP did | hidetzu/connect-doctor#3 AC 3 | final gate `TestTLSOutcomesThroughTheBinary`, `TestTCPOutcomesThroughTheBinary` |
| HTTP | One `GET` on the established connection with `Host`, a `User-Agent` naming ConnectDoctor and the repository, `Connection: close`; HTTP/1.1 only | `docs/DESIGN.md` § 2 (HTTP); `docs/adr/0005`; hidetzu/connect-doctor#4 AC 2 | `internal/diag` `TestHTTPRequestHeaders`; final gate `TestHTTPRequestCarriesOurName` |
| HTTP | ⚠ **Any status is `ok` at this layer and the conclusion is `ok`, saying what the status means** (owner decision); `http.timeout` (bounded by `limits.HTTP`), `http.no_response` (closed with no byte), `http.malformed_response` (incl. headers over 64 KiB) | `docs/DESIGN.md` § 2 (HTTP); owner decisions on hidetzu/connect-doctor#4 | `internal/diag` `TestHTTPOutcomes`, `TestHTTPErrorClassification`, `TestStatusWordsInConclusion`; final gate `TestHTTPOutcomesThroughTheBinary`, `TestHTTPTimeoutIsBounded` |
| HTTP | ⚠ **At most 64 KiB of body is read off the wire**, then the connection is closed; detail says how much was read and whether it was cut | `rules/security.md` § 4; `docs/DESIGN.md` § 3 | `internal/diag` `TestHTTPDetailAndBodyCap` (counts bytes read from the connection); final gate `TestHTTPBodyIsCapped` |
| Result | ⚠ **All four layers run; no step is `not_implemented`.** A clean run concludes `ok` with the status and a pointer to the reader's own network | `docs/DESIGN.md` § 6; hidetzu/connect-doctor#4 AC 3 | `internal/diag` `TestAllLayersOkConcludesOk`; final gate `TestHTTPOutcomesThroughTheBinary` |
| Final gate | ⚠ **Runs only inside an empty network namespace with no route out**, and refuses otherwise | `rules/verification.md` § An exercise must not change the world | final gate `TestHarnessIsIsolated` and `TestMain`'s refusal |
| Page / API | The page renders the API's result: a headline card (`diag.Headline`, `diag.Cause`), then one row of the four steps per hop with durations; the vantage chip on every page; the four-step strip under the input | `docs/adr/0001`, `0006`; owner decision on hidetzu/connect-doctor#25 (candidate A, settled by looking) | `internal/server` `TestAPIAndPageRenderTheSameResult`, `TestPageWithoutURLShowsFormAndVantage`; `internal/diag` `TestHeadlineCauseState`; final gate `TestPage`, `TestPageVisualLanguage` |
| Page | ⚠ **Colour encodes state, never a layer**: ok green, HTTP 4xx/5xx yellow, failure red, ConnectDoctor's refusals neutral | owner decision B on hidetzu/connect-doctor#25 | `internal/server` `TestColourIsStateNotLayer`; `internal/diag` `TestHeadlineCauseState`; final gate `TestPageVisualLanguage` |
| Page | The service icon (candidate A) is served at `/favicon.svg` as `image/svg+xml`, linked as the favicon, and inlined in the header before the name; the CSP adds only `img-src 'self'`; nothing loads from another host | owner decision on hidetzu/connect-doctor#29 (settled by looking) | `internal/server` `TestServiceIcon`; final gate `TestServiceIconThroughTheBinary` |
| Page | Every page shows the tagline, a link to the source, and a privacy sentence that states only what the code does (no storage; the result cache's real duration from `limits.CacheTTL`; the hostname logged on a limit refusal) | owner decisions on hidetzu/connect-doctor#41 | `internal/server` `TestTaglineAndFooter`; final gate `TestPageVisualLanguage` |
| Page | `description` and Open Graph tags (title, the tagline as description); ⚠ **`og:url` / `og:image` / `twitter:card` only from `-public-url`, never guessed from the request**; `/og.png` (candidate A) served as a 1200×630 PNG | owner decisions on hidetzu/connect-doctor#44 | `internal/server` `TestOpenGraph`; final gate `TestOpenGraphThroughTheBinary` |
| Page | The chip names the place given by `-vantage`, and no place without it | owner decision A on hidetzu/connect-doctor#25 | `internal/server` `TestVantageChip`; final gate `TestPageVisualLanguage` |
| API | `400` for input refusals | `docs/DESIGN.md` § 6 | `internal/server` `TestAPIInputRefusalIs400`; final gate `TestRefusedURLsReachNothing` |
| Redirects | 301/302/303/307/308 with `Location` are followed as new hops — ⚠ **each re-parsed, re-resolved and re-checked by the policy from the start**; relative `Location` resolved; 300/304 and a 3xx without `Location` are the answer | `rules/security.md` § 3; `docs/adr/0003`; RFC 9110 § 15.4 | `internal/diag` `TestRedirectIsFollowedAsANewHop`, `TestRelativeLocation`, `TestNotEveryThreeHundredIsFollowed`; final gate `TestRedirectFollowedAndRefused`, `TestRedirectLoopAndRelative` |
| Redirects | ⚠ **A refused hop concludes `http.redirect_refused`, naming the hop and its reason; nothing is resolved or dialled for it** — a redirect to `127.0.0.1` reaches no loopback listener, against a permitted chain that does reach its listener | owner decision on hidetzu/connect-doctor#5; `rules/security.md` § 6 | `internal/diag` `TestRedirectToRefusedTargets`, `TestRedirectToAPrivateName`; final gate `TestRedirectFollowedAndRefused`, `TestRedirectRefusedBeforeResolving` |
| Redirects | More than `limits.RedirectHops` redirects → `http.too_many_redirects`; ⚠ **every hop shares the one `limits.Check` ceiling** | `rules/security.md` § 3, § 4 | `internal/diag` `TestRedirectLoopStops`; final gate `TestRedirectLoopAndRelative`, `TestRedirectChainRespectsTheCeiling` |
| Page | ⚠ **One ladder per hop**, each labelled with its URL | owner decision on hidetzu/connect-doctor#5 | final gate `TestPageShowsEveryHop` |
| Server | ⚠ **Over the concurrency cap (`limits.ConcurrentChecks`, 8 since hidetzu/connect-doctor#26): `503 server.busy`, counted, never a DNS outcome** | `rules/security.md` § 4 | `internal/server` `TestBusyIsNotATargetFailure`, `TestProductionConcurrencyBound` |
| Server | ⚠ **Per client** (IPv4 address, IPv6 /64): burst `limits.ClientBurst`, then one per `limits.ClientRefill`; `limits.ClientPerHour`; `limits.ClientPerDay` ⚠ **best effort** (in memory, per instance); one check at a time. Over the limit: `429 server.rate_limited` + `Retry-After`. The plain page is never limited | owner decisions on hidetzu/connect-doctor#6; `docs/adr/0010` | `internal/ratelimit` `TestBurstThenRefill`, `TestHourAndDay`, `TestOneAtATime`, `TestKeysAndLogPrefixes`; `internal/server` `TestClientLimitAnswers429`; final gate `TestClientRateLimitThroughTheBinary` |
| Server | Over a client limit, the page and the API message name the visitor's own limit (burst / hour / day / one at a time, numbers from `internal/limits`) and the seconds until the next check, equal to `Retry-After` | owner decision on hidetzu/connect-doctor#43 | `internal/server` `TestClientLimitAnswers429`, `TestRateLimitedSentences`; final gate `TestClientRateLimitThroughTheBinary` |
| Server | ⚠ **The client is the LAST `X-Forwarded-For` entry, and only with `-trust-xff`**; forged leading entries do not reset a limit; without the flag the header is ignored | `docs/adr/0008` (measured on Cloud Run) | `internal/server` `TestForgedForwardedForDoesNotResetTheLimit`, `TestForwardedForIgnoredUnlessTrusted`; final gate `TestClientRateLimitThroughTheBinary` |
| Server | Global breaker `limits.GlobalPerHour` per instance → `503 server.busy` + `Retry-After` | `docs/adr/0010` | `internal/ratelimit` `TestGlobalBreaker` |
| Server | ⚠ **A refusal is logged with the target hostname and the client prefix (IPv4 /24, IPv6 /48) only** — never path, query or a full address; ordinary use is not logged | `docs/adr/0010` | `internal/server` `TestClientLimitAnswers429` |
| Targets | ⚠ **Per hostname**: `limits.TargetHostBurst`, then one per `limits.TargetHostRefill`, spent when each hop starts (redirect targets included), whoever asks; past it nothing is resolved or dialled | owner decisions on hidetzu/connect-doctor#6; hidetzu/connect-doctor#27 AC 1, 3 | `internal/ratelimit` `TestHostBudget`; `internal/diag` `TestHostLimitStopsBeforeAnythingLeaves`, `TestRedirectIntoALimitedHostname`; final gate `TestHostnameBudgetThroughTheBinary`, `TestRedirectIntoASpentHostname` |
| Targets | ⚠ **Per destination IP + port**: `limits.TargetDestBurst`, then one per `limits.TargetDestRefill`, ⚠ **counted per TCP attempt**, shared by every hostname on that address | hidetzu/connect-doctor#27 AC 2 | `internal/ratelimit` `TestDestinationBudgetIsSharedAcrossHostnames`; `internal/diag` `TestDestinationLimitIsPerAttempt`; final gate `TestDestinationBudgetThroughTheBinary` |
| Targets | A target-limited check: `429 server.target_rate_limited` + `Retry-After`; logged with the hostname and client prefix only; a production server always carries the target limiter; memory bounded | `docs/adr/0010` | `internal/server` `TestTargetLimitAnswers429`; `internal/ratelimit` `TestTargetMemoryIsBounded` |
| Cache | ⚠ **A repeated check of the same normalised URL within `limits.CacheTTL` is answered without connecting**, with `cached: true` and the original `checked_at`; failures are cached too; target-limit refusals and input refusals are not; bounded (`limits.CacheEntries`) | owner decisions on hidetzu/connect-doctor#6; hidetzu/connect-doctor#28 | `internal/server` `TestCacheTTL`, `TestCacheKeyIsTheNormalisedURL`, `TestWhatIsNotCached`, `TestCacheIsBounded`, `TestRepeatedCheckIsAnsweredFromTheCache`; final gate `TestCacheThroughTheBinary` |
| Cache | The page shows 「N秒前の診断結果です」 and a 再診断 link (`fresh=1`); 再診断 connects again, ⚠ **and the per-client and per-target limits still apply to it**; the per-client limit applies to cached answers too | hidetzu/connect-doctor#28 AC 3 | `internal/server` `TestRepeatedCheckIsAnsweredFromTheCache`, `TestTargetLimitAnswers429`; final gate `TestCacheThroughTheBinary`, `TestHostnameBudgetThroughTheBinary` |
| Server | The limiter's memory is bounded (`limits.TrackedClients`) | `rules/security.md` § 4 | `internal/ratelimit` `TestMemoryIsBounded` |
| Server | ⚠ **Request logs carry the path only; the checked URL never reaches a log** | `rules/security.md` § 5 | `internal/server` `TestLogsCarryNoQueryString` |
| Source | No self-dialling HTTP/net calls, ⚠ **no `InsecureSkipVerify`** (hidetzu/connect-doctor#3 AC 4), no `require` in `go.mod` | `rules/security.md` § 1, `rules/go.md` | `internal/conformance` `TestNoSelfDialingCalls`, `TestGoModHasNoRequire` |

## 2. What this deliberately does not implement

⚠ **A gap named here is a decision.** ⚠ **An unnamed gap is just something not done yet** —
they are different things and the difference is stated, not implied.

| Not implemented | Deliberate? | Why |
|---|---|---|
| Ports other than 80 and 443 | ⚠ **yes** | [`adr/0002`](adr/0002-only-ports-80-and-443-are-ever-dialed.md) |
| Connecting to a name that has any non-public address | ⚠ **yes** | [`adr/0004`](adr/0004-a-name-with-any-non-public-address-is-refused-whole.md) |
| Telling NXDOMAIN from NODATA | ⚠ **not observable with the standard library** (measured, `DESIGN.md` § 2) | Needs our own DNS client: ADR first |
| Internationalised domain names | ⚠ **not implemented yet** | Punycode needs a module we do not take (`adr/0006`); refused as `input.idn_not_implemented` |
| HTTP/2 | ⚠ **yes, for the MVP** | [`adr/0005`](adr/0005-tcp-tls-and-http-share-one-connection.md). ⚠ **An h2-only server would be misdiagnosed** |
| A second vantage point | ⚠ **yes** | [`PRODUCT.md`](PRODUCT.md) § 5, § 6 |
| Everything in [`PRODUCT.md`](PRODUCT.md) § 5 | ⚠ **yes** | Listed there, with reasons |

## 3. Measured numbers

⚠ **Every number here carries the denominator of its claim, the date, and the conditions**
([`evidence.md`](../.claude/rules/evidence.md)): the versions that matter, how the environment was
built, how many runs, which percentile.

⚠ **A number without those is deleted, not corrected.**

⚠ **Conditions**: 2026-10-10. Rows marked *dev* ran on the developer machine (go1.26.2 linux/amd64,
pure-Go resolver, its configured recursive resolver). Rows marked *Cloud Run* ran against the
deployed service `connect-doctor` in `asia-northeast1`, revision `connect-doctor-00001`, image
built from `d40006c` ([`adr/0008`](adr/0008-public-exposure-is-cloud-run-in-tokyo.md),
[`DEPLOY.md`](DEPLOY.md)). One run each.

| What was measured | Value | When | Under what conditions |
|---|---|---|---|
| *dev* — External tier: our DNS step vs `getent ahosts` for `example.com`, `www.cloudflare.com`, `github.com`, `does-not-exist.example.com` | Same address set for all four names; the last `dns.not_found` on both | 2026-10-10 | Above. ⚠ **Four names, one machine, one run — a sanity record, not a claim about resolvers in general** |
| *Cloud Run* — IPv6 egress | Present: `ipv6.google.com` (IPv6-only) diagnosed `ok` on all four layers | 2026-10-10 | Above |
| *Cloud Run* — where the client address arrives | Last entry of `X-Forwarded-For` (earlier entries are client-supplied); `Forwarded: for=` unaffected by forgery | 2026-10-10 | Above; throwaway echo service, since deleted |
| *Cloud Run* — egress per check | ≈ 4.2 KB, ≤ 5.3 KB, checking `https://example.com` | 2026-10-10 | Above; Cloud Monitoring `sent_bytes_count`, `kind=internet`; ⚠ the batch, its size and why the bound is a range are in [`adr/0008`](adr/0008-public-exposure-is-cloud-run-in-tokyo.md) |
| *Cloud Run* — custom domain `connect-doctor.hidetzu.work` | Serving with a Google-managed certificate; the first issuance attempt failed and the next retry succeeded | 2026-10-10 | Domain mapping ([`adr/0009`](adr/0009-the-public-address-is-connect-doctor-hidetzu-work-through-cloud-run-domain-mapping.md)) |
| *Cloud Run* — client address through the custom domain | Last `X-Forwarded-For` entry: with a different forged entry on each request, the per-client limit still answered 429 after the burst | 2026-10-10 | Revision `connect-doctor-00003`, `-trust-xff`; one machine |
