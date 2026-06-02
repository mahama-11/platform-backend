# Platform Backend Guide

## 1. Purpose

This guide defines the current scope and engineering baseline of `v-platform-backend`.

## 1.1 Architecture Docs

- [OBSERVABILITY_EVENT_SPEC.md](architecture/OBSERVABILITY_EVENT_SPEC.md): 共享事件/span/字段契约、敏感字段排除、request/trace ID 继承规则
- [PLATFORM_STABILITY_CLOSED_LOOP_GATES.md](architecture/PLATFORM_STABILITY_CLOSED_LOOP_GATES.md): 平台稳定性闭环质量门禁，覆盖核心契约、runtime/provider/callback/result、quota/metering/grant-policy、storage/audit/observability 与 Ecom/Menu/KYC consumer sweep
- [INTERNAL_API_CONTRACT.md](INTERNAL_API_CONTRACT.md): 平台内部服务接入契约、统一响应、错误处理与重试建议
- [openapi/README.md](openapi/README.md): internal API 的 Swagger/OpenAPI 生成入口与当前覆盖范围
- [PROD_DEPLOY_RUNBOOK.md](PROD_DEPLOY_RUNBOOK.md): Platform prod deploy、drift check、runtime smoke 与证据自动化流程
- [COMMERCIALIZATION_BOUNDARY.md](architecture/COMMERCIALIZATION_BOUNDARY.md): 平台商业化边界、实施顺序、工程化基线
- [COMMERCIALIZATION_FULL_VIEW_AND_PHASES.md](architecture/COMMERCIALIZATION_FULL_VIEW_AND_PHASES.md): 商业化全面视角、P0/P1/P2 分期与扩展预留
- [COMMERCIAL_CATALOG_MODEL.md](architecture/COMMERCIAL_CATALOG_MODEL.md): 商品、SKU、套餐、计费项、价格模型设计
- [COMMERCIAL_ROUTING_MODEL.md](architecture/COMMERCIAL_ROUTING_MODEL.md): 多主体、商户账号、billing profile、路由策略设计
- [METERING_AND_USAGE_SSOT.md](architecture/METERING_AND_USAGE_SSOT.md): 计量事件、用量真相源、同步校验与异步计量设计
- [CHANNEL_PARTNER_MODEL_AND_STATES.md](architecture/CHANNEL_PARTNER_MODEL_AND_STATES.md): 渠道分佣详细模型、状态机、事件契约与 referral 演进路径
- [CHANNEL_PROFIT_SHARE_AND_POLICY_RESOLUTION.md](architecture/CHANNEL_PROFIT_SHARE_AND_POLICY_RESOLUTION.md): 渠道净利润分佣、policy version、assignment 覆盖与冲突审计设计
- [RUNTIME_CONFIGURATION_REFERENCE.md](architecture/RUNTIME_CONFIGURATION_REFERENCE.md): runtime callback/provider/storage 配置说明、支持值与本地 ComfyUI Bridge 接线参考
- [RUNTIME_AND_CHARGE_PLATFORMIZATION.md](architecture/RUNTIME_AND_CHARGE_PLATFORMIZATION.md): runtime job / charge session 平台化落地边界与 internal API
- [ASSET_STORAGE_REGISTRY_AND_IMPORT.md](architecture/ASSET_STORAGE_REGISTRY_AND_IMPORT.md): 平台存储资产注册、import-local 接口与模板示例导入规范
- [DB_MIGRATION_GOVERNANCE.md](../../docs/architecture/DB_MIGRATION_GOVERNANCE.md): platform/menu 当前数据库迁移风险、临时补丁与后续治理方向
- [CODE_QUALITY_REVIEW_2026_04.md](specs/CODE_QUALITY_REVIEW_2026_04.md): 代码质量审查记录（并发安全、事务原子性、安全加固）

## 2. Intended Capability Scope

Should belong here:

- `/auth/*`
- `/users/me`
- `/orgs/*`
- `/memberships/*`
- `/roles/*`, `/permissions/*`
- `/entitlements/*`
- `/subscriptions/*`
- `/payments/*`, `/refunds/*`
- `/wallets/*`, `/coupons/*`, `/rewards/*`
- `/metering/*`, `/billing/*`, `/events/*`

Should not belong here:

- KYC OCR / liveness workflows
- Attendance shift / punch / report workflows
- Menu generation / image enhancement / template workflows

## 3. Engineering Status

- Go service runtime is active with both public `/api/v1/*` and internal `/internal/v1/*` surfaces
- Gin + Gorm + JWT + bcrypt chosen for the first platformization pass
- First copied/adapted capability slice covers auth, identity, org context, and permission loading
- P0 commercialization foundation is already active with catalog, package / rate card CRUD, billing profile / route resolve, routing policy CRUD, metering / usage ingestion, unified usage settlement decision (`quota`, unified wallet-backed `credits`, `included_then_overage`, `wallet` pre-deduct then `billing_ledger` fallback), settlement snapshot records, settlement reverse capability, discount ledger posting, reward / commission linkage, quota / credits reserve-commit-release control routes, wallet/internal-currency ledger baseline, asset lifecycle definitions, allowance policies, automatic expire / cycle-reset scheduling, request correlation, structured logging, OTel HTTP tracing, Prometheus-backed metrics, diff-capable audit logging, permission-gated management APIs, and HMAC-capable internal service authentication
- Platform audit persistence is isolated to `platform_audit_logs`. The service must not reuse legacy shared-business `audit_logs`, because that table is still owned by older services with incompatible row shape and `details` typing.
- Platform balance truth now converges on wallet accounts / buckets / ledgers. `credits_ledger` remains only as a historical compatibility ledger for old rows and reversal fallback; new grant, reserve, commit, settlement, and reverse flows consume wallet-backed credits instead of creating fresh `credits_ledger` entries.
- The migration layer now includes `202604170003_backfill_credits_ledger_into_wallet`, which backfills each subject's historical `credits_ledger` net balance into a `PLATFORM_CREDIT` permanent wallet account / bucket / ledger exactly once before later code cleanup removes the old runtime dependency.
- Platform storage is now treated as a shared internal capability rather than a browser-facing static site: product backends upload source assets through `/internal/v1/storage/assets`, runtime workers resolve `storage_key` back into bytes before calling third-party providers, and browser access should flow through product-owned signed/authorized content routes instead of a public `/storage/*` path.
- Platform storage surface is now broader than a single upload route: internal APIs already cover upload, metadata registration, local import, metadata lookup, content fetch, and bulk resolve under `/internal/v1/storage/assets*`, making storage a reusable registry-backed capability rather than a thin file drop.
- Runtime provider routing is no longer limited to a single static binding. Product backends may still submit a business-facing task type such as `image_generation`, but platform runtime now supports route preferences (`objective` such as `quality`, `speed`, `cost`), ranked provider bindings via binding metadata, async provider polling/callback reconciliation, and provider fallback chains when the current provider becomes unavailable or retryable failures exceed the preferred path.
- ComfyUI Bridge is wired as the first async image-provider sample rather than a one-off special case: `POST /generate/text` and `POST /generate/image` return `task_id`, platform runtime persists that as `provider_job_id`, then reconciles via provider callback plus fallback polling against `GET /tasks/{task_id}`. Result images come back as base64 and are normalized into platform storage before product callbacks receive the final asset records. Binding metadata may now include `objective_scores` and `fallback_on` to express quality-first / speed-first / cost-first routing and fallback behavior without pushing raw provider branching into product backends.
- Runtime product callbacks are now treated as a platform abstraction rather than a Menu-specific shortcut. Platform runtime still supports `menu_internal`, but the final product callback hop is modeled as a product callback layer with callback-kind dispatch (for example `menu_internal` and `ecommerce_internal`) plus normalized runtime/result payloads. Product backends are expected to implement their own internal callback routes while platform remains responsible for provider orchestration, storage normalization, and final callback delivery.
- Runtime observability now includes standardized structured lifecycle logs on the critical async path. Look for events such as `runtime.dispatch.started`, `runtime.dispatch.accepted`, `runtime.poll.progress`, `runtime.fallback.scheduled`, `runtime.completed`, `runtime.failed`, `runtime.callback.update_failed`, and `runtime.callback.results_failed`, all keyed by `runtime_job_id`, `product_code`, `task_type`, `provider_code`, and product source identifiers for faster cross-product incident triage.
- Platform now exposes a clearer service-to-service commercialization surface under `/internal/v1/*`, including write-side entry points (`controls/reservations`, `metering/events`, `metering/settlements/:eventID/reverse`, `commercial/route/resolve`) and read-side query entry points (`metering/settlements`, `metering/discounts`, `wallet/accounts`, `wallet/ledger`, `incentives/rewards`, `incentives/commissions`) for product backends such as `v-menu-backend`.
- Product scope is now part of the internal API contract rather than a caller convention only: subject-scoped commercialization read APIs must carry `product_code` by default, and platform must not silently widen the result set to all products when that field is omitted. If operators truly need a cross-product view, they must opt in explicitly with `include_all_products=true` on the supported internal read routes.
- Shared runtime orchestration is also live as an internal platform surface: `/internal/v1/runtime/providers`, `/internal/v1/runtime/jobs`, `/internal/v1/runtime/jobs/:runtimeJobID/attempts`, and `/internal/v1/runtime/charge-sessions` are already wired for product backends that want platform-owned execution and charge truth.
- Cross-module observability baseline now also covers internal caller tagging (`internalServiceName` / auth mode), access log enrichment, handler-level OTel spans across commercialization and identity-facing modules, broader audit coverage on wallet lifecycle / asset mutation endpoints, and Prometheus-standard HTTP + business metrics instead of ad-hoc in-memory rendering.
- Response and error handling now move closer to `v-backend` standards: typed response codes, message/status mapping, field-level validation error support, paginated response helper, and internal docs endpoints for `/api/v1/docs/internal-access` and `/api/v1/docs/error-codes`.
- Core handlers now actively use the upgraded response layer instead of only defining it: bind failures go through field-aware validation responses, create-style endpoints return `201`, and commercialization-related handlers start using finer business codes such as insufficient quota/credits/wallet, settlement reverse invalid, and route not found.
- Critical internal paths now also attach clearer failure context for debugging: handlers record concrete errors into the Gin error chain, access logs upgrade 4xx/5xx requests to warn/error with response metadata, and key routes such as internal auth, metering, control, wallet, storage, commercial route resolve, and runtime now return more stable `error_code` / `error_hint` values instead of only generic `failed to ...` messages.
- The same baseline should now be treated as the default for newly added modules as well: if a handler records an error in tracing, it should also attach that error into Gin context, prefer semantic response payloads for stable caller troubleshooting, and keep response hints action-oriented enough that product backends can decide whether to retry, fix request data, or escalate with request identifiers.
- To keep that rule consistent instead of relying on memory, platform handlers should prefer the shared response helpers for common failure patterns, especially the observed semantic error helpers that both attach the concrete error into Gin context and emit stable `error_code` / `error_hint` payloads in one step.
- That helper-based pattern is now already rolled through the main platform surfaces, including catalog, identity, organization, access, commercialization, control, metering, wallet, runtime, storage, referral/incentive, and channel-settlement handlers, so new code should follow the same style rather than re-introducing raw `ObserveError + JSONError...` pairs or generic `CodeInternalError` responses.
- Shared infrastructure strings should now also be treated as engineering constants instead of ad-hoc literals: request context keys, cross-service/internal headers, internal auth mode values, and reused status values belong in shared constant packages; user-facing messages, error hints, audit actions, metric names, and log/span event names may stay as descriptive protocol strings as long as they follow naming conventions and remain stable.
- The same principle applies one level deeper for domain-critical strings: when statuses, directions, billing subject types, settlement modes, reservation states, or wallet lifecycle values are reused across handlers, services, repositories, and tests, they should be promoted into shared domain constants rather than copied as raw literals in every flow.
- That domain-constant baseline is no longer limited to metering and wallet internals: control reservation flows, runtime job/session state transitions, middleware permission checks, and commercial bootstrap defaults should all consume the same shared constants where the value is part of cross-module behavior rather than a product-specific business copy.
- Repository queries are part of the same contract surface: if handlers and services already standardize on shared status, direction, subject-type, or settlement-mode constants, repository filters should reuse those exact constants as well so behavior cannot drift between orchestration code and persistence lookups.
- The same rule now extends into incentive and channel-finance flows as well: referral triggers, earned/pending/redeemed-style lifecycle states, settlement-in-progress transitions, and channel policy matching primitives should converge on shared constants wherever the same value appears in multiple write paths, resolution steps, and settlement updates.
- Service-layer critical paths should not rely on handler logs alone: long or stateful flows such as runtime dispatch/poll/result persistence, metering ingest/finalize/reverse, wallet ledger posting, allowance reset, and lifecycle jobs should emit structured begin/success/failure logs with stable identifiers so operators can trace where a request or async task stopped without reverse-engineering database state first.
- Startup and edge hardening should now be treated as part of the default platform baseline rather than follow-up work: non-debug configs must reject insecure default secrets, HTTP ingress should enforce body size and rate limits, JWT parsing must reject unexpected signing methods, and shared-secret middleware must use constant-time comparison for internal service authentication.
- First-wave internal APIs now also carry Swagger/OpenAPI annotations, and `./scripts/gen-swagger-internal.sh` is the committed generation entrypoint for the current internal spec. Browser-readable docs are also exposed at `/docs`, `/api/v1/docs/internal-access`, and `/api/v1/docs/error-codes`.
- Config shape now intentionally follows the broader `v-backend` pattern (`app/database/redis/security/oauth/monitoring`) even though only part of it is consumed today.
- Database and Redis initialization now consume the real infrastructure config shape and no longer stay at config-only placeholder level.

## 4. Principle

Business infrastructure can be platformized.
Business rules remain product-owned.
Shared data queries must be product-scoped by default.

If a product backend asks platform for wallet, reward, commission, discount, settlement, or usage data tied to a subject, the safe default is:

- query with `product_code`
- deny ambiguous unscoped reads
- expose cross-product reads only through an explicit operator-style opt-in

## 5. Engineering Baseline

The platform backend must not add commercialization modules without the following engineering baseline:

- OpenTelemetry trace for request path, internal calls, async jobs, and event consumers
- unified `request_id` / `trace_id` correlation
- Prometheus metrics and Grafana dashboards
- structured logging with business correlation fields
- audit logs for entitlement, billing profile, quota, credits, refund, and routing changes
- idempotency for metering events and payment/refund callbacks
- health checks, retry visibility, and compensation visibility

Commercialization capabilities are not considered complete if only tables and APIs exist but these operational abilities are missing.
