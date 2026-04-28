# 商业化全面视角与分期路线

## 1. 目标

本文档用于把平台商业化建设从“单点能力讨论”提升为“完整工程体系设计”。

目标有三个：

- 把商业化底座需要考虑的视角一次性铺全
- 明确哪些能力属于 `P0` 必须先做，哪些进入 `P1`、`P2`
- 在不一次性做完所有能力的前提下，提前预留工程化扩展空间，避免后续返工

本文档是总览文档，细节设计分别参考已有专题文档：

- [商业化边界](file:///Users/bytedance/Documents/project/go/v/v-platform-backend/docs/architecture/COMMERCIALIZATION_BOUNDARY.md)
- [商品模型设计](file:///Users/bytedance/Documents/project/go/v/v-platform-backend/docs/architecture/COMMERCIAL_CATALOG_MODEL.md)
- [商业主体与商户路由设计](file:///Users/bytedance/Documents/project/go/v/v-platform-backend/docs/architecture/COMMERCIAL_ROUTING_MODEL.md)
- [计量事件与用量真相源设计](file:///Users/bytedance/Documents/project/go/v/v-platform-backend/docs/architecture/METERING_AND_USAGE_SSOT.md)

## 2. 总体判断

平台商业化不是只有“支付”或“积分”。

一个可长期演进的平台级商业化底座，至少要同时覆盖这几类问题：

- 卖什么
- 卖给谁
- 由谁付费
- 用了多少
- 怎么收费
- 钱走哪个主体
- 怎么留痕
- 怎么观测
- 怎么对账
- 出错怎么补偿

如果只做其中一两个点，短期能跑，但中期必返工。

## 3. 全面视角清单

### 3.1 商品与售卖视角

要解决：

- 产品卖什么
- 套餐、资源包、订阅、加购包怎么表达
- 免费、付费、超额收费怎么表达

对应模型：

- `product`
- `sku`
- `package`
- `billable_item`
- `rate_card`

### 3.2 组织归属与结算主体视角

要解决：

- 这次消费是谁发起的
- 归属哪个组织
- 最终由谁承担账单
- 后续是否有总部、门店、代理商、多 workspace 场景

当前建议：

- 行为归属保留 `org_id`
- 结算归属应预留 `billing_subject_type` 和 `billing_subject_id`

建议支持的主体类型：

- `organization`
- `workspace`
- `store`
- `agency_account`

### 3.3 商业主体与商户路由视角

要解决：

- 钱归哪个法务主体
- 支付走哪个通道账号
- 最终结算到哪里
- 能不能按组织、区域、币种、渠道切路由

对应模型：

- `commercial_entity`
- `merchant_account`
- `settlement_account`
- `billing_profile`
- `routing_policy`

### 3.4 entitlement / 配额 / credits 视角

要解决：

- 当前能不能用
- 套餐包含哪些能力
- 额度还剩多少
- 超额后是拦截、转按量还是走 credits

建议能力：

- entitlement 判断
- quota ledger
- credits ledger
- reserve / commit / release

### 3.5 计量与用量真相源视角

要解决：

- 到底用了多少
- 是否按 API 计费
- 是否多个 API 打包计费
- usage 报表和 billing 是否基于同一份事实

核心模型：

- `meter_event`
- `usage_record`
- `usage_agg`

关键语义：

- `event_id`
- `request_id`
- `trace_id`
- `charge_group_id`
- `event_role`

### 3.6 账务视角

要解决：

- 账怎么记
- 如何支持冲正、退款、补偿
- usage、quota、billing 各自边界是什么

建议后续分层：

- `quota_ledger`
- wallet-backed credits assets (`wallet_account` / `wallet_bucket` / `wallet_ledger`)
- `billing_ledger`
- `statement_snapshot`
- `invoice_snapshot`

### 3.7 对账与财务闭环视角

要解决：

- 平台账和支付通道账是否一致
- 路由切换后历史账能不能追溯
- 哪些单子失败了、重放了、补偿了

建议能力：

- 对账任务
- 差异单记录
- 回放与补偿任务
- 账务快照

### 3.8 可观测性视角

要解决：

- 出问题时能不能快速定位
- 一次计费链路能不能串起来
- 异步消费卡住时能不能发现

必须具备：

- Trace
- Metrics
- Structured Logging
- Request / Event Correlation
- Alerting

### 3.9 审计与操作安全视角

要解决：

- 谁改了价格
- 谁改了商户路由
- 谁人工补了积分
- 谁触发了退款或冲正

必须具备：

- 审计日志
- 变更前后快照
- 高风险操作权限
- 追责能力

### 3.10 风控与反作弊视角

要解决：

- 刷接口
- 刷积分
- 推荐奖励作弊
- 重复回调
- 异常退款

这一层不是锦上添花，而是商业化上线后必然会碰到的能力。

### 3.11 成本核算视角

要解决：

- 底层 AI 调用成本是多少
- 存储、导出、第三方服务成本是多少
- 哪些功能赚钱，哪些功能倒贴

建议预留：

- `cost_event`
- `cost_source`
- `gross_margin_view`

### 3.12 税务、发票与地区合规视角

要解决：

- 多主体、多币种下税怎么算
- 发票由哪个主体开
- 含税/未税怎么表达

P0 不一定全部实现，但模型层必须预留：

- 税务配置
- 发票抬头
- 币种与汇率上下文

### 3.13 运营与客服视角

要解决：

- 客服怎么查账
- 运营怎么补偿
- 某个组织额度异常时怎么修
- 某条事件丢了怎么补录

这要求平台不只是提供 API，还要考虑后台运维与人工处理能力。

### 3.14 通知与客户感知视角

要解决：

- credits 快用完是否通知
- 支付失败是否通知
- 套餐即将到期是否通知
- 价格调整是否通知

### 3.15 数据治理与隐私视角

要解决：

- usage 明细保留多久
- 审计日志保留多久
- 哪些字段必须脱敏
- 什么数据允许回放，什么不允许

## 4. 这些视角之间的关系

建议从依赖关系理解，不要平铺：

1. 商品模型提供商业语言
2. 结算主体与路由模型决定归属与收款路径
3. entitlement / quota / credits 决定“能不能用”
4. metering / usage 决定“用了多少”
5. ledger / statement / invoice 决定“怎么记账”
6. observability / audit / risk 决定“能不能稳定运营”

也就是说：

- 商品模型是语言层
- 路由和主体是归属层
- metering 是事实层
- ledger 是账务层
- 可观测和审计是运营保障层

## 5. P0 / P1 / P2 分期原则

分期不等于后面不考虑。

正确做法是：

- `P0` 先做最小可用闭环
- `P1` 补齐核心运营与财务能力
- `P2` 扩展增长、复杂财务与全球化能力

但即便某能力放到 `P1/P2`，也要在 `P0` 的模型和接口里预留扩展点。

## 6. P0 必须交付什么

### 6.1 P0 目标

让平台先具备“可卖、可限、可计、可查、可追”的最小商业化闭环。

### 6.2 P0 范围

P0 必须包含：

1. 商品基础模型
2. 组织归属基础模型
3. 多主体与计费路由基础模型
4. entitlement / quota / credits 基础能力
5. metering event 与 usage SSOT
6. 基础账务台账骨架
7. 可观测与审计基线

### 6.3 P0 具体能力

#### 商品与售卖

- `product`
- `sku`
- `package`
- `billable_item`
- `rate_card`

#### 主体与路由

- `commercial_entity`
- `merchant_account`
- `billing_profile`
- 简单优先级式 `routing_policy`
- route snapshot

#### entitlement / 配额 / credits

- package 到 entitlement 映射
- quota 检查
- credits 检查
- 基础扣减

#### 计量

- `meter_event`
- `usage_record`
- `usage_agg`
- 幂等键
- 异步消费

#### 账务骨架

- `quota_ledger`
- wallet-backed credits assets (`wallet_account` / `wallet_bucket` / `wallet_ledger`)
- 基础 `billing_ledger` 骨架
- `settlement_record`，用于沉淀一次事实消费最终拆成了多少 quota / credits / wallet / discount / receivable
- `discount_ledger`，用于沉淀营销折扣、优惠减免等账务证据
- reward / commission 的最小联动骨架，用于把增长激励挂到消费主链路上

#### 工程化基线

- Trace
- Metrics
- Structured Logging
- `request_id` / `trace_id`
- Audit Log
- Healthcheck
- Retry / Compensation Visibility

### 6.4 P0 要明确但可简化的点

P0 可以先做简单版本：

- 简单价格模型
- 简单路由规则
- 简单 credits 扣减
- 简单日级 usage 聚合

但不能省略概念边界。

### 6.5 P0 不建议遗漏的预留点

P0 即便不完整实现，也要预留：

- `billing_subject_type`
- `billing_subject_id`
- `effective_from`
- `effective_to`
- `version`
- `charge_group_id`
- `event_role`
- `route_snapshot`

## 7. P1 重点补什么

### 7.1 P1 目标

把 P0 的“能跑”升级成“可运营、可结算、可对账”。

### 7.2 P1 范围

P1 重点补齐：

- ledger 完整语义
- statement / invoice
- reconciliation
- 风险控制基础
- 客服/运营操作能力

### 7.3 P1 具体能力

- `billing_ledger` 从骨架升级成正式账务层
- `statement_snapshot`
- `invoice_snapshot`
- reserve / commit / release / reverse
- 对账任务与差异单
- 退款与冲正
- 手工补录、手工补偿、手工调整后台能力
- 审计增强：变更前后快照、审批链预留
- 告警与运营看板

### 7.4 P1 重点风险

如果 P1 不做，平台会遇到：

- 账单难解释
- 退款路径不统一
- 对账成本很高
- 运营问题只能手工查数据库

## 8. P2 扩展什么

### 8.1 P2 目标

把商业化从“结算底座”扩展到“增长、精细化财务、全球化支持”。

### 8.2 P2 范围

P2 重点扩展：

- coupon / referral / reward / wallet
- 成本核算
- 税务与发票复杂场景
- 复杂路由与全球化能力
- 更成熟的风控体系

### 8.3 P2 具体能力

- coupon stack / redemption / rollback
- referral tracking / reward settlement
- wallet / stored value
- 成本事件归集
- 毛利分析
- 多币种与汇率
- 税务配置与开票主体
- 更复杂的路由编排
- 反作弊与异常行为检测

## 9. 每阶段的工程化要求

### 9.1 P0 工程化要求

P0 必须做到：

- 全链路 `request_id`
- 基础 Trace
- Prometheus 指标
- 基础 Grafana
- 结构化日志
- 核心审计日志
- 失败重试与死信可见

### 9.2 P1 工程化要求

P1 必须补到：

- statement / ledger / reconciliation 专项指标
- 运营告警
- 审计快照增强
- 回放任务与补偿任务可视化
- 高风险操作权限与审批预留

### 9.3 P2 工程化要求

P2 再增强：

- 成本监控
- 风控监控
- 多币种、多主体全球路由监控
- 更完整的经营分析与利润看板

## 10. P0 的扩展预留要求

为了避免 P1/P2 返工，P0 必须提前预留这些能力位：

### 10.1 数据模型预留

- 所有核心表预留 `metadata`
- 核心配置表预留 `version`
- 规则表预留 `effective_from / effective_to`
- 事件表预留 `charge_group_id`
- 账务表预留 `subject_type / subject_id`
- 交易表预留 `route_snapshot`

### 10.2 接口语义预留

- quota / credits 接口预留 reserve / commit / release 语义
- event ingestion 接口预留幂等键与 replay 能力
- route resolve 接口预留 `billing_profile_key` 和 `merchant_route_hint`
- 商品接口预留状态与版本切换能力

### 10.3 工程能力预留

- 异步任务框架预留死信与重放
- 审计框架预留 diff snapshot
- 指标体系预留 product / org / subject / route 维度
- 日志字段预留 actor / subject / target 三元信息

## 11. 对当前 KYC 和 Menu 的落点

### 11.1 KYC

P0 就要支撑：

- 按 API 计费
- 多 API 打包计费
- quota 校验
- usage SSOT
- 最小 ledger 骨架

P1 补：

- bundle 场景的 statement 解释
- 对账与冲正

P2 再考虑：

- 更复杂套餐编排
- 跨地区主体

### 11.2 Menu

P0 就要支撑：

- `Free / Pro / Growth`
- 月度 credits
- AI 任务按次扣点
- 免费功能与收费功能共存
- 基础 billing profile

P1 补：

- 订阅 / 发票 / 对账
- 运营补偿与客服工具

P2 再扩：

- referral / reward
- wallet
- 多门店 / 代理商 / 品牌方更复杂结算

## 12. 当前建议的建设顺序

结合前面 P0/P1/P2，建议按下面顺序推进：

1. 商品模型
2. 主体与路由模型
3. metering 与 usage SSOT
4. entitlement / quota / credits
5. 基础 ledger 骨架
6. statement / invoice / reconciliation
7. payment / refund / reverse
8. coupon / referral / reward / wallet
9. cost / tax / global routing / advanced risk

## 13. Definition Of Done

如果要说“平台商业化底座 P0 完成”，至少应满足：

- 能表达卖什么、如何收费、归谁结算
- 能表达当前是否可用、还剩多少
- 能记录真实 usage
- 能做最小账务留痕
- 能按组织和主体查账
- 能追踪一次请求到一次计量事件
- 能对高风险变更留审计日志
- 能为 P1/P2 留出字段、语义和任务框架扩展空间

## 14. 最终结论

平台商业化建设不能只看支付、积分、计量其中一个点。

正确方式是：

- 用 `P0` 做最小闭环
- 用 `P1` 做运营与财务闭环
- 用 `P2` 做增长、全球化和精细化能力

但从 `P0` 开始，就必须把这些后续视角预留到模型、接口和工程体系里。
