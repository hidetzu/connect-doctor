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

## ⚠ Measured after the first deploy, not assumed

- ⚠ **Whether IPv6 egress exists.** If not, IPv6-only targets are `tcp.no_route_family`, worded as ours.
- ⚠ **Which header carries the client address, and which entry of it can be trusted**
  (`X-Forwarded-For`) — ⚠ **not documented for Cloud Run in what was read; decides hidetzu/connect-doctor#6.**
- ⚠ **Egress bytes per check.**

## Rejected

- **Cloudflare Tunnel in front of a host we run** (0007): costs nothing, ⚠ **but checks would
  leave from that host — the owner's own address if it is at home.**
- **Cloudflare Containers ($5/month)**: possibly viable, ⚠ **but whether egress reaches targets
  directly is undocumented and would have to be paid for to find out.** Revisit if Cloud Run's
  free tier stops fitting.
- **`us-central1`**: same price, ⚠ **US vantage** for a Japanese-speaking audience.
