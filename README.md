# ConnectDoctor

> **Why does this URL not connect?** One URL in, one answer out.

## Why ConnectDoctor exists

When a URL does not work, the error you get is usually the least useful one:
*"This site can't be reached"*, *"connection failed"*, a spinner, a timeout in a log.

Behind that one message are four very different problems, each owned by a different person
and fixed in a different place:

| It stopped at | It usually means | Who fixes it |
|---|---|---|
| **DNS** | the name does not exist, or its DNS is broken | whoever runs the domain |
| **TCP** | nothing is listening, or something is dropping the traffic | whoever runs the server or the firewall |
| **TLS** | the certificate is expired, untrusted, or for another name | whoever runs the certificate |
| **HTTP** | it connected, and the application answered with an error | whoever runs the application |

Telling them apart today means `dig`, then `curl -v`, then `openssl s_client`, and knowing how to
read all three. The hosted tools we looked at answer *"is it up?"*, grade your TLS, or show raw
data from fifty locations ([`docs/RESEARCH.md`](docs/RESEARCH.md)). **None of them simply says
which layer it stopped at, and what that means.**

ConnectDoctor does only that:

```text
DNS   ✅
TCP   ✅
TLS   ❌
HTTP  —

結論: TLSハンドシェイクに失敗しています。サーバ証明書の有効期限が切れています。
```

⚠ **Every answer is observed from ConnectDoctor's server, not from your machine.** If it works
here and not for you, the difference is on your side of the network (proxy, VPN, local DNS,
firewall), and ConnectDoctor says so instead of guessing.

## What it does, and what it will not do

It does **one URL, four layers, the first failure, one sentence**, as a page and as JSON:

```sh
curl 'https://<host>/api/check?url=https://example.com'
```

It will not become a port scanner, traceroute, WHOIS, a TLS grader, a performance tool or an
uptime monitor. Each was left out on purpose, with the reason, in
[`docs/PRODUCT.md`](docs/PRODUCT.md) § 5.

## Safety

ConnectDoctor connects to URLs chosen by strangers, so refusing to connect comes first:
only `http`/`https`, only ports 80 and 443, no private, loopback, link-local or reserved
addresses, addresses validated after DNS and again at the moment of connecting, every redirect
re-validated, every step bounded in time and size.
The threat model is [`docs/DESIGN.md`](docs/DESIGN.md) § 4; the rules are
[`.claude/rules/security.md`](.claude/rules/security.md).

## Status

⚠ **Under construction, one layer at a time.** What works today, and which check proves it, is
[`docs/SPEC.md`](docs/SPEC.md) — ⚠ **and only that file.**

## How this repository is worked on

[`CLAUDE.md`](CLAUDE.md), a port of
[`hidetzu/claude-dev-template`](https://github.com/hidetzu/claude-dev-template).
