# 渠道净利润分佣与 Policy 覆盖决策设计

## 1. 目标

本文档在现有 [CHANNEL_PARTNER_MODEL_AND_STATES.md](./CHANNEL_PARTNER_MODEL_AND_STATES.md) 基础上，进一步深化两个尚未工程化完成、但对长期可运营闭环最关键的问题：

1. `按净利润分佣` 应如何建模，而不是仅按 `net_collected_amount` 固定比例抽成
2. `policy 覆盖 / 继承 / 版本切换 / 冲突治理` 应如何设计，避免后续规则越来越多后失控

本文档定位：

- 面向 `v-platform-backend` 的下一阶段详细设计
- 作为后续模型扩展、接口设计、规则引擎、审计落地的统一基线

本文档不直接定义：

- Menu 前端页面交互
- 渠道合同模板文本
- 会计报表口径和法务税务细则

## 2. 当前事实与设计边界

## 2.1 当前代码事实

截至当前代码现实，平台侧已经支持：

- `channel_partner / channel_program / channel_partner_binding / channel_commission_policy`
- `channel_commission_ledger / channel_clawback_ledger`
- `charge -> commission ledger` 的固定比例分佣
- `refund -> reversed / clawback` 的基础闭环
- 结算批次生成、确认、处理、关闭、取消

当前代码仍然局限于：

- `rate_type = fixed_rate`
- `commission_base = net_collected_amount | paid_amount`
- policy 的命中主要靠：
  - `channel_program_id`
  - `product_code`
  - `applies_to`
  - `effective_from / effective_to`
  - `priority`
- 尚未落地“净利润分佣”
- 尚未落地“policy version + assignment 覆盖树”

因此本文档的定位非常明确：

- **当前事实**：平台已有渠道分佣闭环，但还是“按收入比例分佣”的第一阶段实现
- **目标状态**：扩展为“按净利润分佣 + 多层 policy 覆盖 + 版本化切换 + 决策可审计”

## 2.2 边界原则

延续全局与平台既有边界：

- 产品侧定义“哪些业务事件可分佣”
- 平台侧定义“如何归因、如何算账、如何结算、如何冲正”
- 平台不直接理解 Menu 的业务表结构，但可以消费产品提供的标准化事件与利润快照

结论：

- `Menu` 仍然拥有产品业务语义
- `Platform` 仍然是分佣计算、账本、结算和审计的真相源

## 3. 核心结论

## 3.1 不再把“1/3”硬编码理解为收入抽成

业务上经常口头说：

- 成本 1/3
- 平台利润 1/3
- 渠道利润 1/3

但系统里不能硬编码成“三等分”。

原因是后续一定会遇到：

- 不同支付通道手续费不同
- 不同地区税费不同
- 不同 billable item 的履约成本不同
- 部分成本是在事后确认，而不是收费时就能 100% 确定
- 不同 partner / plan / SKU / 合同下，分佣比例和成本口径可能不同

因此正式系统应抽象为：

- `先确定可分配利润`
- `再按 policy 对利润分佣`

## 3.2 平台内部建议使用“可分配利润”语义

不建议直接以 `net_profit_amount` 作为唯一正式字段名。

推荐使用：

- `distributable_profit_amount`
- 或 `settlement_profit_amount`

原因：

- “财务净利润”是更宽的会计概念
- “经营净利润”与“渠道结算利润”也可能不同
- 平台真正关心的是：“本次分佣事件可用于渠道分配的利润口径”

本文后续统一使用：

- `distributable_profit_amount`

## 4. 净利润分佣模型

## 4.1 保留收入事实字段

平台继续保留并标准化以下收入侧字段：

- `gross_amount`
- `discount_amount`
- `paid_amount`
- `refunded_amount`
- `net_collected_amount`

其中：

- `net_collected_amount = paid_amount - refunded_amount`

这些字段仍然是账本、退款、对账、结算的基础事实，不可被利润口径替代。

## 4.2 标准成本组件

为了支持“按净利润分佣”，平台必须把成本拆成标准白名单组件，而不是允许任意字符串表达式。

阶段 2 建议支持以下成本组件：

- `payment_fee_amount`
  - 支付通道手续费
- `tax_amount`
  - 税费或需要从结算利润剔除的税项
- `service_delivery_cost_amount`
  - 履约或服务直接成本
- `infra_variable_cost_amount`
  - 模型调用、云资源、第三方 API 的直接可变成本
- `risk_reserve_amount`
  - 风险准备金
- `manual_adjustment_amount`
  - 人工差异调整，必须带 reason code

阶段 2 不建议支持：

- 总部管理费
- 固定人力摊销
- 任意自由输入的“利润修正”

这些成本过于主观，容易让分佣争议不可审计。

## 4.3 推荐利润公式

建议统一公式如下：

```text
net_collected_amount = paid_amount - refunded_amount

recognized_cost_amount =
  payment_fee_amount
+ tax_amount
+ service_delivery_cost_amount
+ infra_variable_cost_amount
+ risk_reserve_amount
+ manual_adjustment_amount

distributable_profit_amount =
  max(0, net_collected_amount - recognized_cost_amount)

channel_commission_amount =
  round(distributable_profit_amount * commission_rate, scale, rounding_mode)

platform_retained_profit_amount =
  distributable_profit_amount - channel_commission_amount
```

规则：

- 若 `distributable_profit_amount <= 0`，默认本次不产生正佣金
- 若合同允许负利润追缴，不直接把当前佣金记成负值，而是后续通过 `clawback` 或负结算余额处理

## 4.4 利润快照是正式输入真相

平台不应在入账时临时跨表拼接利润口径。

必须引入：

- `channel_profit_snapshot`

它是某笔消费在“佣金确认锚点时刻”的标准化利润输入快照。

建议字段：

- `id`
- `product_code`
- `org_id`
- `source_charge_id`
- `source_order_id`
- `billable_item_code`
- `currency`
- `gross_amount`
- `discount_amount`
- `paid_amount`
- `refunded_amount`
- `net_collected_amount`
- `payment_fee_amount`
- `tax_amount`
- `service_delivery_cost_amount`
- `infra_variable_cost_amount`
- `risk_reserve_amount`
- `manual_adjustment_amount`
- `recognized_cost_amount`
- `distributable_profit_amount`
- `snapshot_basis`
- `snapshot_hash`
- `commission_recognition_at`
- `metadata`
- `created_at`

说明：

- `snapshot_basis` 用于说明这份快照是“实时估算”还是“对账后确认”
- `snapshot_hash` 用于后续回放与防篡改校验
- `commission_recognition_at` 是规则版本切换时真正的命中时间锚点

## 5. Policy 模型升级

## 5.1 为什么当前 policy 不够

当前 `channel_commission_policy` 已能表达：

- 对哪个 `applies_to` 生效
- 固定费率
- 冷静期
- 账期
- 生效时间

但它还不够表达：

- 净利润分佣的成本组件
- 不可变版本
- 覆盖树
- 合同级特例
- 规则命中可回放

因此建议把当前 policy 拆成三层：

1. `channel_commission_policy`
2. `channel_commission_policy_version`
3. `channel_commission_policy_assignment`

## 5.2 `channel_commission_policy`

逻辑政策容器。

建议字段：

- `id`
- `policy_code`
- `name`
- `product_code`
- `program_type`
- `owner_scope`
- `status`
- `metadata`
- `created_at`
- `updated_at`

职责：

- 回答“这是哪类政策”
- 不直接保存可变费率与利润口径

## 5.3 `channel_commission_policy_version`

不可变版本体。

建议字段：

- `id`
- `policy_id`
- `version_code`
- `status`
- `applies_to`
- `trigger_type`
- `profit_basis_config`
- `commission_rule_config`
- `cooldown_days`
- `settlement_cycle`
- `effective_from`
- `effective_to`
- `rounding_mode`
- `rounding_scale`
- `metadata`
- `created_at`

说明：

- `profit_basis_config` 保存利润口径与成本组件白名单
- `commission_rule_config` 保存费率规则
- 版本一旦进入 `active`，不得原地修改关键计算字段

## 5.4 `channel_commission_policy_assignment`

政策分配与覆盖层。

建议字段：

- `id`
- `policy_version_id`
- `assignment_level`
- `partner_id`
- `org_id`
- `binding_id`
- `product_code`
- `billable_item_code`
- `sku_code`
- `plan_code`
- `region_code`
- `currency`
- `partner_tier`
- `priority`
- `status`
- `effective_from`
- `effective_to`
- `metadata`
- `created_at`
- `updated_at`

职责：

- 决定“哪条 version 对哪一类事件生效”
- 决定“覆盖谁、何时覆盖”

## 6. Policy 覆盖与继承规则

## 6.1 覆盖层级

建议命中顺序从高到低如下：

1. `event_override`
2. `contract_override`
3. `binding_override`
4. `partner_program_assignment`
5. `product_default_assignment`
6. `platform_default_deny`

### 6.1.1 `event_override`

说明：

- 仅限可信内部后端在特殊运营或补录场景下显式指定
- 不能开放给前端
- 必须强审计

### 6.1.2 `contract_override`

说明：

- 范围一般是 `partner + org + product`
- 用于合同明确约定的特例

### 6.1.3 `binding_override`

说明：

- 某条归因绑定本身绑定了特殊 policy version
- 常见于临时迁移、活动期试点

### 6.1.4 `partner_program_assignment`

说明：

- 某 partner 在某 channel program 下的默认规则
- 是最常见的 partner 默认配置层

### 6.1.5 `product_default_assignment`

说明：

- 产品级默认政策
- 没有特例时的兜底正向规则

### 6.1.6 `platform_default_deny`

说明：

- 若所有上层都没命中，不要猜
- 应明确返回 `ignored_no_policy`

## 6.2 同层级内部匹配优先级

同一 assignment level 内，按“范围越窄越优先”：

1. `billable_item_code` 精确匹配
2. `sku_code / plan_code` 精确匹配
3. `region_code + currency` 双精确匹配
4. `partner_tier` 精确匹配
5. wildcard/default

`priority` 的使用原则：

- 只在同等 specificity 下做排序
- 不用于掩盖配置冲突

## 6.3 冲突治理

必须引入硬校验：

- 同一 scope、同一时间窗，不允许两条 `active assignment` 重叠
- 同一 version 内，不允许两条规则在相同 specificity 下对同一事件都可能命中
- 若运行时仍出现多命中，系统应拒绝入账并记录 `resolution_conflict`

禁止使用以下隐式逻辑兜底：

- 最新创建优先
- ID 最大优先
- 数据库返回第一条优先

这些做法不可审计，也无法对外解释。

## 7. 版本切换规则

## 7.1 版本状态建议

建议 `channel_commission_policy_version` 使用以下状态：

- `draft`
- `review_pending`
- `approved`
- `scheduled`
- `active`
- `sunset`
- `retired`
- `canceled`

## 7.2 不可变原则

关键原则：

- `active` 版本不可编辑
- 调整费率、利润口径、成本组件、冷静期，都必须新建 version
- 历史 ledger 永远绑定创建时命中的 `policy_version_id`

## 7.3 切换锚点

建议 policy version 的命中时间锚点统一为：

- `commission_recognition_at`

而不是：

- 结算时间
- 批次生成时间
- 页面查看时间

建议：

- 一次性 charge 取 `billable_charge_recorded` 或 `billable_charge_released` 的确认时点
- 账单型 charge 取 `billing_statement_finalized` 时点

## 7.4 切换效果

版本切换只影响：

- 切换后新的佣金确认事件

不影响：

- 已生成的历史 ledger
- 已进入结算批次的历史账务

如需追溯重算：

- 必须通过 `channel_commission_adjustment_ledger`
- 禁止直接覆盖历史 `commission_ledger`

## 8. 账本与补差设计

## 8.1 主账本扩展

在现有 `channel_commission_ledger` 基础上，建议新增：

- `policy_version_id`
- `profit_snapshot_id`
- `assignment_level`
- `matched_rule_code`
- `calculation_formula_code`
- `rounding_mode`
- `calculation_trace_id`

这样每条账本都能回答：

- 命中了哪版 policy
- 用的哪份利润快照
- 走的是哪层覆盖逻辑
- 具体怎么算出来的

## 8.2 调整账本

建议新增：

- `channel_commission_adjustment_ledger`

场景：

- 版本切换后需追溯修正
- 人工争议处理
- 成本确认延迟导致的补差
- 财务发现错误口径后的附加修正

原则：

- 不改原始账本
- 只追加 adjustment 分录
- adjustment 也必须能进入后续结算批次

## 9. 决策审计与权限要求

## 9.1 决策审计

每次命中 policy 时，建议记录：

- `event_id`
- `request_id`
- `trace_id`
- 候选 assignment 列表
- 每条候选被淘汰原因
- 最终命中的 `policy_version_id`
- `assignment_level`
- `matched_rule_code`
- 利润输入快照摘要
- 最终计算明细

建议形成：

- `channel_policy_resolution_audit`

## 9.2 配置审计

以下动作必须带审计：

- 创建 policy
- 创建 version
- 提交审批
- 审批通过
- 发布 version
- 失效 version
- 创建或删除 assignment
- 手工 override

要求保留：

- before snapshot
- after snapshot
- operator
- reviewer
- reason code
- ticket / contract ref

## 9.3 权限要求

以下操作必须是高权限，并建议双人审批：

- `event_override`
- 合同级特例 policy 发布
- 手工改绑
- 手工调账
- 重算批次
- 冻结 / 解冻渠道 partner

## 10. 推荐接口与校验

## 10.1 推荐新增内部接口

建议后续新增：

- `POST /internal/v1/incentives/channel-policy-versions`
- `GET /internal/v1/incentives/channel-policy-versions`
- `POST /internal/v1/incentives/channel-policy-assignments`
- `GET /internal/v1/incentives/channel-policy-assignments`
- `POST /internal/v1/incentives/channel-profit-snapshots`
- `GET /internal/v1/incentives/channel-policy-resolution-preview`

## 10.2 发布前校验

发布 policy version 或 assignment 前，必须执行：

- 时间窗冲突校验
- scope 重叠校验
- specificity 冲突校验
- 历史回放 dry-run
- 影响 partner 清单预览
- 金额波动阈值校验

若存在以下情况，拒绝发布：

- 多命中冲突无法解释
- 大量事件会从“有佣金”变成“无佣金”
- 大量事件佣金差异超阈值但未审批

## 11. 分阶段实施建议

## 11.1 Phase 2A：模型补全

先补：

- `channel_profit_snapshot`
- `channel_commission_policy_version`
- `channel_commission_policy_assignment`
- `resolution audit`

此阶段仍可只支持：

- `fixed_rate`
- 白名单成本组件

## 11.2 Phase 2B：计算引擎切换

实现：

- 先产利润快照
- 再走 assignment resolution
- 再按 version 计算佣金

并保留与现有 `net_collected_amount` 模式兼容的迁移路径。

## 11.3 Phase 2C：调整与回放

实现：

- adjustment ledger
- dry-run 回放
- 版本切换差异预览

## 12. 最终结论

下一阶段渠道分佣的正式方向应是：

1. 从“按收入比例分佣”演进到“按可分配利润分佣”
2. 从“单条 active policy + priority”演进到“policy version + assignment 覆盖树”
3. 从“简单命中”演进到“命中可解释、切换可审计、历史不可篡改”
4. 通过 `profit_snapshot + policy_version + assignment + adjustment_ledger + resolution_audit` 收紧整条分佣链路

只有把这五层补齐，后续才真正具备：

- 合同特例
- 比例变更
- 成本口径演进
- 大规模 partner 管理
- 财务与运营对账

同时仍能保持平台侧“账本真相源”的工程稳定性。
