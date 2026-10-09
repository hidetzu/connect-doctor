# 0006 — One Go binary, standard library only, no frontend build

Status: accepted (2026-10-10)

## Decision

- ⚠ **Go, standard library only.** No third-party module, ⚠ **including `golang.org/x/...`**.
- ⚠ **The page is server-rendered with `html/template`, embedded in the binary.** ⚠ **It works with
  JavaScript disabled.** A form `GET /?url=` renders the result.
- ⚠ **The page and `/api/check` render the same `diag.Result`.**

## Why

- ⚠ **Every dependency is code that runs next to a fetcher of hostile URLs.** The standard library
  covers DNS, TCP, TLS, HTTP and `netip`.
- ⚠ **A frontend framework would be a second build system for one form and one table.**
- ⚠ **One result type, two renderings, means the page and the API cannot disagree** — that is
  PRODUCT § 4 clause 10.

## Rejected

- **A SPA calling the API.** Possible later, ⚠ **and the API is already the contract it would
  use**. Not needed to answer the question.
- **`miekg/dns`** for richer DNS detail. ⚠ **Would let us show which authoritative server failed**;
  rejected for MVP because the conclusion does not need it.
