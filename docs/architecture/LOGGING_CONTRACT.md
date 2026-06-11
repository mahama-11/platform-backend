# Platform Logging Contract

This document defines the vendor-neutral business logging contract for Platform services.

## Runtime output

- Services MUST emit structured JSON stdout for application logs in local, cloud-dev, and production-like runtime modes.
- Log events SHOULD include `timestamp`, `level`, `service`, `message`, `request_id`, `trace_id`, and domain-safe context fields.
- Passwords, bearer tokens, cookies, HMAC secrets, and raw customer payloads MUST NOT be logged.
- Gorm SQL logging defaults to structured warn/slow-query output, parameterizes literal values, and truncates long SQL. Full info SQL logging is opt-in for local diagnostics only.

## Cloud-dev examples

The `docs/architecture/logging/cloud-dev/` snippets are local/cloud-dev examples only. They use unauthenticated Loki/Tempo endpoints and Docker socket access for fast lane diagnostics; do not expose these ports publicly or reuse them in production without authentication, network binding, and access-control hardening.

## Query seam

- Product code talks to a neutral `LogQueryProvider` interface instead of importing Loki, Tempo, Grafana, or any vendor SDK directly.
- The first lightweight cloud-dev backend is Loki/Alloy, but the business contract remains provider-neutral.
- `request_id` and `trace_id` are correlation fields; do **not** promote high-cardinality correlation IDs to labels in Loki/Alloy configs.

## Label policy

Allowed low-cardinality labels: `service`, `environment`, `level`, `component`.
High-cardinality values such as `request_id`, `trace_id`, user IDs, order IDs, and runtime job IDs stay in JSON log fields only.
