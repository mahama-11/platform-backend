# 渠道分佣数据模型与状态机设计

## 1. 目标

本文档在全局文档 `docs/architecture/CHANNEL_PARTNER_REVENUE_SHARE.md` 的基础上，进一步细化 `v-platform-backend` 侧的详细设计，回答以下问题：

- 平台侧应新增哪些核心模型
- 每个模型的职责边界是什么
- 渠道绑定、佣金账本、结算批次、冲正追缴应如何设计状态机
- `Menu` 等产品后端应向平台发送什么事件
- 如何在不推翻现有 referral / commission 基础能力的前提下平滑演进

本文档的定位是：

- 面向平台侧数据模型与状态流的详细设计
- 面向后续 handler / service / repository / migration 的实现前规格

本文档不直接定义：

- 产品前端页面细节
- 渠道合同模板
- 税务与打款的法务细则

## 2.1 当前实现状态

截至当前代码现实，平台侧已经落地以下能力：

- `channel_partner / channel_program / channel_partner_binding / channel_commission_policy`
- `channel_commission_ledger / channel_clawback_ledger`
- 消费事件入站与退款事件入站
- 渠道结算批次生成、确认、处理、关闭、取消
- 结算 item 与 item -> ledger / clawback 关联表
- 面向 admin/internal 的查询接口

当前仍未落地的部分包括：

- 自动打款编排
- 财务导出格式适配
- 手工调整 settlement item 金额
- 风控冻结规则引擎

## 2. 设计原则

- 平台只保存通用商业化抽象，不保存 Menu 专属业务词
- 产品负责“哪些消费可分佣”，平台负责“如何记账、如何结算、如何冲正”
- 一笔消费对应的佣金结果必须可回放、可审计、可冲正
- 结算真相必须来源于账本与事件，不依赖产品侧临时聚合
- 退款、撤销、拒付必须是正向模型的一等公民，不能作为补丁逻辑
- 所有写入链路都必须支持幂等

## 3. 设计范围

### 3.1 阶段 1 目标

阶段 1 详细设计覆盖：

- 单层渠道商
- 单产品单客户唯一有效渠道归因
- 按有效消费比例分佣
- `pending -> earned -> settled` 的标准闭环
- 退款 / 取消触发冲正
- 月结批次

### 3.2 阶段 1 不做

- 多级代理链
- reseller 二次定价差价模型
- 自动打款编排
- 税票引擎
- 部分退款的精细拆分算法
- 动态分润路由

## 4. 平台核心对象

### 4.1 `channel_partner`

表示渠道主体，是后续归因、分佣、结算的对手方主体。

建议字段：

- `id`
- `code`
- `name`
- `partner_type`
- `settlement_subject_type`
- `status`
- `risk_level`
- `country_code`
- `default_currency`
- `contact_profile`
- `metadata`
- `created_at`
- `updated_at`
- `disabled_at`

建议枚举：

- `partner_type`: `channel`, `agent`, `reseller`, `sales_partner`
- `settlement_subject_type`: `individual`, `company`
- `status`: `draft`, `active`, `frozen`, `terminated`
- `risk_level`: `low`, `medium`, `high`

说明：

- `channel_partner` 回答的是“谁拿佣金”
- 它不是客户组织，也不是支付主体
- 是否允许结算，由 `status + 风控规则 + 结算配置` 决定

### 4.2 `channel_program`

表示渠道计划，用于表达“当前按哪套渠道政策执行”。

建议字段：

- `id`
- `code`
- `name`
- `program_type`
- `status`
- `product_scope`
- `default_settlement_cycle`
- `default_cooldown_days`
- `default_holdback_ratio`
- `metadata`
- `created_at`
- `updated_at`

建议枚举：

- `program_type`: `signup_referral`, `channel_revenue_share`
- `status`: `draft`, `active`, `paused`, `archived`

说明：

- 渠道计划是“政策容器”
- 不直接写死费率，而是与 policy 关联
- 未来 referral 和 revenue share 可以共用此模型

### 4.3 `channel_partner_programs`

表示某渠道参与了哪些计划，以及它在该计划中的个性化覆盖配置。

建议字段：

- `id`
- `channel_partner_id`
- `channel_program_id`
- `status`
- `effective_from`
- `effective_to`
- `override_settlement_cycle`
- `override_cooldown_days`
- `override_holdback_ratio`
- `metadata`
- `created_at`
- `updated_at`

说明：

- 允许在计划通用配置上为特定渠道做覆盖
- 阶段 1 可以只做少量覆盖字段

### 4.4 `channel_partner_binding`

表示客户归因绑定，是“这个客户当前归谁”的真相源。

建议字段：

- `id`
- `product_code`
- `org_id`
- `channel_partner_id`
- `channel_program_id`
- `binding_source`
- `source_code`
- `source_ref_id`
- `binding_scope`
- `status`
- `effective_from`
- `effective_to`
- `locked_until`
- `replaced_by_binding_id`
- `reason_code`
- `evidence`
- `created_by`
- `created_at`
- `updated_at`

建议枚举：

- `binding_source`: `signup_code`, `manual_assign`, `contract_import`, `system_backfill`
- `binding_scope`: `product_org`
- `status`: `pending`, `active`, `superseded`, `expired`, `canceled`

说明：

- 阶段 1 建议唯一约束：
  - 同一 `product_code + org_id` 同时只能有一条 `active` 绑定
- `locked_until` 用于避免频繁抢归因
- `evidence` 用于保留绑定证据，例如注册邀请码、合同编号、审批单号

### 4.5 `channel_commission_policy`

表示佣金策略，是“按什么口径算”的规则对象。

建议字段：

- `id`
- `channel_program_id`
- `product_code`
- `policy_code`
- `status`
- `applies_to`
- `trigger_type`
- `commission_base`
- `rate_type`
- `fixed_rate`
- `rate_config`
- `cooldown_days`
- `settlement_cycle`
- `holdback_ratio`
- `priority`
- `effective_from`
- `effective_to`
- `metadata`
- `created_at`
- `updated_at`

建议枚举：

- `status`: `draft`, `active`, `paused`, `archived`
- `applies_to`: `signup_order`, `subscription_initial`, `subscription_renewal`, `wallet_recharge`, `usage_charge`
- `trigger_type`: `charge_recorded`, `statement_finalized`
- `commission_base`: `net_collected_amount`, `paid_amount`, `net_revenue`
- `rate_type`: `fixed_rate`, `tiered`
- `settlement_cycle`: `weekly`, `biweekly`, `monthly`

说明：

- 阶段 1 推荐只实现：
  - `rate_type = fixed_rate`
  - `commission_base = net_collected_amount`
- `priority` 用于同一产品下多条策略命中时决策

### 4.6 `channel_commission_ledger`

表示佣金账本，是分佣真相源的主表。

建议字段：

- `id`
- `ledger_no`
- `product_code`
- `channel_partner_id`
- `channel_program_id`
- `binding_id`
- `policy_id`
- `source_event_id`
- `source_charge_id`
- `source_statement_id`
- `source_refund_id`
- `currency`
- `gross_amount`
- `discount_amount`
- `paid_amount`
- `refunded_amount`
- `net_collected_amount`
- `commissionable_amount`
- `commission_rate`
- `commission_amount`
- `holdback_amount`
- `settleable_amount`
- `status`
- `available_at`
- `earned_at`
- `settled_at`
- `reversed_at`
- `reversal_of_ledger_id`
- `dimensions`
- `metadata`
- `created_at`
- `updated_at`

建议枚举：

- `status`: `pending`, `earned`, `frozen`, `settlement_in_progress`, `settled`, `reversed`, `void`

说明：

- 这是渠道收益的 SSOT 主表
- `commission_amount` 是理论佣金
- `holdback_amount` 是留存金额
- `settleable_amount = commission_amount - holdback_amount`
- 一笔原始消费允许产生多条账本，但阶段 1 建议一笔 charge 对应一条主佣金账本

### 4.7 `channel_clawback_ledger`

表示已确认或已结算佣金的反向追缴记录。

建议字段：

- `id`
- `product_code`
- `channel_partner_id`
- `source_commission_ledger_id`
- `source_refund_event_id`
- `clawback_type`
- `currency`
- `clawback_amount`
- `reason_code`
- `status`
- `applied_in_settlement_batch_id`
- `metadata`
- `created_at`
- `updated_at`

建议枚举：

- `clawback_type`: `refund`, `chargeback`, `manual_adjustment`
- `status`: `pending`, `applied`, `waived`, `void`

说明：

- 若原佣金未结算，优先直接反转原账本
- 若原佣金已结算，生成 `clawback_ledger`

### 4.8 `channel_settlement_batch`

表示一次结算批次。

建议字段：

- `id`
- `batch_no`
- `product_code`
- `channel_program_id`
- `settlement_cycle`
- `period_start`
- `period_end`
- `currency`
- `status`
- `total_partner_count`
- `total_item_count`
- `gross_commission_amount`
- `gross_clawback_amount`
- `net_settleable_amount`
- `generated_at`
- `confirmed_at`
- `closed_at`
- `metadata`
- `created_at`
- `updated_at`

建议枚举：

- `status`: `draft`, `generated`, `confirmed`, `processing`, `closed`, `canceled`

说明：

- 阶段 1 可先做到结算批次与明细出数
- 自动打款可以后续在 `processing -> payout_completed` 再扩展

### 4.9 `channel_settlement_item`

表示某个结算批次下单个渠道的结算明细。

建议字段：

- `id`
- `settlement_batch_id`
- `channel_partner_id`
- `currency`
- `commission_amount`
- `clawback_amount`
- `adjustment_amount`
- `net_amount`
- `status`
- `statement_snapshot`
- `metadata`
- `created_at`
- `updated_at`

建议枚举：

- `status`: `pending`, `confirmed`, `processing`, `completed`, `failed`, `canceled`

说明：

- 一个批次下每个渠道一条 item，再与具体 ledger 明细关联
- 若后续接打款系统，`item` 是最自然的执行对象

## 5. 关系模型

推荐关系如下：

1. 一个 `channel_partner` 可以参与多个 `channel_program`
2. 一个 `channel_program` 可以绑定多条 `channel_commission_policy`
3. 一个 `product_code + org_id` 在同一时刻只有一条 `active` 的 `channel_partner_binding`
4. 一条 `binding` 可以命中多笔消费
5. 一笔消费事件可以生成一条主 `channel_commission_ledger`
6. 一条 `channel_commission_ledger` 可被反转或被追缴
7. 一个 `channel_settlement_batch` 包含多个 `channel_settlement_item`
8. 一个 `channel_settlement_item` 汇总多条 `commission_ledger` 和 `clawback_ledger`

建议增加关联表：

- `channel_settlement_item_ledgers`
- `channel_settlement_item_clawbacks`

## 6. 状态机设计

### 6.1 `channel_partner_binding` 状态机

状态：

- `pending`
- `active`
- `superseded`
- `expired`
- `canceled`

推荐流转：

1. 渠道码注册、人工指派、合同导入后创建 `pending`
2. 校验通过后转为 `active`
3. 若被新的有效绑定替换，则旧记录转为 `superseded`
4. 到达有效期终点可转为 `expired`
5. 若绑定无效或被撤销，可转为 `canceled`

规则：

- 同一 `product_code + org_id` 新绑定要生效前，必须处理旧 `active`
- 若 `locked_until` 未过期，不允许普通操作直接改绑
- 人工改绑必须写审计

### 6.2 `channel_commission_ledger` 状态机

状态：

- `pending`
- `earned`
- `frozen`
- `settlement_in_progress`
- `settled`
- `reversed`
- `void`

推荐流转：

1. 产品消费事件入站并命中绑定与策略后，生成 `pending`
2. 超过冷静期 / 观察期后，转为 `earned`
3. 若命中风控或人工冻结，转为 `frozen`
4. 被结算批次选中后，转为 `settlement_in_progress`
5. 结算完成后，转为 `settled`
6. 若在结算前发生退款 / 撤销，则转为 `reversed`
7. 若事件本身无效或人工废弃，可转为 `void`

规则：

- `pending` 与 `earned` 都可以在结算前被反转
- `settled` 不直接改成 `reversed`，而是通过 `clawback_ledger` 补偿

### 6.3 `channel_clawback_ledger` 状态机

状态：

- `pending`
- `applied`
- `waived`
- `void`

推荐流转：

1. 已结算佣金遇到退款 / 拒付时，创建 `pending`
2. 在下个结算批次中被核销后，转为 `applied`
3. 若人工豁免，转为 `waived`
4. 若源事件被判定无效，转为 `void`

### 6.4 `channel_settlement_batch` 状态机

状态：

- `draft`
- `generated`
- `confirmed`
- `processing`
- `closed`
- `canceled`

推荐流转：

1. 系统按周期出批，创建 `draft`
2. 汇总完成后转为 `generated`
3. 运营 / 财务确认后转为 `confirmed`
4. 开始执行结算流程后转为 `processing`
5. 全部 item 完成后转为 `closed`
6. 若批次作废则转为 `canceled`

规则：

- `generated` 前允许重算
- `confirmed` 后不应直接改账本金额，只能通过补充调整项修正

### 6.5 `channel_settlement_item` 状态机

状态：

- `pending`
- `confirmed`
- `processing`
- `completed`
- `failed`
- `canceled`

说明：

- 阶段 1 即使没有真实打款，也建议保留此状态机
- 未来接 payout / finance export 时可以自然扩展

## 7. 事件契约

### 7.1 设计原则

- 产品发标准化事件，平台做幂等、归因、落账
- 平台不直接理解 Menu 的内部 job 表、充值表、订单表
- 事件侧要能追溯到业务源对象，但不复制完整业务模型

### 7.2 推荐事件类型

阶段 1 推荐支持：

- `channel_customer_bound`
- `billable_charge_recorded`
- `billable_charge_refunded`
- `billable_charge_voided`

后续可加：

- `billing_statement_finalized`
- `channel_binding_superseded`

### 7.3 `channel_customer_bound`

用于创建或激活客户归因绑定。

建议字段：

- `event_id`
- `request_id`
- `trace_id`
- `event_type`
- `product_code`
- `org_id`
- `channel_code`
- `channel_partner_id`
- `program_code`
- `binding_source`
- `source_ref_id`
- `occurred_at`
- `idempotency_key`
- `metadata`

幂等建议：

- `idempotency_key = product_code + org_id + source_ref_id + binding_source`

### 7.4 `billable_charge_recorded`

用于表达一笔可参与分佣的有效消费。

建议字段：

- `event_id`
- `request_id`
- `trace_id`
- `event_type`
- `product_code`
- `org_id`
- `user_id`
- `billable_item_code`
- `source_charge_id`
- `source_order_id`
- `currency`
- `gross_amount`
- `discount_amount`
- `paid_amount`
- `refunded_amount`
- `net_collected_amount`
- `occurred_at`
- `billing_profile_key`
- `dimensions`
- `idempotency_key`

规则：

- 产品只上传事实金额与业务维度
- 平台负责根据 policy 算出 `commissionable_amount` 和 `commission_amount`

### 7.5 `billable_charge_refunded`

用于表达已入账消费被退款或撤销。

建议字段：

- `event_id`
- `request_id`
- `trace_id`
- `event_type`
- `product_code`
- `org_id`
- `source_charge_id`
- `source_refund_id`
- `refund_amount`
- `refund_type`
- `occurred_at`
- `reason_code`
- `idempotency_key`

建议枚举：

- `refund_type`: `full_refund`, `partial_refund`, `chargeback`, `void`

阶段 1 建议先只支持：

- `full_refund`
- `void`

### 7.6 事件样例

```json
{
  "event_id": "evt_channel_charge_202604190001",
  "request_id": "req_1777000000001",
  "trace_id": "tr_1777000000001",
  "event_type": "billable_charge_recorded",
  "product_code": "menu_ai",
  "org_id": "org_123",
  "user_id": "user_456",
  "billable_item_code": "menu_wallet_recharge",
  "source_charge_id": "chg_789",
  "source_order_id": "ord_789",
  "currency": "CNY",
  "gross_amount": 30000,
  "discount_amount": 0,
  "paid_amount": 30000,
  "refunded_amount": 0,
  "net_collected_amount": 30000,
  "occurred_at": "2026-04-19T12:00:00Z",
  "billing_profile_key": "menu_default_cn",
  "dimensions": {
    "scene": "wallet_recharge",
    "client_type": "web"
  },
  "idempotency_key": "menu_ai:chg_789:recorded"
}
```

## 8. 计算与落账规则

### 8.1 标准入账步骤

对 `billable_charge_recorded`，平台推荐执行：

1. 幂等检查
2. 找到当前有效 `channel_partner_binding`
3. 找到命中的 `channel_commission_policy`
4. 计算 `commissionable_amount`
5. 计算 `commission_amount`
6. 计算 `holdback_amount`
7. 生成 `channel_commission_ledger(status=pending)`

### 8.2 佣金公式

阶段 1 推荐公式：

- `commissionable_amount = net_collected_amount`
- `commission_amount = commissionable_amount * fixed_rate`
- `holdback_amount = commission_amount * holdback_ratio`
- `settleable_amount = commission_amount - holdback_amount`

### 8.3 退款处理步骤

对 `billable_charge_refunded`，平台推荐执行：

1. 幂等检查
2. 找到 `source_charge_id` 对应的佣金账本
3. 若账本状态为 `pending/earned/frozen`
   - 直接反转账本为 `reversed`
4. 若账本状态为 `settled`
   - 生成 `channel_clawback_ledger(status=pending)`
5. 写审计与补偿事件

## 9. 幂等与一致性

### 9.1 幂等对象

以下对象必须按自然键或业务幂等键控制重复入账：

- 渠道绑定
- 消费事件
- 退款事件
- 结算批次生成
- 结算批次确认

### 9.2 推荐幂等键

- 绑定事件：`product_code + org_id + source_ref_id + binding_source`
- 消费事件：`product_code + source_charge_id + event_type`
- 退款事件：`product_code + source_refund_id + event_type`
- 结算批次：`product_code + channel_program_id + period_start + period_end`

### 9.3 一致性建议

- 事件接入层与账本写入建议在同一平台服务内完成
- 结算批次应以“已确认账本快照”运行，不要动态实时汇总未锁定数据
- 所有跨期追缴必须保留来源账本 ID

## 10. 与现有 referral / commission 的兼容路径

### 10.1 兼容目标

不能推倒当前已有 referral / commission 能力重来。

推荐演进方式：

- 现有 referral 保持可用
- 新增渠道分佣详细模型
- 在服务层逐步抽象出更通用的 incentive / partner service

### 10.2 推荐分层

- `referral_program`
  - 偏“注册归因 / 邀请奖励”
- `channel_program`
  - 偏“长期消费分成”
- `commission_ledger`
  - 可逐步抽象为共享佣金账本基类或统一域

### 10.3 阶段 1 的务实策略

阶段 1 可以接受：

- 代码层先并存
- 设计层先统一

但新增渠道消费分佣时，必须以本文档模型为准，避免继续扩散一次性 referral 特化结构。

## 11. 实施顺序

建议按以下顺序推进：

1. 确认模型字段与枚举
2. 设计 migration 与唯一索引
3. 设计内部 API contract
4. 实现事件接入与幂等
5. 实现绑定服务
6. 实现佣金入账服务
7. 实现退款反转 / 追缴服务
8. 实现结算批次服务
9. 提供 Menu 聚合查询接口依赖的内部读模型

## 12. 最终结论

平台侧详细设计应以以下闭环为核心：

1. `binding` 解决归因真相
2. `policy` 解决计算规则
3. `commission_ledger` 解决佣金真相
4. `clawback_ledger` 解决已结算反向追缴
5. `settlement_batch + settlement_item` 解决周期结算
6. 标准化事件契约解决产品接入一致性

只要这 6 层设计收紧，后续接口、代码、前端、运营流程都能围绕同一闭环逐步落地。
