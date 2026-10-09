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
- **Workers with TCP sockets (`connect()`)** — ⚠ **our reading, not tested**: a Worker is a
  JavaScript runtime, so the Go implementation would be rewritten, and `connect()` takes a host
  name, so resolving once and dialling the validated address (`adr/0003`) and observing DNS as
  its own step could not be done the way the design requires
  ([TCP sockets](https://developers.cloudflare.com/workers/runtime-apis/tcp-sockets/)).

## ⚠ Defence in depth that this adds, and that it does not

- Adds: no open inbound port; Cloudflare in front for volumetric abuse.
- ⚠ **Does not add egress filtering.** ⚠ **Denying private ranges at the host's firewall is still
  recommended** (`DESIGN.md` § 4) ⚠ **and depends on which host is chosen.**

## ⚠ Still open (owner)

- ⚠ **Which host runs the binary** — a machine on the owner's own network, or a VM elsewhere.
  ⚠ **On a home network, strangers' URLs are fetched from the owner's home address**: the policy
  refuses private addresses, ⚠ **but the owner's IP reputation and ISP terms are then exposed.**
- Whether the vantage sentence names the region.
