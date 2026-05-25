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
- Access control: JWT + `platform.admin` permission.

### 3. Metrics

- Source: Gin metrics middleware and business counters
- Endpoint: `/metrics` when enabled
- Production collector: Prometheus / Grafana
- Purpose: health, latency, error rates, business operation counters.

### 4. Trace seeds

The codebase already contains OpenTelemetry seeds:

- `otelgin.Middleware(serviceName)` for Gin request spans
- handler-level spans via `telemetry.StartGinSpan`
- `trace_id` fields in audit/usage/settlement paths
- config block: `monitoring.tracing`

Production tracing remains disabled until a trace backend is deployed.

## Production target

Recommended target topology:

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
2. Update production config:

```yaml
monitoring:
  tracing:
    enabled: true
    service_name: "platform-service"
    environment: "production"
    jaeger_endpoint: "http://<collector-or-jaeger>:14268/api/traces"
    sample_rate: 0.1
```

3. Start with conservative sampling (`0.1`), then raise sampling only for short incident windows.
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
