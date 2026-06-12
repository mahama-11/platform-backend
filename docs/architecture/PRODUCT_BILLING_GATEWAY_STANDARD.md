# Product Billing Gateway Standard

## 结论

Platform 当前商业化底座能力已经比较全，但暴露给产品侧的粒度过低：catalog、wallet、quota、reservation、charge session、metering finalization、settlement 都需要产品自己编排。这个复杂度不应该由每个产品重复承担。

后续标准是：

- Platform 继续拥有商业化真相源和低层账本能力。
- 产品后端不直接拼装低层商业化流程。
- Platform 提供一个稳定的 Product Billing Gateway / Product OAPI，把套餐、权益、预占、结算、释放封装成少量业务语义接口。
- Menu/Ecommerce/Novel 等产品只接 Gateway 合约和产品配置，不复制 wallet/quota/metering 编排逻辑。

## 目标

降低产品接入难度，同时保持财务、计量、权益、对账的可审计性。

产品侧应该回答：

- 这是什么业务动作？
- 对哪个组织/用户收费？
- 使用哪个产品能力/场景？
- 预计消耗多少？
- 成功或失败了吗？

平台侧负责：

- 找商品/套餐/权益/费率。
- 创建 charge session。
- 预占 quota/credits/wallet。
- runtime 绑定。
- 成功结算。
- 失败释放。
- 幂等、审计、对账、repair。

## 当前问题

### 低层 API 暴露过多

产品现在需要理解并正确组合：

- `/internal/v1/catalog/offerings`
- `/internal/v1/wallet/summary`
- `/internal/v1/controls/quota/balance`
- `/internal/v1/controls/package-activations`
- `/internal/v1/runtime/charge-sessions`
- `/internal/v1/controls/reservations`
- `/internal/v1/metering/finalizations`
- `/internal/v1/metering/settlements`

这导致每个产品都可能在以下环节出错：

- 创建了 runtime job 但没带 charge session。
- reserve 成功后 completed 没 finalize。
- failed/canceled 后没 release。
- 本地显示 credits 但平台没有 wallet/quota 事实。
- 有 billable item 但没有 SKU/package/rate/quota policy。
- settlement mode 选错，产品侧还不知道怎么解释。

### OAPI 不够产品语义化

现在 OAPI 更像“平台内部零件库”，不是“产品接入 SDK”。

产品真正需要的是少数稳定用例：

- 读当前商业化视图。
- 激活套餐/试用/购买权益。
- 开始一个收费动作。
- 绑定 runtime job。
- 完成收费动作。
- 取消/失败释放。
- 查询账本和对账状态。

## 标准接口分层

### Layer 0：Platform Internal Primitives

已有低层能力，继续保留，主要给 Platform 内部模块和少数高级产品适配器使用。

- catalog
- wallet
- quota
- reservation
- metering
- settlement
- runtime charge session

这些接口不作为普通产品接入首选。

### Layer 1：Product Billing Gateway

新增/沉淀产品接入主接口。推荐路径前缀：

```text
/internal/v2/product-billing/*
```

#### 1. Product commercial view

```http
GET /internal/v2/product-billing/commercial-view?product_code={product_code}&billing_subject_type=organization&billing_subject_id=...
```

返回产品可用商业化视图：

- product
- active offerings
- current package/plan if any
- wallet summary
- quota summary by billable item
- capabilities/entitlements
- display-safe pricing metadata

产品侧不要自己分别调用 catalog + wallet + quota 再拼。

#### 2. Activate package

```http
POST /internal/v2/product-billing/package-activations
```

输入：

```json
{
  "product_code": "menu",
  "package_code": "menu.pkg.trial.signup",
  "billing_subject_type": "organization",
  "billing_subject_id": "org-id",
  "activation_reason": "signup_trial",
  "reference_id": "user-or-order-id",
  "metadata": {"reference_type": "signup"}
}
```

Gateway 内部调用现有 `controls/package-activations`，并统一返回 wallet/quota delta。

#### 3. Begin billable action

```http
POST /internal/v2/product-billing/actions/begin
```

输入：

```json
{
  "product_code": "novel_video",
  "organization_id": "org-id",
  "user_id": "user-id",
  "source_type": "novel_video_job",
  "source_id": "job-id",
  "source_action": "video_generation",
  "task_type": "video_text_to_video",
  "billing_subject_type": "organization",
  "billing_subject_id": "org-id",
  "billable_item_code": "video_text_to_video",
  "resource_type": "quota",
  "estimated_units": 45,
  "unit": "video",
  "idempotency_key": "novel_video:job-id:begin",
  "metadata": {
    "mode": "text",
    "model": "v5.6",
    "quality": "720p",
    "duration": 5
  }
}
```

返回：

```json
{
  "action_id": "cs_...",
  "charge_session_id": "cs_...",
  "reservation_id": "res_...",
  "status": "reserved",
  "billable_item_code": "video_text_to_video",
  "estimated_units": 45
}
```

Gateway 内部负责：

- create charge session
- reserve resource
- 持久化 billing action
- 失败时状态可对账

#### 4. Bind runtime job

```http
POST /internal/v2/product-billing/actions/{billing_action_id}/bind-runtime
```

输入：

```json
{
  "runtime_job_id": "runtime-id",
  "metadata": {
    "provider_code": "pai_video",
    "provider_job_id": "provider-id"
  }
}
```

用于在平台账本里把 product job、runtime job、charge session 串起来。

#### 5. Complete billable action

```http
POST /internal/v2/product-billing/actions/{billing_action_id}/complete
```

输入：

```json
{
  "runtime_job_id": "runtime-id",
  "final_units": 45,
  "finalization_id": "fin_job-id",
  "event_id": "evt_job-id",
  "settlement_id": "settlement_job-id",
  "occurred_at": "2026-06-11T00:00:00Z",
  "dimensions": {
    "provider_job_id": "provider-id"
  }
}
```

Gateway 内部负责：

- metering finalization
- settlement record
- charge session settled
- billing action settled
- idempotent replay

#### 6. Cancel/release billable action

```http
POST /internal/v2/product-billing/actions/{billing_action_id}/release
```

输入：

```json
{
  "reason": "runtime_failed",
  "metadata": {
    "runtime_job_id": "runtime-id",
    "error_code": "PROVIDER_FAILED"
  }
}
```

Gateway 内部负责：

- release reservation
- charge session released
- billing action released
- release 失败进入 repair/reconcile

### Layer 2：Product SDK / Client Package

每个 Go 产品后端不应复制一套 Platform client 类型。

建议抽出共享 client 或生成 SDK：

```text
platform-product-oapi-go
```

至少包含：

- `CommercialView(ctx, input)`
- `ActivatePackage(ctx, input)`
- `BeginBillableAction(ctx, input)`
- `BindRuntime(ctx, input)`
- `CompleteBillableAction(ctx, input)`
- `ReleaseBillableAction(ctx, input)`

产品 repo 只维护业务 adapter，不维护低层账务编排。

## 产品接入标准

### 产品后端必须保存的最少字段

任何收费业务对象必须保存：

- `billing_action_id`
- `charge_session_id`
- `reservation_id`
- `runtime_job_id` if runtime-backed
- `billable_item_code`
- `estimated_units`
- `final_units`
- `billing_status`
- `settlement_id` if settled
- `billing_failure_code`
- `billing_failure_message`

如果不想污染主业务表，可建产品本地 charge intent 表。

### 状态机

```text
created -> reserved -> runtime_bound -> settled
created -> reserved -> released
created -> failed_need_reconcile
reserved -> failed_need_reconcile
runtime_bound -> failed_need_reconcile
```

completed 但未 settled，不允许对用户展示为“已扣费完成”。

### Fail-closed

- begin 失败：业务动作不得进入执行。
- reserve 失败：runtime job 不得创建。
- runtime create 失败：release reservation。
- runtime completed 但 complete/finalize 失败：业务结果可保留，但 billing_status 必须 `failed_need_reconcile`，并产生 repair evidence。
- runtime failed/canceled：release 失败也必须进入 reconcile。

## Catalog 标准

一个产品如果要展示套餐/订阅/积分包，Platform 至少必须存在：

- `product`
- `billable_items`
- `skus`
- `commercial_packages`
- `rate_cards`
- `quota_grant_policies` 或 `asset_definitions`

缺一类时，产品 UI 不得展示成真实可购买/可消费套餐，只能标记为配置缺失或隐藏入口。

## 复杂度降低策略

### 对产品团队

产品只学 4 个动作：

```text
view -> activate -> begin -> complete/release
```

不用理解 10 张平台表和 8 个低层 API。

### 对平台团队

平台保留低层能力，但把编排沉淀到 Gateway：

- billing action table
- idempotency
- repair/reconcile
- OAPI schema
- generated SDK
- contract tests
- downstream product smoke

### 对 QA / CI

任何产品接入必须通过统一 gate：

- catalog completeness gate
- begin action gate
- complete settlement gate
- failure release gate
- wallet/quota delta gate
- runtime charge binding gate

## Novel 对齐要求

Novel 不能继续直接拼低层接口。应收敛到 Product Billing Gateway：

1. Platform 为 `novel_video` 补齐 catalog：SKU/package/rate/quota/asset。
2. Novel 后端接 `CommercialView` 替代裸 `WalletSummary`。
3. Novel 生成提交调用 `BeginBillableAction`。
4. Runtime job 创建必须绑定 `charge_session_id` 和 `billing_action_id`。
5. Runtime completed 调 `CompleteBillableAction`。
6. Runtime failed/canceled 调 `ReleaseBillableAction`。
7. DEV 验收看 Platform 账本，不看本地 credits 字段。

## 验收口径

一次收费生成的 PASS 证据必须包含：

- product job id
- runtime job id
- charge session id
- reservation id
- billing action id
- meter event id
- settlement id
- charge session final status
- quota/wallet before/after delta

没有这些证据，只能算业务生成成功，不能算商业化接入成功。
