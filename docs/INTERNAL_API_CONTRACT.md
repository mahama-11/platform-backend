# 平台内部 API 契约

## 1. 目标

本文档约束 `v-platform-backend` 对产品后端的内部服务接入方式。

当前主要面向：

- `v-menu-backend`
- 后续其他产品后端

目标不是描述所有业务细节，而是定义稳定接入契约。

## 2. 路径分层

- 公共管理/控制台接口：`/api/v1/*`
- 内部服务接口：`/internal/v1/*`

`/internal/v1/*` 的职责：

- 产品后端调用平台共享能力
- 触发计量、结算、资源预占、路由决策
- 查询结算、折扣、钱包、奖励、返佣结果
- 在出现补偿场景时触发 reverse

## 3. 统一响应结构

所有接口必须返回统一 envelope：

```json
{
  "code": 0,
  "message": "success",
  "request_id": "req_xxx",
  "timestamp": 1710000000000,
  "data": {}
}
```

错误响应：

```json
{
  "code": 2001,
  "message": "Insufficient quota balance",
  "request_id": "req_xxx",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "timestamp": 1710000000000,
  "error": "insufficient quota balance",
  "error_code": "INSUFFICIENT_QUOTA",
  "error_hint": "Check quota balance or release stale reservations before retrying."
}
```

如需字段级错误，可追加 `errors`：

```json
{
  "code": 1000,
  "message": "Invalid parameter",
  "request_id": "req_xxx",
  "timestamp": 1710000000000,
  "error": "invalid request",
  "errors": [
    {
      "field": "event_id",
      "message": "event_id is required"
    }
  ]
}
```

## 4. 统一请求头

推荐请求头：

- `X-Internal-Service`
- `X-Internal-Timestamp`
- `X-Internal-Signature`
- `X-Request-ID`
- `traceparent`
- `X-Trace-ID`（兼容字段）

Correlation semantics:

- `X-Request-ID` is the stable business/customer-support correlation ID and should be preserved end-to-end.
- `traceparent` is the authoritative W3C/OpenTelemetry distributed tracing context.
- `X-Trace-ID` is retained for compatibility and response headers, but callers must not rely on it as the only cross-service tracing mechanism; when `traceparent` is present, Platform uses the OTel trace ID from `traceparent`.

兼容旧方式：

- `X-Internal-Service-Secret`

## 5. 幂等要求

- `POST /internal/v1/metering/events`
  - 必须传稳定的 `event_id`
  - 同一业务事件重试时，必须复用相同 `event_id`
- `POST /internal/v1/metering/finalizations`
  - 必须传稳定的 `finalization_id`、`reservation_id`、`event_id`
  - 同一业务动作重试时，必须复用相同 `finalization_id` 与 `event_id`
- `POST /internal/v1/metering/settlements/:eventID/reverse`
  - 调用方必须把一次冲正视为单一补偿动作
  - 在重试前应先查询 settlement 状态
- `POST /internal/v1/controls/reservations`
  - 调用方应传稳定的 `reservation_key`
  - 调用方应持久化 reservation 结果
  - 避免对同一业务动作重复 reserve

## 6. 产品作用域约束

对于 `v-platform-backend` 的内部查询接口，`product_code` 不再是“可选语义提示”，而是默认的数据作用域边界。

统一原则：

- 面向产品后端的共享读接口，默认必须带 `product_code`
- 不允许因为漏传 `product_code` 而回退成“返回该主体下所有产品数据”
- 如确有平台运营、审计、排障需要查询全量数据，必须显式传 `include_all_products=true`
- `product_code` 与 `include_all_products=true` 不能同时使用
- 产品后端日常业务链路不得依赖 `include_all_products=true`

当前已按该原则收口的内部读接口包括：

- `GET /internal/v1/wallet/accounts`
- `GET /internal/v1/wallet/summary`
- `GET /internal/v1/wallet/ledger`
- `GET /internal/v1/incentives/rewards`
- `GET /internal/v1/incentives/commissions`
- `GET /internal/v1/metering/usage-summary`
- `GET /internal/v1/metering/settlements`
- `GET /internal/v1/metering/discounts`

推荐调用方式：

```text
GET /internal/v1/wallet/accounts?billing_subject_type=organization&billing_subject_id=org_123&product_code=ecommerce
```

受控全量方式：

```text
GET /internal/v1/wallet/accounts?billing_subject_type=organization&billing_subject_id=org_123&include_all_products=true
```

错误约束：

- 未传 `product_code` 且未显式声明 `include_all_products=true`：拒绝返回数据
- 同时传 `product_code` 与 `include_all_products=true`：拒绝返回数据
- 产品后端需要产品级视图时，应先按 `product_code` 缩域，再做自己的产品聚合展示

## 7. 读写闭环

建议接入模式：

1. 写入行为
2. 查询结果
3. 失败时补偿

示例：

1. `POST /internal/v1/controls/reservations`
2. 执行业务动作
3. `POST /internal/v1/metering/finalizations`
4. 如无需预占直结，也可直接 `POST /internal/v1/metering/events`
5. `GET /internal/v1/metering/settlements/:eventID`
6. 如需补偿：`POST /internal/v1/metering/settlements/:eventID/reverse`

## 8. 错误处理建议

- `4xx`
  - 优先判断是否为参数错误、权限错误、业务冲突
  - 不应无脑重试
- `409`
  - 优先查询当前资源状态
  - 常见于余额不足、状态非法、重复补偿
- `5xx`
  - 可按幂等策略重试
  - 重试后应查询资源最终状态，不应只依据最后一次 HTTP 结果判断

## 9. 当前推荐接入接口

- `POST /internal/v1/controls/reservations`
- `POST /internal/v1/controls/reservations/:reservationID/commit`
- `POST /internal/v1/controls/reservations/:reservationID/release`
- `POST /internal/v1/metering/events`
- `POST /internal/v1/metering/finalizations`
- `GET /internal/v1/metering/settlements`
- `GET /internal/v1/metering/settlements/:eventID`
- `POST /internal/v1/metering/settlements/:eventID/reverse`
- `GET /internal/v1/metering/discounts`
- `GET /internal/v1/runtime/capabilities`
  - Read-only product-scoped runtime capability matrix.
  - Required query: `product_code`; optional query: `task_type`.
  - Returns known P0 runtime task types (`image_understanding`, `ocr`, `image_generation`, `image_inpainting`, `video_keyframe`) with provider/callback/storage/billing readiness and stable reason codes such as `contract-needed`, `provider_binding_missing`, `storage_binding_missing`, and `billable_item_missing`.
- `POST /internal/v1/runtime/jobs`
  - Creates a product-scoped runtime job and enqueues dispatch atomically.
  - Idempotency is scoped to `product_code + organization_id + source_type + source_id + task_type + idempotency_key`; the same client key may be reused by a different product/org/source/task boundary without colliding.
  - A replay within the same scope returns the existing job. A semantic boundary conflict detected by older clients or stale storage returns HTTP `409` with `RUNTIME_JOB_IDEMPOTENCY_CONFLICT`.
- `POST /api/v1/controls/package-activations`
  - Admin/JWT Platform Console route handled by `ActivatePackage`; same request/response envelope and fail-closed semantics as the internal route.
- `POST /internal/v1/controls/package-activations`
  - Generic package activation API handled by `ActivatePackage` and used by product backends after they choose a business package/campaign.
  - Required body: `product_code`, `package_code`, `billing_subject_type`, `billing_subject_id`, stable `reference_id`; optional `activation_reason`, `metadata` (JSON object or JSON string accepted; object metadata is stored as compact JSON).
  - Response envelope contains `product_code`, `package_code`, `billing_subject_type`, `billing_subject_id`, `activation_reason`, `reference_id`, `quota_grants`, `capability_grants`, `granted_quota_units`, and `idempotent`.
  - Platform resolves active `quota_grant_policies` and `package_capability_policies`, applies quota/capability grants idempotently by `reference_id`, and fails closed when the package is inactive/missing or has no active policies.
- `POST /internal/v1/incentives/rewards`
- `GET /internal/v1/incentives/referral-programs`
- `POST /internal/v1/incentives/referral-programs`
- `GET /internal/v1/incentives/referral-codes`
- `GET /internal/v1/incentives/referral-codes/:code/resolve`
- `POST /internal/v1/incentives/referral-codes`
- `GET /internal/v1/incentives/referral-conversions`
- `POST /internal/v1/incentives/referral-conversions`
- `GET /internal/v1/incentives/channel-partners`
- `POST /internal/v1/incentives/channel-partners`
- `GET /internal/v1/incentives/channel-programs`
- `POST /internal/v1/incentives/channel-programs`
- `GET /internal/v1/incentives/channel-bindings`
- `POST /internal/v1/incentives/channel-bindings`
- `GET /internal/v1/incentives/channel-policies`
- `POST /internal/v1/incentives/channel-policies`
- `GET /internal/v1/incentives/channel-policy-versions`
- `POST /internal/v1/incentives/channel-policy-versions`
- `GET /internal/v1/incentives/channel-policy-assignments`
- `POST /internal/v1/incentives/channel-policy-assignments`
- `GET /internal/v1/incentives/channel-profit-snapshots`
- `GET /internal/v1/incentives/channel-adjustments`
- `POST /internal/v1/incentives/channel-adjustments`
- `POST /internal/v1/incentives/channel-policy-resolution-preview`
- `GET /internal/v1/incentives/channel-commissions`
- `GET /internal/v1/incentives/channel-clawbacks`
- `GET /internal/v1/incentives/channel-settlement-batches`
- `GET /internal/v1/incentives/channel-settlement-batches/:batchID`
- `POST /internal/v1/incentives/channel-settlement-batches`
- `POST /internal/v1/incentives/channel-settlement-batches/:batchID/confirm`
- `POST /internal/v1/incentives/channel-settlement-batches/:batchID/process`
- `POST /internal/v1/incentives/channel-settlement-batches/:batchID/close`
- `POST /internal/v1/incentives/channel-settlement-batches/:batchID/cancel`
- `GET /internal/v1/incentives/channel-settlement-items`
- `POST /internal/v1/incentives/channel-events/charges`
- `POST /internal/v1/incentives/channel-events/refunds`
- `PUT /internal/v1/users/:userID/profile`
- `PUT /internal/v1/orgs/:orgID/profile`
- `POST /internal/v1/commercial/route/resolve`
- `GET /internal/v1/wallet/accounts`
- `GET /internal/v1/wallet/summary`
- `GET /internal/v1/wallet/buckets`
- `GET /internal/v1/wallet/ledger`
- `GET /internal/v1/wallet/assets`
- `POST /internal/v1/wallet/assets`
- `GET /internal/v1/wallet/allowance-policies`
- `POST /internal/v1/wallet/allowance-policies`
- `POST /internal/v1/wallet/cycle-allowances`
- `POST /internal/v1/wallet/expire`
- `POST /internal/v1/wallet/lifecycle/run`
- `GET /internal/v1/incentives/rewards`
- `GET /internal/v1/incentives/commissions`
- `POST /internal/v1/incentives/commissions/redeem`
- `POST /internal/v1/incentives/channel-events/charges`
  - 产品上报一笔可分佣消费事件
  - 当前阶段幂等优先基于 `event_id`，并对 `product_code + source_charge_id` 做重复保护
  - 现支持两套路径：
    - 旧路径：按 `net_collected_amount | paid_amount` 的固定比例分佣
    - 新路径：命中 `channel-policy-version + assignment` 后，按 `distributable_profit_amount` 或其他 version 定义口径分佣
  - 当调用方传入以下可选字段时，平台会自动生成 `channel_profit_snapshot`：
    - `payment_fee_amount`
    - `tax_amount`
    - `service_delivery_cost_amount`
    - `infra_variable_cost_amount`
    - `risk_reserve_amount`
    - `manual_adjustment_amount`
    - `commission_recognition_at`
    - `snapshot_basis`
    - `policy_version_id`（仅内部受控 override 场景）
  - 若没有有效 binding 或没有命中 policy，返回成功但 `matched=false`
- `POST /internal/v1/incentives/channel-events/refunds`
  - 产品上报退款 / 撤销事件
  - 若原佣金尚未结算，则直接反转原渠道佣金账本
  - 若原佣金已结算，则生成 `clawback` 记录供后续结算批次抵扣
- `POST /internal/v1/incentives/channel-policy-versions`
  - 创建不可变 policy version
  - 当前阶段支持 `commission_base = paid_amount | net_collected_amount | distributable_profit_amount`
  - `distributable_profit_amount` 需要同时提供 `profit_basis_config`
- `POST /internal/v1/incentives/channel-policy-assignments`
  - 创建 policy assignment
  - 用于表达 `contract_override / binding_override / partner_program_assignment / product_default_assignment`
  - 平台会拒绝同一 scope、同一时间窗的 active assignment 重叠
- `GET /internal/v1/incentives/channel-profit-snapshots`
  - 查询平台已持久化的利润快照
  - 主要用于审计、回放、排障与规则验证
- `POST /internal/v1/incentives/channel-adjustments`
  - 创建渠道补差 / 追溯修正分录
  - 当前支持：
    - `manual_credit`
    - `manual_debit`
    - `reprice_delta`
    - `cost_true_up`
    - `dispute_resolution`
  - adjustment 会在后续 settlement batch 中按 partner + currency 聚合进入 `adjustment_amount`
- `GET /internal/v1/incentives/channel-adjustments`
  - 查询 adjustment ledger
  - 主要用于运营排障、争议处理和结算核对
- `POST /internal/v1/incentives/channel-policy-resolution-preview`
  - 对单笔 charge 输入执行 dry-run 预演
  - 返回：
    - 命中的 binding
    - 命中的 policy / policy version / assignment
    - 候选 assignment 快照
    - 利润快照
    - 佣金、holdback、settleable 预演结果
  - 不持久化 ledger，也不写入 settlement
- `POST /internal/v1/incentives/channel-bindings`
  - 当前阶段同一 `product_code + org_id` 同时只允许一条有效 binding
  - 若已有活动 binding 且未过锁定期，返回冲突
- `POST /internal/v1/incentives/channel-settlement-batches`
  - 生成渠道结算批次
  - 生成前会先成熟所有 `available_at <= period_end` 的 `pending commission`
  - 会归集所有未被非取消批次占用的 `earned commission` 与 `pending clawback`
- `POST /internal/v1/incentives/channel-settlement-batches/:batchID/confirm`
  - 将批次从 `generated` 推进到 `confirmed`
  - 批次内 item 会同步从 `pending` 推进到 `confirmed`
- `POST /internal/v1/incentives/channel-settlement-batches/:batchID/process`
  - 将批次从 `confirmed` 推进到 `processing`
  - 批次关联的佣金账本会从 `earned` 置为 `settlement_in_progress`
- `POST /internal/v1/incentives/channel-settlement-batches/:batchID/close`
  - 将批次从 `processing` 推进到 `closed`
  - 批次关联的佣金账本会置为 `settled`
  - 批次关联的 clawback 会置为 `applied`
- `POST /internal/v1/incentives/channel-settlement-batches/:batchID/cancel`
  - 当前只允许取消 `generated` 或 `confirmed` 批次
  - 已进入 `processing` 的批次不能直接取消
- `GET /internal/v1/incentives/channel-settlement-batches/:batchID`
  - 返回批次主对象、结算 item 以及 item 关联的 ledger / clawback ID

## 9. 生命周期与运维语义补充

- `GET /internal/v1/wallet/summary`
  - 返回产品维度多资产汇总，不再假设只有单一余额
  - 当前会区分 `permanent_balance`、`reward_balance`、`allowance_balance`
- `POST /internal/v1/wallet/allowance-policies`
  - 定义周期额度策略真相源
  - 自动任务会据此生成当期 `cycle_reset` bucket
- `POST /internal/v1/wallet/expire`
  - 手动执行 expiring bucket 过期
  - 主要用于补跑、排障、运维恢复
- `POST /internal/v1/wallet/lifecycle/run`
  - 手动执行一次完整生命周期任务
  - 包括过期扫描与周期额度重置

## 10. 观测与排障约定

- `/internal/v1/*` 接口应统一携带 `request_id` / `trace_id`
- 平台当前已统一到 OTel tracing + Prometheus metrics 管道
- 钱包生命周期、计量、返佣、控制类写接口应具备：
  - handler 级 span
  - 业务计数器
  - 可审计 mutation 记录

### 10.1 Platform Audit / Diagnostics 查询接口

以下接口是平台管理员排障用 public API，使用 JWTAuth + `platform.admin` 权限，并沿用统一 `JSONSuccess` 响应 envelope。

- `GET /api/v1/audit/logs` (`auditHandler.ListLogs`)
  - 查询 `platform_audit_logs`，不写入 access log DB。
  - Query filters: `query`（匹配 `request_id`、`trace_id`、`actor_user_id`、`actor_org_id`、`action`、`target_type`、`target_id`、`route`、`details`）、`action`、`target_type`、`status`、`actor_user_id`、`actor_org_id`、`request_id`、`trace_id`、`limit`、`offset`。
  - Pagination: `limit` 默认 50、最大 200；`offset` 最小 0；排序为 `created_at DESC`。
  - Response data: `{items,total,limit,offset,stats}`；`stats` 包含 `total`、`success_count`、`failure_count`、`distinct_actions`、`latest_created_at`、`by_status`、`by_action`、`by_target_type`。
- `GET /api/v1/audit/logs/:auditID` (`auditHandler.GetLog`)
  - 按 audit ID 返回单条 `platform_audit_logs` 详情。
  - 404 使用统一 error envelope，调用方应校验 audit ID 是否存在。

## 11. Swagger / OpenAPI 现状

平台当前已经具备两类文档基础：

- internal service API 的 handler 注解与生成产物
- 浏览器可读的基础文档入口与错误码文档

当前可用能力：

- `./scripts/gen-swagger-internal.sh`
- `docs/openapi/README.md` 中维护当前覆盖范围
- `/docs`、`/api/v1/docs/internal-access`、`/api/v1/docs/error-codes` 提供浏览器侧基础文档入口

当前需要注意：

- 仓库里当前可见的 Swagger 生成脚本是 internal 用的 `./scripts/gen-swagger-internal.sh`
- 若后续补 public API 独立 OpenAPI 产物，应新增明确脚本或生成入口后再写入文档

仍需持续补齐的不是“是否有 OpenAPI”，而是：

- 扩大更多模块和错误语义的注解覆盖
- 保持生成产物与实际路由同步
- 在新增 internal/public 接口时同步更新 README 与注解
