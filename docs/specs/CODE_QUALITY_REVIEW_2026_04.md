# Platform Backend — 代码质量审查与修复记录 (2026-04)

> **审查范围**: `v-platform-backend` 全量关键逻辑链路
> **审查维度**: 可观测性 · 标准化 · 健壮性 · 并发安全 · 事务原子性
> **审查日期**: 2026-04-29

---

## 1. 已修复问题 (Critical + High)

### C-1 · 钱包事务内 bucket 查询脱离事务（双花风险）

| 字段 | 值 |
|------|-----|
| **严重级别** | Critical |
| **文件** | `internal/modules/wallet/service.go` |
| **问题** | `debitAccountTx` / `creditAccountTx` / `grantCycleBucketTx` 内部使用 `s.repo.ListSpendableWalletBuckets` 等方法查询 bucket，绕过事务连接读取快照，导致并发扣减场景下可能读到过期余额，造成双花。 |
| **修复** | 将 4 处 `s.repo` 调用替换为直接使用事务内的 `tx` 执行等价查询，确保读-改-写在同一事务隔离级别内完成。 |

### C-2 · findOrCreateWalletAccountTx 缺少行级锁

| 字段 | 值 |
|------|-----|
| **严重级别** | Critical |
| **文件** | `internal/modules/wallet/service.go` |
| **问题** | `findOrCreateWalletAccountTx` 在事务内查询 wallet account 时未加 `SELECT FOR UPDATE`，两个并发事务可能同时判定"不存在"而创建重复账户。 |
| **修复** | 添加 `clause.Locking{Strength: "UPDATE"}`，确保查找-创建序列化。 |

### C-3 · 过期桶列表在事务外读取

| 字段 | 值 |
|------|-----|
| **严重级别** | Critical |
| **文件** | `internal/modules/wallet/service.go` |
| **问题** | `ExpireWalletBuckets` 先在事务外查询待过期桶列表，再进入事务逐条处理，存在 TOCTOU 竞态。 |
| **修复** | 将查询移入事务内并加 `FOR UPDATE` 锁，使查询与更新在同一事务内原子完成。 |

### C-4 · 作业创建与入队非原子操作

| 字段 | 值 |
|------|-----|
| **严重级别** | Critical |
| **文件** | `internal/modules/runtime/service.go` |
| **问题** | `CreateRuntimeJob` 先 `repo.CreateRuntimeJob` 再 `queue.EnqueueDispatch`，若入队失败则 DB 中有已创建但永远不会被处理的孤儿作业。 |
| **修复** | 先检查 `s.queue == nil` 提前失败，再用 `DB().Transaction()` 包裹创建+入队，入队失败时整个事务回滚。 |

### C-5 · 作业状态机无转移校验

| 字段 | 值 |
|------|-----|
| **严重级别** | Critical |
| **文件** | `internal/modules/runtime/service.go` |
| **问题** | `UpdateRuntimeJob` 接受任意状态值写入，可绕过合法状态转移路径（如从 completed 回退到 queued）。 |
| **修复** | 新增 `validRuntimeJobTransitions` 白名单 map 和 `validateRuntimeJobStatusTransition` 校验函数，在更新前校验。 |

### C-6 · ChargeSession 状态机无转移校验

| 字段 | 值 |
|------|-----|
| **严重级别** | Critical |
| **文件** | `internal/modules/runtime/service.go` |
| **问题** | `UpdateChargeSession` 同样缺少状态转移校验，可能导致结算状态被非法篡改。 |
| **修复** | 新增 `validChargeSessionTransitions` 白名单和 `validateChargeSessionStatusTransition` 校验函数。 |

### C-7 · Asynq Worker Start 错误被静默吞没

| 字段 | 值 |
|------|-----|
| **严重级别** | Critical |
| **文件** | `internal/modules/runtime/asynq_runtime.go` |
| **问题** | `Start()` 通过 `go server.Run()` 启动 worker，启动失败时错误无人消费，服务看起来正常但不处理任何任务。 |
| **修复** | 使用 channel + `select/timeout` 模式：500ms 内返回错误则传播给调用方；超时则认为启动成功，后台 goroutine 继续监听意外退出并记录日志。 |

### C-8 · RecordChannelCharge 多步写操作无事务包裹

| 字段 | 值 |
|------|-----|
| **严重级别** | Critical |
| **文件** | `internal/modules/incentive/channel_service.go` |
| **问题** | `RecordChannelCharge` 依次写入 snapshot、ledger、audit 三张表，中间步骤失败会留下不一致的部分数据。 |
| **修复** | 用 `DB().Transaction()` 包裹三步操作，任一步失败整体回滚。同时在 `channel_policy_resolution.go` 新增 `createChannelPolicyResolutionAuditTx` 方法以支持事务内调用。 |

### H-1 · JWT 签名算法未校验（算法混淆攻击）

| 字段 | 值 |
|------|-----|
| **严重级别** | High |
| **文件** | `internal/middleware/jwt.go` |
| **问题** | `jwt.Parse` 的 key function 未校验 token 声称的签名算法，攻击者可构造 `alg: none` 或 `alg: RS256`（使用 HMAC 密钥作为公钥）绕过验证。 |
| **修复** | 在 key function 中增加 `token.Method.(*jwt.SigningMethodHMAC)` 类型断言，非 HMAC 算法直接拒绝。 |

### H-2 · 内部服务密钥比较存在时序攻击风险

| 字段 | 值 |
|------|-----|
| **严重级别** | High |
| **文件** | `internal/middleware/internal_service.go` |
| **问题** | 使用 `==` 比较内部服务密钥，字符串比较在第一个不匹配字节处短路返回，攻击者可通过响应时间差异逐字节猜测密钥。 |
| **修复** | 改用 `crypto/subtle.ConstantTimeCompare`，比较时间恒定。 |

---

## 2. 第二轮修复（High / Medium / Low）

以下问题在第二轮迭代中全部处理完毕：

### H-3 · 非 debug 模式启动校验拒绝默认密钥 ✅
- 新增 `validateSecurityConfig` 函数，在 `gin_mode != debug` 时检查 jwt_secret / kong_shared_secret / internal_service_secret / encryption_key 是否为已知不安全默认值或空值。
- 命中则拒绝启动并给出明确的环境变量覆盖提示。

### H-4 · 优雅停服（Graceful Shutdown） ✅
- `cmd/server/main.go` 改用 `http.Server` + `signal.Notify(SIGINT, SIGTERM)` + `srv.Shutdown(ctx)` 模式。
- 15 秒超时等待在途请求完成。

### H-5 · 外部图片/API 响应下载添加大小限制 ✅
- `fetchImageAsDataURL` 添加 20MB `io.LimitReader`，超限返回错误。
- Volcengine API 响应和 ComfyUI Bridge 响应均添加 50MB `io.LimitReader`。

### H-6 · AutoMigrate 生产环境防护 ✅ (已有)
- 代码中已有 `validateAutoMigratePolicy` 且 `auto_migrate_enabled` 默认 `false`。确认无需额外修复。

### H-7 · 全局速率限制 + 请求体大小限制 ✅
- 新增 `middleware/rate_limit.go`：`RateLimit`（全局令牌桶）、`PerIPRateLimit`（按 IP）、`BodySizeLimit` 三个中间件。
- 路由中接入 `BodySizeLimit(4MB)` 和 `RateLimit(cfg.Security.RateLimitPerSecond, cfg.Security.RateLimitBurst)`。

### H-8 · bcrypt cost 提升至 12 ✅
- 新增 `const bcryptCost = 12`，替换所有 `bcrypt.DefaultCost` 引用。

### M-1 · 请求体大小限制 ✅ (合并至 H-7)
### M-2 · request_id 日志关联 ✅ (已有)
- `access_log.go` 已在每条日志中携带 `request_id` 和 `trace_id`。

### M-3 · DB 连接池参数配置 ✅ (已有 + 增强)
- MaxOpenConns/MaxIdleConns 已有配置。新增 `conn_max_lifetime` 可配置字段（默认 5m），替代原来的硬编码值。

### M-4 · 单元测试覆盖 — 持续关注
- 30 个包全部有测试且通过。后续新增模块时按 Validation Requirements 补测。

### M-5 · Asynq 重试策略 ✅ (确认设计合理)
- 当前设计：任务入队时 `MaxRetry(0)` + 应用层管理重试（re-enqueue with backoff + max_attempts tracking）。
- 服务端设置 `RetryDelayFunc` 作为安全兜底。设计合理，无需修改。

### L-1 · 常量定义位置 ✅ (已集中)
- 所有业务常量已集中在 `pkg/platformconst/status.go` 和 `domain.go`。内部包级常量合理保留在模块内。

### L-2 · TODO 注释 ✅ (无遗留)
- 扫描确认 `internal/` 下无未关联的 TODO 注释。

### L-3 · 日志级别一致性 ✅ (无问题)
- 直接 logger 调用仅 2 处，均为合理的 Error 级别。主要日志通过 access_log 中间件统一输出。

---

## 3. 验证结果

```
$ cd v-platform-backend && go test ./... -count=1
# 30 个测试包全部通过，0 失败（两轮修复后均通过）
```

---

## 4. 影响范围

| 模块 | 修改文件数 | 风险等级 |
|------|-----------|---------|
| wallet (C-1/C-2/C-3) | 1 | 高 — 金融核心路径 |
| runtime (C-4/C-5/C-6/C-7) | 2 | 中 — 作业调度路径 |
| incentive (C-8) | 2 | 中 — 佣金结算路径 |
| middleware (H-1/H-2/H-7) | 3 | 高 — 安全边界 |
| config (H-3) | 1 | 中 — 启动防护 |
| main (H-4) | 1 | 中 — 生命周期管理 |
| provider (H-5) | 2 | 低 — 内部调用 |
| identity (H-8) | 1 | 低 — 哈希成本 |
| storage (M-3) | 1 | 低 — 连接池配置 |

建议在部署后重点监控钱包扣减/充值、运行时作业状态转移、渠道佣金记录三条核心链路的错误率和延迟指标。

---

## 5. 工程原则文档

本次审查总结的通用工程质量原则已沉淀至：

→ [`docs/engineering/GO_SERVICE_QUALITY_PRINCIPLES.md`](../../../docs/engineering/GO_SERVICE_QUALITY_PRINCIPLES.md)

包含十项原则 + 检查清单，供所有 Go 后端项目遵循。
