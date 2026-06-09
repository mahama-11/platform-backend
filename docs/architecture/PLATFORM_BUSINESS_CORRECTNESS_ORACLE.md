# Platform Business Correctness Oracle

目标：把“逻辑严密”升级为“业务正确性可验证”。本文件不是单纯覆盖率说明，而是把业务 oracle 固定成可执行 gate：

```text
business invariant -> Test* / smoke / SelfCheck command -> report path -> CI gate
```

## 1. 正确性指标

| 指标 | 定义 | 当前 gate 来源 |
|---|---|---|
| invariant pass rate | 关键业务不变量通过数 / 不变量总数 | `scripts/business-correctness-gate.py` |
| critical journey pass rate | 关键 journey 通过数 / journey 总数 | `scripts/business-correctness-gate.py` |
| contract compatibility pass rate | contract compile/smoke 通过率 | `contract_compatibility_compile` |
| prod drift critical count | live prod drift check 中 CRITICAL 数量 | `tools/prod/platform-drift-check.sh --env prod --fail-on-critical` |
| runtime job terminal success/failure distribution | completed/failed/canceled 对 charge session 的终态覆盖 | runtime targeted Go tests；live 分布需另接 prod/runtime evidence |
| charge session settled/released/canceled reconciliation | charge session 是否按 runtime terminal 状态 settle/release/cancel | runtime charge-session tests |
| callback delivery success rate | callback 成功、签名错误、endpoint 缺失/不可达是否按预期处理 | runtime callback/product callback tests |
| quota/wallet ledger reconciliation mismatch count | reserve/finalize/wallet ledger 是否重复扣减或 over-reserve | control/metering/wallet targeted tests |
| consumer contract break count | consumer compile / internal API contract 是否破坏 | `go test ./... -run ^$` + SelfCheck downstream sweep |

## 2. 不变量 traceability

| invariant | business invariant | Test* / smoke / SelfCheck command | report path | CI gate |
|---|---|---|---|---|
| INV-AUTH-001 | 注册成功后必须有默认 active org context、owner membership、starter plan，登录/Me 能拿到同一 org 权限上下文。 | `go test ./internal/modules/identity ./internal/modules/access -run '^(TestIdentityServiceRegisterLoginAndProfileFlow|TestSeedDefaultsAndAccessContext|TestHandlerMePermissionsAndInternalMembershipAccess|TestIdentityServiceOwnerRoleDoesNotImplyPlatformAdmin|TestIdentityServicePlatformAdminFlagGrantsPlatformPermission)$'` | `reports/business-correctness/business-correctness-gate-report.json` | `scripts/business-correctness-gate.py` in CI |
| INV-MENU-SIGNUP-001 | Menu signup package 激活必须幂等；不能重复发额度；无 active policy/package 时 fail-closed 且不能留下部分 grant。 | `go test ./internal/modules/control ./internal/migration -run '^(TestActivatePackageAppliesPoliciesAndIsIdempotent|TestActivatePackageFailsClosedWithoutActivePolicies|TestActivatePackageRejectsInactiveCommercialPackageWithoutPartialQuotaGrant|TestSeedMenuOfferings)$'`；SelfCheck: `platform-stability-closed-loop-gates` | `reports/business-correctness/business-correctness-gate-report.json`；`reports/platform-stability-closed-loop-gates/platform-stability-closed-loop-static.json` | CI + SelfCheck |
| INV-PRODUCT-SCOPE-001 | `product_code` 缺失时不能返回跨产品钱包/计量/settlement 数据；显式 `include_all_products=true` 才能跨产品。 | `go test ./internal/modules/runtime ./internal/modules/control ./internal/modules/metering -run '^(...|TestMeteringListHandlersRejectMissingOrConflictingProductScope)$'` | `reports/business-correctness/business-correctness-gate-report.json` | CI |
| INV-QUOTA-001 | quota reserve 成功但业务失败必须 release；commit/release/finalize 必须幂等，不能 over-reserve 或重复结算。 | `quota_reserve_commit_release_journey` + `failure_path_journey` | `reports/business-correctness/business-correctness-gate-report.json` | CI |
| INV-RUNTIME-CHARGE-001 | runtime completed 必须 settle charge session；failed/canceled 必须 release/cancel charge session；boundary mismatch 必须回滚 runtime terminal update。 | `runtime_callback_asset_charge_journey` + `failure_path_journey` | `reports/business-correctness/business-correctness-gate-report.json` | CI |
| INV-CALLBACK-001 | provider callback 必须校验签名；晚到 progress/callback 不能覆盖 terminal result；产品 endpoint 缺失或不可达必须失败可见。 | `runtime_callback_asset_charge_journey` + `failure_path_journey`；live prod: `tools/prod/platform-drift-check.sh --env prod --fail-on-critical` | `reports/business-correctness/business-correctness-gate-report.json`；prod drift stdout evidence | CI；live evidence gate requires approved prod run |
| INV-ASSET-CONSUMER-001 | provider result asset / inline output 必须规范化并注册到 storage，保证 product callback 可消费。 | `TestHandleProviderCallbackPayloadNormalizesOutputManifestAndRegistersStorage`；`TestHandleProviderCallbackPayloadNormalizesTextInlineDataForStorage`；SelfCheck downstream consumer compile sweep | `reports/business-correctness/business-correctness-gate-report.json` | CI + SelfCheck |
| INV-AUDIT-TRACE-001 | 所有成本动作必须具备 metering / audit / request_id / trace_id 证据，不能产生无上下文的成本副作用。 | `TestMeteringHandlerIngestBackfillsRequestTraceAndActorContext`；`TestAuditServiceRecordAndHelpers`；runtime charge tests | `reports/business-correctness/business-correctness-gate-report.json` | CI |
| INV-CONTRACT-001 | Platform 内部 API/DTO/consumer contract 必须编译兼容；破坏消费者时 gate fail-closed。 | `go test ./... -run '^$' -count=1`；SelfCheck Ecom/Menu/KYC backend compile + frontend typecheck | `reports/business-correctness/business-correctness-gate-report.json`；`reports/platform-stability-closed-loop-gates/platform-stability-closed-loop-static.json` | CI + SelfCheck |

## 3. Critical journey gates

| journey | executable command |
|---|---|
| Platform auth/org/access journey | `auth_org_access_journey` |
| Menu signup package activation journey | `menu_signup_package_activation_journey` |
| Platform quota reserve/commit/release journey | `quota_reserve_commit_release_journey` |
| Runtime create → provider accepted → callback → result asset → charge settlement journey | `runtime_callback_asset_charge_journey` |
| 失败链路：余额不足、provider fail、重复 callback、secret mismatch、产品 endpoint 不可达 | `failure_path_journey` + approved live `platform-drift-check` |

## 4. Gate usage

Local/CI correctness gate:

```bash
python3 scripts/business-correctness-gate.py
```

Approved prod/live evidence after drift repair:

```bash
python3 scripts/business-correctness-gate.py --run-prod-drift --require-prod-drift
```

The live prod flag is deliberately explicit because prod drift checks depend on SSH/topology and must not silently run in GitHub CI. The correctness claim is only full when local business oracle passes and live prod drift returns `critical=0`.

## 5. Interpretation

- `PASS` in `business-correctness-gate.py` means the local executable business oracle passed in one run.
- `PASS` with `--run-prod-drift --require-prod-drift` means local oracle plus read-only prod drift evidence passed in one run.
- Coverage remains a quality floor, not a correctness oracle. The oracle above proves that key business invariants are tied to executable tests/smokes and fail-closed gates.
