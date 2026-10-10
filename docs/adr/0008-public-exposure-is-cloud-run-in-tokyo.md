# 0008 — Public exposure is Cloud Run in Tokyo, on the free tier

Status: accepted (2026-10-10). ⚠ **Supersedes [`0007`](0007-public-exposure-is-a-free-cloudflare-tunnel-in-front-of-one-host.md).**

## Decision (owner, 2026-10-10)

- ⚠ **The Go binary runs as one Cloud Run service in `asia-northeast1` (Tokyo)**, request-based
  billing, ⚠ **within the free tier**. The owner registers the billing account.
- ⚠ **Bounded by configuration, not by hope:** maximum instances 1, the smallest CPU and memory
  that pass the final gate, and the per-client rate limit (hidetzu/connect-doctor#6).
- ⚠ **A dedicated service account with no roles.** ⚠ **The metadata server hands out that account's
  token; an account that can do nothing makes a leaked token worth nothing.**
- The service's own `*.run.app` HTTPS URL. ⚠ **No Cloudflare in front for now** — a custom domain
  through Cloudflare stays possible later and changes nothing here.
- Go stays ([`0006`](0006-one-go-binary-standard-library-only-no-frontend-build.md)).

## Why

- ⚠ **Ordinary sockets.** The DNS, TCP and TLS steps observe the target directly, as
  [`DESIGN.md`](../DESIGN.md) § 2 requires. ⚠ **The Workers runtime could not**
  ([`0007`](0007-public-exposure-is-a-free-cloudflare-tunnel-in-front-of-one-host.md)), and for
  Cloudflare Containers it is undocumented whether egress is direct.
- ⚠ **Tokyo costs the same as `us-central1` for CPU, memory and requests**: both are Tier 1, and
  the free tier is a spending discount at Tier 1 prices
  ([Cloud Run pricing](https://cloud.google.com/run/pricing), read 2026-10-10).
- ⚠ **Egress is charged in either region for users in Japan**: the free 1 GiB is "within North
  America" only. ⚠ **Expected to be small (a few KB per check is an estimate, not a
  measurement); measured after the first deploy.**
- ⚠ **The vantage becomes a Japanese data centre.** Sites that refuse foreign addresses are
  diagnosed as they appear from Japan.

## ⚠ What this adds to the threat model, and what answers it

| | |
|---|---|
| ⚠ **Metadata server** (`metadata.google.internal`, `169.254.169.254`) issues the service account's OAuth token ([container contract](https://docs.cloud.google.com/run/docs/container-contract)) | Refused by name (`.internal`, T3) and by address (link-local, T1) before anything is sent; ⚠ **we never send `Metadata-Flavor: Google`**; ⚠ **and the account has no roles** |
| ⚠ **No network-level egress filter at zero cost** (one would need VPC egress, which needs paid NAT for the internet) | ⚠ **Accepted gap.** ⚠ **The code is the only egress defence** — which is what [`security.md`](../../.claude/rules/security.md) already assumes |
| ⚠ **No hard spending cap** on Cloud Run | Max instances 1, rate limit, ⚠ **and a budget alert to the owner** (an alert does not stop billing — stated so nobody believes it does) |

## ⚠ Measured after the first deploy (2026-10-10, revision `connect-doctor-00001`)

| What | Observed | How |
|---|---|---|
| ⚠ **IPv6 egress** | ⚠ **Present.** An IPv6-only name (`ipv6.google.com`) was diagnosed `ok` on all four layers, connecting to an IPv6 address | ConnectDoctor's own API, from the deployed service, one run |
| ⚠ **Client address** | ⚠ **Cloud Run appends the client's address as the LAST entry of `X-Forwarded-For`**; entries a client sends are kept in front of it, so ⚠ **only the last entry can be trusted**. `Forwarded: for=` carried the client's address alone, unaffected by a forged `X-Forwarded-For`. The container's `RemoteAddr` was a link-local address, never the client | A throwaway echo service in the same project and region, queried with no header, one forged entry and two forged entries; then deleted (service and image) |
| ⚠ **Egress per check** | ⚠ **About 4.2 KB per check, at most 5.3 KB** (`kind=internet`). Of it, the JSON returned to the caller was 2.5 KB per check | 20 checks of `https://example.com` within one second; Cloud Monitoring `run.googleapis.com/container/network/sent_bytes_count`. ⚠ The minute containing the batch read 84,681 bytes; ⚠ the minute before read 20,502 bytes that cannot be attributed to the batch with certainty, so the upper bound includes it |

⚠ **These are one run each, from one client, on one day** (`evidence.md`). ⚠ **The client
addresses seen are not recorded here** (`git.md`).
⚠ **At those sizes, 1 GiB of egress is on the order of 200,000 checks.** ⚠ **That is arithmetic on
the measurement above, not a measured monthly figure.**

## Rejected

- **Cloudflare Tunnel in front of a host we run** (0007): costs nothing, ⚠ **but checks would
  leave from that host — the owner's own address if it is at home.**
- **Cloudflare Containers ($5/month)**: possibly viable, ⚠ **but whether egress reaches targets
  directly is undocumented and would have to be paid for to find out.** Revisit if Cloud Run's
  free tier stops fitting.
- **`us-central1`**: same price, ⚠ **US vantage** for a Japanese-speaking audience.
