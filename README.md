# v-platform-backend

共享平台能力后端仓库的 Go 初始化工程。

## 目标

该仓库用于承载跨产品复用的平台基础能力，包括但不限于：

- 身份与登录注册
- 用户/组织/成员关系
- 角色权限基础设施
- entitlement / subscription 基础能力
- 支付、退款、积分、折扣、奖励等商业基础设施底座
- 计量、账单、事件采集与通用统计基础设施

## 当前状态

- 已选择 Go 作为当前实现技术栈。
- 已落最小可运行服务骨架，包含健康检查、认证、当前用户、组织列表、组织切换、权限读取等首批平台能力。
- 当前实现优先承接身份/组织/权限基础层，后续可在内部继续扩展商业化平台能力模块。

## 目录

```text
v-platform-backend/
├── AGENTS.md
├── README.md
├── cmd/
├── config.local.env.example
├── docs/
│   ├── BACKEND_GUIDE.md
│   └── architecture/
│       └── SERVICE_BOUNDARY.md
├── internal/
├── pkg/
└── test/
```
