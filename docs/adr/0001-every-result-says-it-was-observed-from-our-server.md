# 0001 — Every result says it was observed from our server

Status: accepted (2026-10-10)

## Decision

⚠ **Every result, in the page and in the JSON API, states that it was observed from
ConnectDoctor's server and not from the user's machine.** ⚠ **In the result itself, every time** —
not in a footnote, not on an about page.

- The page shows it above the input and beside the conclusion.
- The API carries `observed_from: "server"` and a human sentence, on every response.
- Conclusions put the subject in the sentence ("ConnectDoctorのサーバからは…").

## Why

⚠ **The two readings that go wrong are both likely**: "it works from ConnectDoctor, so the problem
is mine" and "it fails from ConnectDoctor, so the site is down". ⚠ **Neither follows**
([`../PRODUCT.md`](../PRODUCT.md) § 6). Services that phrase a vantage difference as a verdict
("It's just you") are the precedent we are avoiding ([`../RESEARCH.md`](../RESEARCH.md) § 4).

## Rejected

- **A single disclaimer on the landing page.** ⚠ **The API has no landing page**, and an agent
  calling it would never see it.
- **Running checks in the user's browser.** A browser cannot resolve DNS, open raw TCP, or see
  TLS errors in detail. ⚠ **It would answer a different, weaker question.**
