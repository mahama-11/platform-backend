# 计量事件与用量真相源设计

## 1. 目标

本文档定义平台侧 metering、usage SSOT、聚合统计与账务证据设计，解决这些问题：

- 产品发生一次业务消费时，平台如何接住计量事件
- 哪些动作需要同步校验，哪些动作应异步计量
- 平台如何保存 usage 真相源
- billing、quota、credits、statement 如何共享这套数据

## 2. 结论先说

主流做法不是“每次业务消费都同步调用一次统计接口并等待落库完成”，而是：

1. 前置同步校验
2. 后置异步计量

具体来说：

- entitlement / quota / credits 校验属于同步链路
- usage event / billing evidence / aggregation 属于异步链路

原因：

- 强约束能力必须能即时拦截
- 统计与账务沉淀不应把产品主链路绑死在平台实时落库上

## 3. 总体架构

推荐分层如下：

1. 产品业务请求
2. 平台同步校验层
3. 产品业务执行
4. 平台异步计量事件接入层
5. usage SSOT 明细层
6. usage 聚合层
7. billing / quota / statement 消费层

一句话：

- 前面同步判断“能不能用”
- 后面异步记录“到底用了多少”

## 4. 同步与异步边界

### 4.1 同步链路负责什么

同步链路适合处理这些动作：

- entitlement 判断
- quota 是否足够
- credits 是否足够
- 当前组织是否允许调用某能力
- 是否需要 reserve 资源

同步链路的目标是：

- 快速返回
- 强约束
- 不做重分析

### 4.2 异步链路负责什么

异步链路适合处理这些动作：

- usage event 入库
- usage 明细持久化
- usage 聚合
- billing ledger 生成
- statement 汇总
- 经营分析指标计算

异步链路的目标是：

- 高吞吐
- 幂等
- 可重放
- 可补偿

## 5. 核心对象

### 5.1 `meter_event`

表示一条标准化计量事件，是平台 usage 真相的源头输入。

建议字段：

- `id`
- `event_id`
- `request_id`
- `trace_id`
- `source_type`
- `source_id`
- `source_action`
- `product_code`
- `org_id`
- `user_id`
- `billable_item_code`
- `charge_group_id`
- `parent_event_id`
- `event_role`
- `usage_units`
- `unit`
- `billable`
- `billing_profile_key`
- `currency_context`
- `dimensions`
- `occurred_at`
- `received_at`
- `status`

说明：

- `event_id` 用作平台幂等键
- `request_id` 用于串联请求链路
- `trace_id` 用于可观测链路
- `dimensions` 用于扩展长尾业务维度
- `charge_group_id` 用于把多次 API 调用归并到同一次业务收费
- `event_role` 用于区分入口计费事件与内部子调用事件

### 5.2 `usage_record`

表示平台确认后的 usage 明细记录，是 usage SSOT 的主明细表。

建议字段：

- `id`
- `event_id`
- `request_id`
- `trace_id`
- `product_code`
- `org_id`
- `user_id`
- `billable_item_code`
- `charge_group_id`
- `event_role`
- `usage_units`
- `billable`
- `billing_profile_id`
- `commercial_entity_id`
- `merchant_account_id`
- `dimensions`
- `occurred_at`
- `recorded_at`

说明：

- `usage_record` 是后续统计、billing、statement 的唯一事实来源
- 平台不应让产品本地 quota 表成为最终真相

### 5.3 `usage_agg`

表示预聚合表，用于提升查询和报表性能。

建议字段：

- `id`
- `product_code`
- `org_id`
- `billable_item_code`
- `time_granularity`
- `stat_time`
- `dimensions`
- `usage_units`
- `event_count`
- `billable_units`
- `updated_at`

建议粒度：

- `hour`
- `day`
- `month`

说明：

- `usage_agg` 只是加速层，不是真相源
- 不允许 billing 直接绕过明细真相源完全依赖聚合表

## 6. 建议事件契约

产品后端向平台发计量事件时，至少应包含：

- `event_id`
- `request_id`
- `product_code`
- `org_id`
- `user_id`
- `billable_item_code`
- `source_action`
- `usage_units`
- `unit`
- `billable`
- `occurred_at`

建议可选字段：

- `billing_profile_key`
- `currency_context`
- `merchant_route_hint`
- `charge_group_id`
- `parent_event_id`
- `event_role`
- `dimensions`

### 6.1 `event_id` 规则

建议：

- 由产品后端生成
- 保证全局唯一
- 一次业务动作只生成一个稳定 event id

不要用：

- 每次重试重新生成新 id

否则平台无法做真正幂等。

### 6.2 `dimensions` 规则

允许扩展，但应限制范围。

建议用于：

- `channel`
- `region`
- `template_id`
- `scene`
- `client_type`

不建议放入：

- 大对象
- 敏感原文
- 请求体全文

## 7. 推荐链路

### 7.1 标准链路

1. `v-menu-backend` 收到业务请求
2. 同步调用平台做 entitlement / quota / credits 校验
3. 产品执行业务
4. 产品生成标准化 `meter_event`
5. 平台异步接收并落 `usage_record`
6. 平台聚合为 `usage_agg`
7. billing、statement、dashboard 消费这些结果

### 7.2 高价值调用链路

对于高成本、高价值能力，可以支持：

1. reserve
2. execute
3. commit
4. release

适用于：

- 大额消耗
- 长耗时任务
- 成功率不稳定的外部调用

第一版可以不完整实现，但接口语义应预留。

### 7.3 多 API 打包计费链路

对于 KYC 这类“多个 API 或内部步骤合并算一次钱”的场景，建议使用以下语义：

1. 入口请求生成一个稳定的 `charge_group_id`
2. 入口计费事件设置 `event_role=entry`
3. 内部子调用事件设置 `event_role=child`
4. 需要合并收费的子调用可以保留 usage 记录，但设置 `billable=false`
5. 最终由入口事件或汇总事件承担真实收费

适用场景：

- OCR + 人脸 + 活体组合成一次套餐收费
- 多个内部 API 协同完成一次外部服务请求
- 一次长流程拆成多个步骤执行，但只允许客户看到一笔收费

### 7.4 单 API 独立计费链路

对于 KYC 这类“按具体 API 单独统计、单独计费”的场景：

- 每个 API 直接映射一个 `billable_item_code`
- `source_action` 记录真实入口动作，例如 `kyc.ocr.submit`
- `charge_group_id` 可为空，或等于当前 `event_id`

这样既可以按 API 出 usage 报表，也可以按 billable item 做账单汇总。

## 8. 为什么要把同步校验和异步计量拆开

如果把统计入库做成同步强依赖，会带来：

- 平台抖动导致产品主链路失败
- 请求延迟显著升高
- 重试与幂等复杂度升高
- usage 采集和业务执行强耦合

拆开的好处是：

- 主链路更稳
- 可异步削峰
- 更适合后续扩展聚合、账单、分析

## 9. 为什么 usage SSOT 必须单独存在

如果直接依赖：

- quota 表
- billing summary 表
- dashboard 聚合表

来充当真实 usage，会带来这些问题：

- 统计口径互相污染
- 无法精确对账
- 无法做事件级回放
- 退款、补偿、冲正难做

所以必须有一张清晰的 usage 明细真相源表。

## 10. 与 KYC 当前设计的衔接

`v-backend` 当前已有几个值得继承的点：

- `usage_logs` 作为真相源思路是对的
- `request_id` 串联链路是对的
- Redis Stream 异步消费思路是对的
- `Billable`、`UsageUnits` 这些字段设计思路是对的

但平台化后要进一步加强：

- 从单产品 usage log 升级为多产品统一 meter event
- 增加 `product_code`、`billable_item_code`、`billing_profile_id`
- 增加 `charge_group_id`、`source_action`、`event_role`，支撑按 API 计费与打包计费并存
- 明确 usage record 与 quota ledger、billing ledger 的边界
- 把异步链路的重放、补偿、告警能力纳入标准设计

## 11. 处理语义

### 11.1 幂等

必须保证：

- 同一个 `event_id` 重复投递不会重复入账
- 同一个 callback 重放不会重复生成 billing 记录

### 11.2 顺序

不强依赖全局顺序，但应保证：

- 同一业务动作的重复事件不会造成多次计费
- 需要顺序敏感的账务场景依赖 ledger 层处理，而不是依赖 usage 事件天然顺序

### 11.3 补偿

需要支持：

- 事件接收成功但聚合失败
- usage 成功但 billing 衍生失败
- 异步任务执行中断后恢复

### 11.4 回放

需要支持：

- 按时间段回放
- 按 product 回放
- 按 org 回放
- 按 event id 定点重放

## 12. 可观测性与工程化要求

### 12.1 Trace

必须能串起来：

- 产品请求
- entitlement 校验
- metering 事件发送
- 平台事件消费
- usage 落库
- usage 聚合
- billing ledger 生成

### 12.2 Metrics

建议至少有这些指标：

- `meter_event_ingest_total`
- `meter_event_ingest_failed_total`
- `meter_event_duplicate_total`
- `usage_record_write_total`
- `usage_agg_update_total`
- `usage_pipeline_lag_seconds`
- `usage_replay_total`

### 12.3 Logging

结构化日志必须带：

- `request_id`
- `trace_id`
- `event_id`
- `product_code`
- `org_id`
- `billable_item_code`

### 12.4 Audit

以下操作应记录审计：

- 手工补录 usage
- 手工冲正 usage
- 回放任务触发
- 计量规则变更

## 13. 推荐的第一版数据表

第一版建议先做：

- `meter_events`
- `usage_records`
- `usage_aggs`
- `usage_replay_jobs`

后续再补：

- `quota_ledgers`
- `billing_ledgers`
- `statement_snapshots`

## 14. 第一版实施建议

先做：

1. 定义事件契约
2. 定义幂等键
3. 落 usage 明细真相源
4. 做日级聚合
5. 做基本监控和告警

第一版先不急着做：

- 过度复杂的实时多维 OLAP
- 复杂计费 DSL
- 很重的运营分析模型

## 15. DoD

这一模块进入可实施状态的标准：

- 已明确同步校验与异步计量边界
- 存在统一 `meter_event` 契约
- 存在独立 `usage_record` 作为 SSOT
- 聚合表只作为加速层，不冒充真相源
- 具备幂等、重试、回放、补偿、监控基础能力
- 能支撑至少一个 Menu 场景的完整用量链路
