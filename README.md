# v-platform-backend

`v-platform-backend` 是平台共享能力后端，承接跨产品复用的身份、组织、权限、商业化、运行时、资产存储与内部服务协作能力。

## 目标

该仓库用于承载跨产品复用的平台基础能力，包括但不限于：

- 身份与登录注册
- 用户 / 组织 / 成员关系
- 角色权限基础设施
- entitlement / subscription / wallet / discount / reward / commission
- 商品目录、计量、结算、账务与渠道激励
- runtime job、charge session、provider orchestration
- 平台资产存储注册、内容回读与内部服务分发

## 当前状态

- 已落地 Go + Gin + Gorm 的平台服务主干，同时对外提供 `/api/v1/*` 与 `/internal/v1/*` 两套接口面。
- 身份、组织、权限、平台商业化、runtime、wallet、template ops、asset storage 等核心模块均已接入统一响应、审计、追踪、指标和结构化日志基线。
- 平台商业化真相已收敛到 wallet / metering / settlement / discount / incentive 几条主链路，产品侧通过 internal API 消费平台主控能力。
- runtime 已支持 provider routing、fallback、异步 polling / callback、结果归档到平台 storage，并回调产品后端完成最终业务落账。
- 平台审计表现已独立落到 `platform_audit_logs`，不再与历史业务侧共用 `audit_logs`，避免跨服务 `AutoMigrate` 互相污染共享库表结构。
- 当前轮新增的代码质量与安全加固包括：
  - 非 debug 模式默认密钥拦截
  - JWT 签名算法校验
  - 内部服务密钥常量时间比较
  - 全局速率限制与请求体大小限制
  - graceful shutdown
  - DB `conn_max_lifetime` 配置化
  - 钱包 / runtime / incentive 关键事务原子性修复

## 文档入口

- [docs/BACKEND_GUIDE.md](docs/BACKEND_GUIDE.md): 后端能力范围、工程基线与架构文档索引
- [docs/specs/CODE_QUALITY_REVIEW_2026_04.md](docs/specs/CODE_QUALITY_REVIEW_2026_04.md): 本轮代码质量审查与修复记录

## 开发校验

```bash
go test ./... -count=1
```

## 目录

```text
v-platform-backend/
├── AGENTS.md
├── README.md
├── cmd/
├── docs/
│   ├── BACKEND_GUIDE.md
│   ├── architecture/
│   └── specs/
├── internal/
├── pkg/
└── test/
```
