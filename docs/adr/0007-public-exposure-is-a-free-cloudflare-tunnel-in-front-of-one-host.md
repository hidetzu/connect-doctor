# 0007 — Public exposure is a free Cloudflare Tunnel in front of one host we run

Status: accepted (2026-10-10), ⚠ **with the host itself still open** (hidetzu/connect-doctor#8)

## Decision

- ⚠ **Owner decision: Cloudflare, at no running cost.**
- ⚠ **So: the Go binary runs on one host, listening on loopback only, and `cloudflared` (Cloudflare
  Tunnel) carries public HTTP to it.** No inbound port is opened on the host.
- ⚠ **The client's address is `CF-Connecting-IP`, trusted only because the listener is reachable
  from `cloudflared` on loopback and nothing else.** ⚠ **If the listener is ever exposed another
  way, that header becomes forgeable and must stop being trusted** (feeds hidetzu/connect-doctor#6).

## ⚠ What this means for the vantage point

⚠ **Checks leave from the host's network, not from Cloudflare's.** The tunnel carries the user's
request *in*; ⚠ **our DNS, TCP, TLS and HTTP go *out* from wherever the binary runs.**
⚠ **The vantage sentence must therefore never say "from Cloudflare"** (`adr/0001`).

## Why not run on Cloudflare itself

⚠ **Read from Cloudflare's documentation on 2026-10-10; not tried.**

- **Cloudflare Containers** would run the binary as-is, ⚠ **but require the Workers Paid plan
  ($5/month); the Free plan lists them as not available**
  ([Containers pricing](https://developers.cloudflare.com/containers/pricing/)).
  ⚠ **Rejected by the owner's cost decision, not on technical grounds.** Revisit if that changes.
- **Workers (free) with TCP sockets (`connect()`), in any language.** ⚠ **The owner allowed
  changing the language to fit the free tier (2026-10-10). ⚠ It would not help: the limits below
  belong to the Workers runtime, so JavaScript, TypeScript and Rust/Wasm all hit them.**
  From [TCP sockets](https://developers.cloudflare.com/workers/runtime-apis/tcp-sockets/), read
  2026-10-10:
  - ⚠ **"Outbound TCP sockets to Cloudflare IP ranges are blocked."** ⚠ **Measured the same day:
    `example.com` and `www.cloudflare.com` resolve only to addresses inside Cloudflare's published
    ranges (`104.16.0.0/13`, `172.64.0.0/13`, [ips-v4](https://www.cloudflare.com/ips-v4/)).**
    ⚠ **So the TCP and TLS steps could not run for any site behind Cloudflare** — ⚠ **including
    the README's own example.**
  - ⚠ **No documented way to read why a TLS handshake failed** — a rejected promise only.
    ⚠ **Expired / untrusted / name mismatch (`DESIGN.md` § 2, TLS) is the core of the TLS answer.**
  - ⚠ **No documented way to tell `refused` from `timeout`.**
  - Ports 80/443 are pointed at `fetch` rather than sockets; `connect()` takes a host name, and
    whether it resolves internally is not documented — ⚠ **so `adr/0003` could not be shown to hold.**
  - Free plan: 10 ms CPU per request, 50 subrequests, 100,000 requests/day
    ([limits](https://developers.cloudflare.com/workers/platform/limits/)) — ⚠ workable, ⚠ **not
    the reason for rejecting it.**
  ⚠ **Rejected: the product's question ("which layer, and why") could not be answered there.
  Go stays** (`adr/0006`).

## ⚠ Defence in depth that this adds, and that it does not

- Adds: no open inbound port; Cloudflare in front for volumetric abuse.
- ⚠ **Does not add egress filtering.** ⚠ **Denying private ranges at the host's firewall is still
  recommended** (`DESIGN.md` § 4) ⚠ **and depends on which host is chosen.**

## ⚠ Still open (owner)

- ⚠ **Which host runs the binary** — a machine on the owner's own network, or a VM elsewhere.
  ⚠ **On a home network, strangers' URLs are fetched from the owner's home address**: the policy
  refuses private addresses, ⚠ **but the owner's IP reputation and ISP terms are then exposed.**
- Whether the vantage sentence names the region.
