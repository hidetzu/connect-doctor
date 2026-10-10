# 0009 — The public address is connect-doctor.hidetzu.work, through Cloud Run domain mapping

Status: accepted (2026-10-10). Builds on [`0008`](0008-public-exposure-is-cloud-run-in-tokyo.md).

## Decision (owner, 2026-10-10)

- ⚠ **`connect-doctor.hidetzu.work` is mapped to the Cloud Run service with Cloud Run domain
  mapping** (`gcloud beta run domain-mappings`), certificate managed by Google.
- ⚠ **The Cloudflare record is a CNAME to `ghs.googlehosted.com` with the proxy OFF** (DNS only).
- The `*.run.app` URL keeps working.

## Why

- ⚠ **No running cost** — the owner's constraint ([`0008`](0008-public-exposure-is-cloud-run-in-tokyo.md)).
- ⚠ **Nothing new between the user and the service**: requests still arrive through Google's
  front end, so the client address arrives the way 0008 measured it (re-measured below).
- `hidetzu.work` was already verified for this Google account; no new verification was needed.

## ⚠ What it costs

- ⚠ **Domain mapping is a preview feature.** Google's documentation: "Due to latency issues, they
  are not production-ready and are not supported at General Availability"
  ([mapping custom domains](https://docs.cloud.google.com/run/docs/mapping-custom-domains), read
  2026-10-10). ⚠ **Accepted: one check takes seconds; added front-end latency does not change
  which layer fails.**
- ⚠ **TLS 1.0 / 1.1 cannot be disabled** on the mapping (same page). ⚠ **That concerns the page
  users load, not the targets ConnectDoctor checks.**

## ⚠ Observed when it went live (2026-10-10)

- ⚠ **The first certificate attempt failed** ("the challenge data was not visible through the
  public internet", 02:15 UTC) ⚠ **and the next retry succeeded** (served at 02:37 UTC). The mapping
  had been created at 01:39, before the CNAME existed. ⚠ **Inference, not measured: a cached
  negative DNS answer.** ⚠ **Create the CNAME before (or right after) the mapping, and expect up to
  one retry interval (15 minutes) of waiting.**
- Through the custom domain: the page and the API answer; `http://` redirects to `https://`;
  `https://example.com` is `ok` on all four layers; `169.254.169.254`, `metadata.google.internal`
  and `127.0.0.1` are refused. ConnectDoctor diagnosing its own address: `ok`, TLS 1.3, a
  certificate from Google Trust Services.
- ⚠ **NOT re-measured: where the client address arrives through the mapping.** ⚠ **The 0008
  measurement used a throwaway service on its own `run.app` URL; repeating it through this domain
  would need the mapping moved off the live service.** ⚠ **Expected to be the same (the same Google
  front end); that expectation is not a measurement.** ⚠ **hidetzu/connect-doctor#6 measures it
  through this domain before relying on it.**

## Rejected

- **Cloudflare proxy in front** (orange cloud): Cloud Run routes by `Host`, so a rewrite (a Worker)
  would be needed, ⚠ **and the client address would become Cloudflare's** — redesigning
  hidetzu/connect-doctor#6's trust model for no gain the owner asked for.
- **Global external Application Load Balancer**: the documented recommendation, ⚠ **but a fixed
  monthly cost.**
