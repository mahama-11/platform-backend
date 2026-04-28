# 商业主体与商户路由设计

## 1. 目标

本文档定义平台侧多商业主体、多商户账号、结算账户与计费路由模型，解决这些问题：

- 平台如何同时支撑多个收款主体
- 一笔订单如何决定最终走哪个商户账号
- 产品是否能指定“走哪个主体账号”
- 计费、支付、退款、对账如何共享同一条路由真相

## 2. 设计原则

- 商业主体、支付商户、结算账户必须拆开建模
- 产品可以给出路由提示，但最终决策权在平台
- 前端不能直接操作底层支付商户号
- 路由决策必须可审计、可回放、可解释
- 同一笔交易在支付、退款、对账链路上必须复用同一条路由真相

## 3. 核心对象

### 3.1 `commercial_entity`

表示商业主体、法务主体、签约主体。

示例：

- 北京主体
- 香港主体
- 海外主体

建议字段：

- `id`
- `code`
- `name`
- `entity_type`
- `country_code`
- `currency`
- `tax_profile`
- `status`
- `metadata`
- `created_at`
- `updated_at`

说明：

- 它回答的是“这笔收入法律上归谁”
- 不等于支付通道账号

### 3.2 `merchant_account`

表示支付渠道上的商户账号。

示例：

- Stripe Account A
- WeChat Mch B
- Alipay PID C

建议字段：

- `id`
- `commercial_entity_id`
- `channel`
- `account_code`
- `account_name`
- `country_code`
- `currency`
- `capabilities`
- `status`
- `metadata`
- `created_at`
- `updated_at`

说明：

- 它回答的是“这笔支付从哪个通道账号发起”
- 一个主体可以有多个 merchant account

### 3.3 `settlement_account`

表示最终的结算账户或收款归集账户。

建议字段：

- `id`
- `commercial_entity_id`
- `account_type`
- `currency`
- `bank_region`
- `masked_account_no`
- `status`
- `metadata`
- `created_at`
- `updated_at`

说明：

- 它回答的是“这笔钱最终结到哪里”
- 结算账户与支付商户账号不一定是一一对应

### 3.4 `billing_profile`

表示一组面向产品、组织、地区、币种、支付策略的计费画像配置。

这是产品接入层和平台计费层之间最重要的桥。

建议字段：

- `id`
- `code`
- `product_id`
- `commercial_entity_id`
- `default_merchant_account_id`
- `default_settlement_account_id`
- `region_scope`
- `currency`
- `pricing_strategy`
- `tax_strategy`
- `status`
- `metadata`
- `created_at`
- `updated_at`

说明：

- 一个产品可以有多个 billing profile
- 一个组织通常会绑定一个默认 billing profile
- rate card、route policy、invoice 口径都可以挂在这个对象上

### 3.5 `routing_policy`

表示商户路由规则。

建议字段：

- `id`
- `billing_profile_id`
- `priority`
- `match_type`
- `match_config`
- `target_merchant_account_id`
- `target_settlement_account_id`
- `status`
- `metadata`
- `created_at`
- `updated_at`

建议支持的匹配维度：

- `channel`
- `currency`
- `region`
- `country`
- `product`
- `org`
- `payment_scene`
- `order_type`

说明：

- 它回答的是“在当前条件下，这笔订单应该走哪个 merchant”
- 支持同一个 billing profile 下多条规则按优先级匹配

## 4. 关系模型

推荐关系：

1. 一个 `commercial_entity` 可以绑定多个 `merchant_account`
2. 一个 `commercial_entity` 可以绑定多个 `settlement_account`
3. 一个 `product` 可以有多个 `billing_profile`
4. 一个 `billing_profile` 归属于一个 `commercial_entity`
5. 一个 `billing_profile` 下可以有多条 `routing_policy`
6. 一个组织可以绑定一个默认 `billing_profile`
7. 一笔订单可以最终解析出一条确定的 merchant route snapshot

建议增加组织绑定表：

- `org_billing_profiles`

建议增加交易快照字段：

- `commercial_entity_id`
- `merchant_account_id`
- `settlement_account_id`
- `billing_profile_id`
- `routing_policy_id`
- `routing_snapshot`

## 5. 路由决策流程

### 5.1 默认决策顺序

推荐顺序如下：

1. 显式 override
2. 组织绑定的 billing profile
3. 产品默认 billing profile
4. routing policy 匹配
5. billing profile 默认 merchant
6. 平台兜底拒绝或报错

### 5.2 显式 override 的边界

支持显式指定，但不支持直接传底层商户账号。

允许产品后端提交：

- `billing_profile_key`
- `merchant_route_hint`
- `payment_scene`

不允许前端直接提交：

- `merchant_account_id`
- `settlement_account_id`
- `commercial_entity_id`

原因：

- 前端可见即意味着可伪造
- 商户路由属于平台内部结算与风控域

### 5.3 路由结果必须快照化

一旦订单生成，路由结果必须写入订单或账务快照。

至少要保存：

- 使用了哪个 `billing_profile`
- 命中了哪条 `routing_policy`
- 最终使用哪个 `merchant_account`
- 最终归哪个 `commercial_entity`
- 最终走哪个 `settlement_account`

否则后续退款、对账、追溯时会出现口径不一致。

## 6. 为什么 billing profile 是核心对象

`commercial_entity` 太偏法务主体。

`merchant_account` 太偏支付通道。

`routing_policy` 太偏执行规则。

而产品接入时真正需要的是：

- 某个产品
- 某个组织
- 某个区域
- 某个币种
- 当前应该走哪套商业配置

所以 `billing_profile` 才是最适合做平台对外接入锚点的对象。

建议产品侧只理解到：

- `billing_profile_key`

而不要理解底层商户账号。

## 7. 推荐的第一版数据表

建议第一版先建设：

- `commercial_entities`
- `merchant_accounts`
- `settlement_accounts`
- `billing_profiles`
- `routing_policies`
- `org_billing_profiles`

同时在订单或账本相关表中预留：

- `commercial_entity_id`
- `merchant_account_id`
- `settlement_account_id`
- `billing_profile_id`
- `routing_policy_id`
- `routing_snapshot`

## 8. 典型场景

### 8.1 同一产品多地区主体

场景：

- `menu_ai` 在国内和海外使用不同主体

处理方式：

- 建两个 `commercial_entity`
- 建两个 `billing_profile`
- 通过 `region` 或 `currency` 路由到不同主体

### 8.2 同一主体多支付渠道

场景：

- 同一主体支持 Stripe 和微信支付

处理方式：

- 同一 `commercial_entity` 下绑定多个 `merchant_account`
- 通过 `channel` 路由到对应账号

### 8.3 指定某个大客户走特殊商户

场景：

- 某大客户要求独立账务口径

处理方式：

- 给该 org 绑定单独的 `billing_profile`
- 通过该 profile 走独立 merchant route

### 8.4 临时 override

场景：

- 某次活动或临时切量，产品希望指定走某个主体

处理方式：

- 产品后端提交 `merchant_route_hint`
- 平台进行权限校验
- 平台写审计日志
- 平台生成 route snapshot

## 9. 与其他模块的依赖关系

### 9.1 对 billing 的影响

billing 需要知道：

- 账单归属哪个主体
- 应用哪个 billing profile
- 对应哪个 merchant route

### 9.2 对 payment 的影响

payment 需要知道：

- 支付时用哪个 merchant account
- 回调时如何找到原交易 route snapshot

### 9.3 对 refund 的影响

refund 必须复用原始 route snapshot，不能重新计算路由。

### 9.4 对 metering 的影响

usage event 里建议带：

- `billing_profile_key`
- `currency_context`
- 可选 `merchant_route_hint`

但最终解析和落账仍由平台完成。

## 10. 工程化要求

这个模块必须具备以下能力：

- 所有 route decision 带 `request_id` / `trace_id`
- route override 必须有 audit log
- route decision 结果必须可观测
- 命中率、拒绝率、fallback 率要有 metrics
- routing_policy 变更必须可审计
- route snapshot 必须可追溯

建议指标：

- `routing_decision_total`
- `routing_decision_failed_total`
- `routing_override_total`
- `routing_fallback_total`

## 11. 第一版落地建议

第一版不追求过度复杂，先做：

1. 静态配置式 `billing_profile`
2. 简单优先级式 `routing_policy`
3. 订单级 route snapshot
4. 审计日志
5. 路由指标与错误告警

先不要急着做：

- 可视化复杂规则编排器
- 非常复杂的动态脚本路由
- 过多运行时自动学习路由策略

## 12. DoD

这一模块进入可实施状态的标准：

- 多主体、多商户、多结算账户概念边界清晰
- 产品侧只需理解 `billing_profile_key` 这一层
- 平台能完成路由决策、快照、审计
- 支付、退款、账单可复用同一份 route truth
- 支持至少一个 Menu 多主体场景
