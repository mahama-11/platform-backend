# Platform Observability Event Specification

本文件定义 Platform 后端的结构化事件、span 和敏感字段边界。Ecommerce 等产品服务可引用自己的产品侧扩展，但共享 request/trace/audit 语义以本文件为准。

## Event naming

使用点分隔、领域优先命名：

```text
<platform_or_product>.<module>.<object_or_step>.<operation>.<phase>
```

生命周期 phase 固定为：

- `started`
- `finished`
- `failed`

Platform 事件示例：

```text
platform.auth.session.create.started
platform.auth.session.create.finished
platform.auth.session.create.failed
platform.audit.diagnostics.request.summary.started
platform.audit.diagnostics.request.summary.finished
platform.audit.diagnostics.request.summary.failed
platform.runtime.job.create.started
platform.runtime.job.create.finished
platform.runtime.job.create.failed
platform.metering.usage.record.started
platform.metering.usage.record.finished
platform.metering.usage.record.failed
platform.storage.asset.import.started
platform.storage.asset.import.finished
platform.storage.asset.import.failed
```

## Span naming

Span 名称去掉 terminal phase，保持 operation 语义稳定：

```text
platform.auth.session.create
platform.audit.diagnostics.request.summary
platform.runtime.job.create
platform.metering.usage.record
platform.storage.asset.import
```

## Standard fields

事件字段优先使用低基数字段：

```text
request_id
trace_id
service
module
operation
product_code
org_id
user_id_hash
job_id
provider
status
latency_ms
error_code
```

可按需增加 `runtime_job_id`、`charge_session_id`、`reservation_id`、`asset_id`、`settlement_id` 等排障 ID，但不要写入高敏 payload。

## Error fields

- `status`: `failed`
- `error_code`: 稳定机器码，不是 raw provider message
- `error`: 已脱敏短消息，建议不超过 300 字符

不得在标准请求事件里写入 stack trace 或 raw upstream response body。

## Forbidden sensitive fields

禁止记录：

```text
token
secret
provider key
raw prompt or large prompt text
signed URL / storage key
user private free text
full provider payload
full runtime manifest
idempotency key raw value
```

## request_id / trace_id inheritance

1. 入口 HTTP 请求：接受 `X-Request-ID`；缺失时生成并回写响应头，用于业务、客服和日志检索。
2. 分布式追踪以 W3C `traceparent` 为准；Platform 的 OTel middleware 会继承 `traceparent`，并把继承/生成后的 OTel trace ID 写入日志、响应 `X-Trace-ID` 以及错误响应 body 的 `trace_id`。
3. `X-Trace-ID` 仅作为兼容字段保留。没有 `traceparent` 时它不能创建完整 OTel trace；有 `traceparent` 时不应覆盖 OTel trace ID。
4. Platform → Product 或 Product → Platform internal API：必须透传 `X-Request-ID` 和 `traceparent`；可同步透传 `X-Trace-ID` 兼容旧日志检索。
5. 诊断 API：先用 request_id 查日志，再从匹配日志反推 trace_id；不要把 raw log line 写入业务数据库。

## Implementation entrypoints

- Platform helper: `platform-backend/internal/observability`
- Diagnostics API: `GET /api/v1/audit/diagnostics/requests/:requestID`
- Product-side example: `ecommerce-backend/docs/architecture/OBSERVABILITY_EVENT_SPEC.md`
