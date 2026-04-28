# 商业化边界

## 1. 目标

本文档用于定义 `v-platform-backend` 中共享商业化基础设施，与 `v-menu-backend` 这类产品后端中的业务商业策略之间的边界。

目标不是做一个“什么都塞进去的大而全商业系统”，而是明确：

- 钱、权益、台账、计量这些基础能力沉淀在平台
- 定价、售卖包装、业务语义留在产品侧
- 后续新增产品时，不需要每个产品都重做一套计费系统

## 2. 主流模式

主流商业化系统里，边界基本都比较稳定：

- 支付与 billing 系统负责收款、退款、订阅、发票、税务、按量计费
- 营销与 loyalty 系统负责活动、券码校验、叠加规则、核销、回滚、积分、返佣
- entitlement 系统负责把套餐、订阅、资源包映射成可用能力与使用上限

参考资料：

- Stripe coupons / promotion codes: https://docs.stripe.com/billing/subscriptions/coupons
- Voucherify key concepts: https://docs.voucherify.io/get-started/key-concepts
- Stigg entitlement overview: https://docs.stigg.io/documentation/managing-customers-and-subscriptions/entitlements/overview

共同规律是：

- 平台负责可复用的执行底座
- 产品负责自己的商业策略

## 3. 边界结论

### 3.1 平台负责的商业化基础能力

以下能力只要会被多个产品复用，就应该放在 `v-platform-backend`：

- 支付通道适配层，以及商户账号接入
- 订单、支付、扣款、结算、退款、对账等基础状态流转
- 订阅生命周期与 billing 周期引擎
- 钱包、余额、积分、优惠券、奖励、台账等基础设施
- 营销核销基础设施，以及防重放、幂等等控制
- 推广转化记录与返佣结算基础能力
- 计量事件采集、标准化、聚合、计费账本生成
- entitlement 计算，以及套餐到能力的映射
- 发票生成、账单快照、计费凭证留存

平台负责的是：

- 可审计的持久化账本
- 资金相关安全
- 幂等与事件一致性
- 跨产品复用的一致性

### 3.2 产品负责的商业逻辑

以下内容应继续放在产品后端：

- 产品具体卖什么
- 某个功能怎么包装成套餐
- 哪些动作需要收费
- 某个业务动作应该消耗多少资源
- 活动文案、页面路径、销售话术、转化策略
- 产品自己的 KPI 口径与报表语义
- 某个功能是试用、会员、增值包还是 bundle

结论很直接：

- 产品负责“商业策略”
- 平台负责“商业执行底座”

### 3.3 硬规则

不要把产品特有语义直接塞进平台表结构或平台路由，比如：

- menu publish package
- attendance late penalty
- kyc manual review upsell

平台模型应尽量保持通用：

- `product`
- `sku`
- `package`
- `entitlement`
- `meter`
- `billable_item`
- `merchant_account`
- `settlement_account`

## 4. 多商业主体 / 多商户模型

### 4.1 结论

是，平台应该支持多个商业主体，并且应该支持“指定走哪个主体账号”。

这不是后面再说的可选能力，而是平台商业化从一开始就应预留的能力。只要出现以下任一情况，单主体模型很快就会不够用：

- 多产品线
- 多地区
- 不同签约主体
- 不同支付渠道
- 不同结算路径

### 4.2 为什么必须由平台负责

多主体路由通常依赖这些共享且敏感的能力：

- 法务主体和签约主体
- 支付渠道账号绑定
- 地区、币种、税务规则
- 退款路径和对账路径
- 结算账户归属
- 风控与审计

如果每个产品自己做一套商户路由，最终会出现：

- 支付逻辑重复实现
- 退款行为不一致
- 对账口径混乱
- 审计难度很高

### 4.3 建议的平台模型

平台建议拆分以下概念：

- `commercial_entity`：商业主体 / 法务主体 / 签约主体
- `merchant_account`：支付通道上的商户账号，例如 Stripe Account、微信商户号、支付宝 PID
- `settlement_account`：最终结算账户
- `billing_profile`：绑定产品、组织、地区、币种、商业规则的一组计费配置
- `routing_policy`：决定某一笔订单最终走哪个商户账号的规则

推荐关系：

- 一个产品可以绑定多个 `billing_profile`
- 一个组织可以绑定一个默认 `billing_profile`
- 一个 `billing_profile` 可以按照渠道、地区、币种、场景路由到不同 `merchant_account`
- 一笔订单可以在受控前提下带一个覆盖路由，用于特殊场景

### 4.4 是否支持明确指定走哪个主体账号

支持，但必须受控，不能裸传商户号。

建议规则：

- 产品后端可以提交 `billing_profile_key` 或 `merchant_route_hint`
- 平台负责解析出最终的 `merchant_account_id`
- 前端不应直接传真实商户账号 id
- 任何显式 override 都必须有权限控制和审计记录

建议优先级：

1. 来自可信后端的显式路由 override
2. 组织绑定的 billing profile
3. 产品默认 routing policy
4. 平台兜底策略

这样既能支持灵活切换主体，又不会把支付账号细节暴露给前端。

## 5. 计量、计费、用量边界

### 5.1 结论

计量和计费基础能力应该归平台，不应该每个产品自己重建一套。

但产品侧仍然负责定义“什么动作应该收费”。

换句话说：

- 产品负责定义 billable 业务行为
- 平台负责统一承接 metering、billing、ledger、settlement

### 5.2 平台负责什么

平台应该统一负责：

- metering event 的标准契约
- 事件接入 API / 队列 / 幂等控制
- usage 的 SSOT 与不可篡改的计费凭证
- 从产品动作到通用 billable meter 的标准化
- 按组织、产品、sku、商户、周期、维度做聚合
- rate card 套用与账单行生成
- quota / credits / wallet / subscription 的消费台账
- usage statement API 和财务对账导出

### 5.3 产品负责什么

产品应继续负责：

- 哪些动作发出 metering event
- 动作如何映射到 sku
- usage unit 在业务语义里的含义
- 该动作是免费、收费、打包还是赠送
- 产品定价策略和售卖包装
- 产品自己的经营分析看板

### 5.4 建议的事件契约

产品后端调用平台计量能力时，建议至少上报这些字段：

- `product_code`
- `org_id`
- `user_id`
- `billable_item_code`
- `usage_units`
- `currency_context`
- `billing_profile_key`
- `dimensions`
- `request_id`
- `occurred_at`
- `billable`

职责划分：

- 产品发出事件
- 平台落库、聚合、结算

### 5.5 一个实用判断方法

如果问题是：

- “这个 org 一共调用了多少次平台能力 X？”

那这属于平台计量。

如果问题是：

- “Menu AI 这个套餐怎么命名、怎么卖、怎么展示？”

那这属于产品商业策略。

## 6. Menu 的接入建议

对于 Menu AI，建议边界如下：

- 平台负责支付、订阅、credits / wallet 台账、优惠券 / 推广返佣基础能力、商户路由、计量管道、entitlement 基础校验
- `v-menu-backend` 负责 Menu 的套餐定义、模板高级能力规则、AI job 扣费规则、Menu 自己的经营报表口径
- `v-menu-frontend` 可以继续直连平台认证接口，但商业结算真相仍应落在平台

当前平台已开始把“推广返佣基础能力”从单纯 `commission_ledger` 扩展为三层底座：

- `referral_program`：按 `product_code`、`trigger_type`、`commission_policy` 配置规则
- `referral_code`：承载推广码与推广主体
- `referral_conversion`：承载转化归因与结算状态，并与 `commission_ledger` 关联

产品侧不应自己计算最终佣金金额；产品应提交标准化事件或调用内部推广接口，由平台完成规则命中、归因和返佣落账。

建议链路：

1. `v-menu-backend` 判断某个动作是否收费。
2. `v-menu-backend` 向平台提交标准化的计量事件或订单请求。
3. 平台完成 entitlement、商户路由、quota / credits / billing 处理。
4. 平台落 SSOT 明细和账本。
5. Menu 读取平台提供的账单摘要接口，再组合成产品侧报表。

## 7. KYC 当前设计参考

`v-backend` 现在已经有一版可参考的 quota / usage 设计，核心点包括：

- `checkAndConsumeQuota`：实时配额校验与扣减
- `organization_quotas`：按组织、按服务类型记录分配额度与已消耗额度
- `usage_logs`：被定义为 billing / usage 的 SSOT
- `usage_metric_aggs`：承接聚合统计
- `plans.quota_config`：存储套餐内 quota 定义

这套设计对当前 KYC 业务是有价值的，但它仍然是“单产品内计费实现”，不能原样当作长期平台商业化架构。

### 7.1 优点

- 约束性强：所有消耗型操作都要求走 `checkAndConsumeQuota`
- Redis + DB fallback，适合高频扣量场景
- `organization_quotas` 让服务级配额边界比较清楚
- `usage_logs` 作为 SSOT，这个方向是对的
- `Billable`、`UsageUnits`、`SessionID`、`Metadata` 这些字段已经有事件化设计思路
- Redis Stream 异步日志落库，降低了主链路压力

### 7.2 缺点

- quota 和 billing 仍然耦合在单一产品服务内部，没有平台化
- 主要维度仍偏向 `org + service_type`，不够支撑多产品、多主体、多商户路由
- `plans.quota_config` 适合 MVP，但不够支撑复杂商业建模
- 当前 `GetBilling` 更像 usage summary，不是完整 billing engine
- 服务类型与路由推断耦合在单仓库代码里，不利于多产品演进
- Redis 成功、DB 异步补写的模式，适合 quota 热路径，但不适合作为最终财务账本
- 这套设计更偏“配额闸门”，还不是“统一收入账务底座”

### 7.3 哪些思路值得继承

这些思路建议保留到平台商业化体系里：

- 所有成本型操作必须经过统一守卫
- usage event 必须显式带 `billable` 语义
- 明细日志 + 聚合表的分层是对的
- request 级别的幂等证据必须保留
- quota 扣减和 usage 统计应该关联，但不能混成一张表

### 7.4 平台化时需要改什么

平台商业化模型应进一步拆开：

- 分离 `meter_event`、`quota_ledger`、`billing_ledger`、`invoice`
- 增加一等维度：`product_code`、`billable_item_code`、`commercial_entity_id`、`merchant_account_id`、`billing_profile_id`
- 改成由产品服务发事件给平台，而不是每个产品自己维护 billing 表
- 保留 SSOT 明细日志，但不能再把产品本地 quota 表当成最终商业真相

## 8. 推荐实施顺序

这里拆成两个层次：

- 哪些模块现在就可以进入详细设计
- 哪些模块要等上游模型稳定后再正式实施

### 8.1 第一阶段：先统一商业语言

先做这些基础模型设计：

1. `product`
2. `sku`
3. `package`
4. `billable_item`
5. `rate_card`

原因：

- 这是 entitlement、metering、billing 的共同语言
- 如果这层没先统一，后面每个模块都会出现不同命名和不同口径

这一阶段可以直接开始详细设计。

### 8.2 第二阶段：多主体与计费路由模型

接着做这些模型：

1. `commercial_entity`
2. `merchant_account`
3. `settlement_account`
4. `billing_profile`
5. `routing_policy`

原因：

- 这层决定“谁收钱、谁结算、走哪个商户号”
- 后面 billing、payment、invoice 都会依赖这层维度

这一阶段也可以直接开始详细设计，不需要等支付代码先写完。

### 8.3 第三阶段：metering 事件体系

优先做：

1. metering event 契约
2. ingestion 接口或队列
3. 幂等规则
4. usage 明细 SSOT
5. usage 聚合表

原因：

- quota、credits、billing statement、invoice 都要依赖 usage 真相源
- 这是最核心、最先卡住后续建设的一层

对于当前 Menu 场景，这一层应视为最高优先级实施项。

### 8.4 第四阶段：entitlement / quota / credits

然后做：

1. package 到 capability 的映射
2. entitlement 判断
3. quota ledger
4. credits ledger
5. reserve / commit / release 语义

原因：

- 这是“能不能用、剩多少、是否超额”的统一控制层
- 它依赖前面的商品模型和计量口径，但不强依赖支付先上线

### 8.5 第五阶段：billing ledger / statement / invoice

再做：

1. billing ledger
2. statement snapshot
3. invoice line
4. invoice snapshot
5. finance export

原因：

- 这层依赖 metering、rate card、billing profile
- 没有前面的统一维度，这层很容易返工

### 8.6 第六阶段：payment / refund / reconciliation

这一阶段做：

1. order
2. payment
3. refund
4. reconciliation
5. settlement confirmation

这层如果当前只是内部试运行、手工开通，可以稍后；
如果开始正式收费，就应尽快补齐。

### 8.7 第七阶段：营销与余额类能力

最后再做：

1. coupon
2. referral
3. reward
4. wallet

原因：

- 这些能力更依赖前面的账本、计量、路由、账单基础
- 没有前面几层，先做出来也会比较虚

### 8.8 当前建议的总体顺序

按优先级汇总如下：

1. `product / sku / package / billable_item / rate_card`
2. `commercial_entity / merchant_account / settlement_account / billing_profile / routing_policy`
3. `metering event / usage SSOT / aggregation`
4. `entitlement / quota / credits`
5. `billing ledger / statement / invoice`
6. `payment / refund / reconciliation`
7. `coupon / referral / reward / wallet`

## 9. 工程化基线要求

商业化基础能力不是只把表和接口做出来就算完成，必须从第一天就具备工程化基线。

### 9.1 可观测性

参考 `v-backend` 当前的 OTel / 监控思路，平台商业化模块至少要具备：

- OpenTelemetry Trace 贯穿入口请求、内部调用、异步任务、事件消费
- 每个请求和事件都具备统一 `request_id` / `trace_id`
- Prometheus 指标暴露
- Grafana 看板可视化
- 关键错误路径进入告警系统

必须覆盖的关键链路：

- entitlement 判断
- quota / credits 扣减
- metering event 入站
- usage 聚合
- billing ledger 生成
- payment / refund 回调
- merchant routing 决策

### 9.2 日志规范

日志必须使用结构化日志，并遵守这些规则：

- 日志中必须带 `request_id`
- 关键异步链路必须带 `trace_id`
- 商业事件必须带 `org_id`、`product_code`、`billable_item_code`
- 支付与账单事件必须带 `billing_profile_id`、`merchant_account_id` 或可追溯路由信息
- 不得在日志中记录敏感原文，如卡号、身份证原文、支付密钥、完整请求体

### 9.3 审计与账务证据

以下场景必须保留审计或可追溯证据：

- entitlement 变更
- package 变更
- rate card 变更
- billing profile 绑定或切换
- 商户路由 override
- quota / credits 人工调整
- refund / settlement 人工处理

要求：

- 审计日志与业务日志分层
- 账务证据与普通运行日志分层
- 关键操作支持“谁改的、何时改的、改前是什么、改后是什么”

### 9.4 幂等与一致性

商业化链路必须默认按“会重试、会重复投递、会网络抖动”来设计：

- metering event 必须支持幂等键
- payment / refund callback 必须支持幂等处理
- 账本写入必须可重放但不重复入账
- 异步任务必须区分“已接收”“处理中”“已完成”“失败待补偿”

### 9.5 可运维性

平台商业化模块要从第一版开始具备：

- 健康检查
- 指标检查
- 死信 / 失败重试监控
- 基本告警
- 对账或补偿任务的运行状态可见

### 9.6 对 `v-platform-backend` 的硬要求

后续在 `v-platform-backend` 落商业化模块时，以下能力不是加分项，而是必选项：

- Trace
- Metrics
- Structured Logging
- Audit Log
- Idempotency
- Request / Event Correlation
- Healthcheck
- Retry / Compensation Visibility

## 10. 最终结论

针对你前面两个具体问题，结论是：

- 平台应该支持多个商业主体，并支持受控地指定走哪个主体账号
- 平台应该统一承接 metering、billing、ledger、settlement 这些共享基础设施
- 产品侧负责定义 billable 业务语义、套餐内容、定价策略

不要让每个产品都各自重做一套计费系统。
