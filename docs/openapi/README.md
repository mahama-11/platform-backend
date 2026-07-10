# OpenAPI / Swagger

## 当前状态

平台当前已经为第一批 internal API 提供 Swagger / OpenAPI 注解，并同时提供浏览器可读的基础文档入口。

当前优先覆盖：

- `controls/reservations`
- `metering/events`
- `metering/settlements`
- `metering/discounts`
- `wallet/accounts`
- `wallet/ledger`
- `incentives/rewards`
- `incentives/commissions`
- `incentives/channel-*`
- `incentives/channel-settlement-*`
- `commercial/route/resolve`
- `runtime/capabilities`
- `runtime/providers/:providerCode` 的余额、TTS voice、文件上传、URL 导入与 provider action

## 生成 internal OpenAPI

先安装 `swag`：

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

然后在 `v-platform-backend` 目录执行：

```bash
./scripts/gen-swagger-internal.sh
```

生成结果输出到：

```bash
docs/openapi/internal/
```

## 说明

- 当前阶段先完成 internal API 的最小可产出规范。
- 当前仓库中已提交的生成脚本只有 `./scripts/gen-swagger-internal.sh`。
- 生成脚本只输出 `swagger.json` 与 `swagger.yaml`，不会生成需要额外 Go 依赖的 `docs.go`。
- 生成产物用于本地/CI 契约校验，不提交到仓库；仓库以 handler 注解、生成脚本和本 README 为真相源。
- `/docs`、`/api/v1/docs/internal-access`、`/api/v1/docs/error-codes` 已作为浏览器侧基础文档入口存在。
- public API 侧目前以路由实现、handler 注解现状和浏览器文档入口为主，尚未看到与 internal 对等的独立生成脚本落在仓库中。
- 现阶段仍建议结合 `docs/INTERNAL_API_CONTRACT.md` 一起阅读：
  - OpenAPI 负责描述接口结构
  - `INTERNAL_API_CONTRACT.md` 负责描述幂等、重试、补偿和接入建议
