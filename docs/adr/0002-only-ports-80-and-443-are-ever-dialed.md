# 0002 — Only ports 80 and 443 are ever dialled

Status: accepted (2026-10-10)

## Decision

⚠ **A URL whose effective port is not 80 or 443 is refused before anything is resolved.**
`http://host:8080/` is refused; `https://host:80/` and `http://host:443/` are permitted (they are
real misconfigurations worth diagnosing).

## Why

⚠ **A server that connects to any port a stranger names, and reports refused / timeout /
connected, is a port scanner pointed at third parties.** ⚠ **The distinction between
`tcp.refused` and `tcp.timeout` is exactly what makes our TCP step useful — and exactly what makes
an arbitrary-port version a scanner.** Limiting the ports keeps the first and removes the second.

## Rejected

- **A wider allow-list (8080, 8443, …).** ⚠ **Every added port is another service fingerprintable
  through us.** Revisit only with a concrete demand, ⚠ **and it is an owner decision**.
- **Allowing any port but hiding the TCP outcome.** ⚠ **Timing still leaks it**, and a hidden
  outcome contradicts the product.

## Consequence

⚠ **ConnectDoctor cannot diagnose services on other ports.** Stated in the refusal message as our
limitation, not the URL's fault (`CLAUDE.md` § 4-1).
