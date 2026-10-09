# 0004 — A name with any non-public address is refused whole, and the address is never shown

Status: accepted (2026-10-10)

## Decision

- ⚠ **If any A / AAAA address of a name is refused by policy, the whole check is refused** —
  we do not connect to the public addresses that remain.
- ⚠ **A refused address is never shown**, in the page or the API. ⚠ **We say that the name
  resolves to an address ConnectDoctor does not connect to, and nothing more.**

## Why

- A public name that also publishes a private address is either misconfigured or an attempt to
  steer us. ⚠ **Connecting to "the public half" turns a rebinding attempt into a coin toss we
  hope to win.** Refusing has no such dependency.
- ⚠ **Our resolver may answer for zones that are internal to where we run.** Printing the answer
  would make us a lookup service for an internal DNS (DESIGN § 4, T5).

## Rejected

- **Filter to the public addresses and continue.** Rejected above.
- **Show the refused address "because it is public DNS anyway".** ⚠ **Not when our resolver is
  the one answering.**

## Consequence

⚠ **A legitimate site that publishes a stray private record gets "refused" rather than a
diagnosis.** That is a real cost; the message says it is our policy, not the site's failure.
