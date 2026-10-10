# Security

ConnectDoctor connects to URLs chosen by anyone, so a way around its refusals is the most
serious kind of bug it can have.

## What to report

- **A way to make ConnectDoctor connect somewhere it should refuse**: a loopback, private,
  link-local, metadata or other non-public address, a port other than 80 / 443, a scheme other
  than `http` / `https`, by any route (DNS answers, redirects, alternate address spellings).
- **A way to read anything the server should not show**: an address it refused, its own
  configuration, another user's request.
- **A way to use it against a third party** beyond what its limits allow.

The rules it is meant to keep are [`.claude/rules/security.md`](.claude/rules/security.md); the
threat model is [`docs/DESIGN.md`](docs/DESIGN.md) § 4.

## How to report

Use **GitHub's private vulnerability reporting** for this repository: the *Security* tab →
*Report a vulnerability*. Please do not open a public issue for a vulnerability.

If ConnectDoctor connected to your site and you did not expect it, report it the same way. Its
requests carry `User-Agent: ConnectDoctor/<version> (+https://github.com/hidetzu/connect-doctor)`.

## Please do not

Test against hosts you do not own or have permission to test, or run load against the public
service at <https://connect-doctor.hidetzu.work>. Run your own copy instead: see
[`README.md`](README.md).

## Scope notes

- Its abuse limits are in-memory and per instance (`docs/adr/0010`); exceeding them briefly
  after an instance restart is known, not a vulnerability.
- This is a personal project. Reports are read; there is no guaranteed response time.
