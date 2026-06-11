# Observability, Audit, and Diagnostics

This document defines the current Platform observability shape and the intended production path. It separates immutable business audit facts from high-volume request/runtime telemetry so the business PostgreSQL database does not become a general log sink.

## Current layers

### 1. Structured request logs

- Source: `internal/middleware/access_log.go`
- Destination: stdout / container logs
- Purpose: request-level diagnostics and operational log collection
- Required correlation fields:
  - `request_id`
  - `trace_id`
  - `method`, `path`, `route`, `status`, `latency_ms`
  - `user_id`, `org_id`, internal service context when present
- Constraint: do not persist every access log into the Platform business database.

### 2. Business audit logs

- Source: `internal/modules/audit`
- Storage: `platform_audit_logs`
- Purpose: immutable admin/operator-visible business audit trail
- Captures:
  - actor user/org
  - action and target
  - request/trace correlation
  - before/after snapshots and diff summary for write operations
- Query API:
  - `GET /api/v1/audit/logs`
  - `GET /api/v1/audit/logs/:auditID`
  - `GET /api/v1/audit/diagnostics/requests/:requestID` — sanitized request diagnostics derived from audit facts; raw stdout/trace search remains external until a log/trace backend is connected.
- Access control: JWT + `platform.admin` permission.

### 3. Metrics

- Source: Gin metrics middleware and business counters
- Endpoint: `/metrics` when enabled
- Production collector: Prometheus / Grafana
- Purpose: health, latency, error rates, business operation counters.

### 4. OpenTelemetry traces

The codebase already contains OpenTelemetry seeds:

- `otelgin.Middleware(serviceName)` for Gin request spans
- handler-level spans via `telemetry.StartGinSpan`
- `trace_id` fields in audit/usage/settlement paths
- config block: `monitoring.tracing`

Cloud DEV currently runs OTel export through the shared observability stack:

```text
platform-backend -> enterprise-otel-collector:4317 -> enterprise-tempo -> enterprise-grafana
```

DEV configuration keeps `monitoring.tracing.enabled=true`, backend `tempo`, OTLP gRPC endpoint `enterprise-otel-collector:4317`, and `sample_rate=1.0` so cross-service propagation can be verified deterministically. Production enablement remains a separate approval boundary with conservative sampling and capacity review.

## Correlation contract

- `X-Request-ID` is the stable business/operator lookup key. It is accepted from callers, generated if missing, returned as a response header, and included in structured logs and audit facts.
- W3C `traceparent` is the distributed-tracing source of truth. Product services must propagate it on Platform calls so Tempo can stitch the full request tree.
- `X-Trace-ID` is a compatibility response/header field that reflects the active OTel trace ID. It must not override a valid `traceparent`.
- Error, internal, and diagnostics responses should include `request_id`, `trace_id`, `error_code`, and `error_hint`. Ordinary success responses may stay lightweight.
- To manually verify a DEV trace, send a request with both `X-Request-ID` and a valid `traceparent`, then query Tempo by the trace ID, for example:

```bash
curl -sS -H 'X-Request-ID: platform-dev-direct-<ts>' \
  -H 'traceparent: 00-55555555555555555555555555555555-1111111111111111-01' \
  http://127.0.0.1:8195/api/v1/docs/internal-access

curl -sS http://127.0.0.1:3200/api/traces/55555555555555555555555555555555
```

When verifying Cloud DEV from outside the Cloud host, use an SSH local forward to the Cloud backend or run the probe on the Cloud host directly. A public gateway route may not prove the request hit the DEV backend container.

## Production target

Recommended production target topology:

```text
Platform backend stdout JSON logs -> log collector -> Loki/ELK/ClickHouse
Platform backend /metrics          -> Prometheus -> Grafana dashboards
Platform backend OTel spans        -> OTel Collector -> Tempo or Jaeger -> Grafana trace view
Platform audit business events     -> platform_audit_logs -> Platform Console Audit page
```

## Platform Console diagnostics UX

The Platform Console should use audit APIs for business diagnostics, not raw log tailing.

Primary workflow:

1. Search by `request_id`, `trace_id`, actor, action, target, or route.
2. Open audit event detail.
3. Inspect before/after/diff for business changes.
4. Use `trace_id` to jump to Grafana/Tempo/Jaeger once trace backend is enabled.
5. Use request logs for low-level transport failures and audit logs for business facts.

## Trace backend enablement plan

1. Deploy Tempo or Jaeger plus optional OTel Collector on the production Docker network.
2. Update production config after approval:

```yaml
monitoring:
  tracing:
    enabled: true
    service_name: "platform-service"
    environment: "production"
    backend: "tempo"
    otlp_endpoint: "enterprise-otel-collector:4317"
    otlp_insecure: true
    sample_rate: 0.05
```

3. Start with conservative sampling (`0.05` or lower), then raise sampling only for short incident windows or selected high-risk internal/runtime paths.
4. Add Grafana data source and dashboard links.
5. Add a frontend environment variable for trace deep links only after the backend collector is confirmed healthy.

## Guardrails

- Business database may store audit facts, usage facts, settlement facts, and runtime job facts.
- Business database must not become the long-term sink for all stdout/access logs.
- Audit detail may include snapshots; keep the API admin-only and avoid adding secrets, raw tokens, or provider credentials into snapshot fields.
- For any new billable/runtime/admin write path, add or verify:
  - `request_id`
  - `trace_id`
  - audit event when business state changes
  - metrics counter or latency observation
  - error code/hint for operator diagnosis
