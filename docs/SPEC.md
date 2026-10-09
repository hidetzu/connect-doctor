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
| — | — | — | — |

## 2. What this deliberately does not implement

⚠ **A gap named here is a decision.** ⚠ **An unnamed gap is just something not done yet** —
they are different things and the difference is stated, not implied.

| Not implemented | Deliberate? | Why |
|---|---|---|
| Ports other than 80 and 443 | ⚠ **yes** | [`adr/0002`](adr/0002-only-ports-80-and-443-are-ever-dialed.md) |
| Connecting to a name that has any non-public address | ⚠ **yes** | [`adr/0004`](adr/0004-a-name-with-any-non-public-address-is-refused-whole.md) |
| HTTP/2 | ⚠ **yes, for the MVP** | [`adr/0005`](adr/0005-tcp-tls-and-http-share-one-connection.md). ⚠ **An h2-only server would be misdiagnosed** |
| A second vantage point | ⚠ **yes** | [`PRODUCT.md`](PRODUCT.md) § 5, § 6 |
| Everything in [`PRODUCT.md`](PRODUCT.md) § 5 | ⚠ **yes** | Listed there, with reasons |

## 3. Measured numbers

⚠ **Every number here carries the denominator of its claim, the date, and the conditions**
([`evidence.md`](../.claude/rules/evidence.md)): the versions that matter, how the environment was
built, how many runs, which percentile.

⚠ **A number without those is deleted, not corrected.**

⚠ **Nothing has been measured yet.** ⚠ **The first row states its conditions here.**

| What was measured | Value | When | Under what conditions |
|---|---|---|---|
| — | — | — | — |
