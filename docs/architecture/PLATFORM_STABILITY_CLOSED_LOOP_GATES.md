# Platform Stability Closed-Loop Gates

## Purpose

This is the shared Platform stability contract for V. Platform is a common middle layer for Ecom/Menu/KYC, so producer-only tests or product-only smoke are not enough.

A Platform stability claim is complete only when the same gate run proves:

1. core API/DTO/state contracts are stable;
2. runtime/provider/callback/result asset paths form a real closed loop;
3. quota/metering/grant-policy and wallet/charge paths are internally consistent;
4. storage registry/product asset consumption remains compatible;
5. audit/observability evidence carries request_id and trace_id without leaking secrets;
6. Ecom/Menu/KYC consumers are swept;
7. failures are fail-closed and produce RepairTask/fix work instead of being hidden as PASS.

## Blocking Domains

- auth / org / RBAC:
  - stable auth envelope, JWT/session validation, organization context, membership, access checks;
  - negative cases for missing/invalid auth, permission denial, and product-scope ambiguity.
- wallet / quota / grant-policy / metering:
  - reserve/commit/release, wallet debit/credit ledger, quota exhaustion, metering event idempotency, settlement/reverse;
  - no overdraw/overconsume on concurrent or retried operations.
- runtime job / provider routing / fallback:
  - create runtime job, route to provider binding, accept provider attempt, schedule fallback on retryable failure;
  - explicit unsupported provider/input-mode requests fail closed.
- callback / result ingestion:
  - callback writes terminal state and result assets;
  - duplicate, stale, late, or out-of-order callback does not downgrade terminal jobs.
- storage registry / asset binding:
  - source assets are registered through Platform storage, resolved by workers, and exposed to products through product-owned authorized routes;
  - products must not depend on raw local paths or public `/storage/*` assumptions.
- audit / observability event:
  - critical state changes emit audit/event/log evidence with request_id, trace_id, runtime_job_id/provider_attempt_id where applicable;
  - logs redact tokens, service secrets, provider keys, face/OCR/image sensitive payloads.
- downstream Ecom/Menu/KYC consumer regression:
  - backend consumers compile against current Platform contracts;
  - release/deploy scoped changes also run product journey smoke in the target lane.

## Executable Gate

Primary SelfCheck feature:

```bash
cd /root/work/agentic-selfcheck
scripts/requirement-gate.sh platform-stability-closed-loop-gates static requirement.changed.v.platform-stability-closed-loop-gates
```

The gate writes:

```text
/root/work/v/reports/platform-stability-closed-loop-gates/platform-stability-closed-loop-static.json
```

It fails closed if any of the following fail in the same run:

- `platform-core-engineering-baseline` contract/static scanner;
- runtime/provider/callback/result focused Go tests;
- financial/quota/metering/grant-policy focused Go tests;
- package activation / signup quota focused Platform + Menu contract tests;
- Ecom/Menu/KYC backend consumer compile sweep;
- Ecom/Menu/KYC frontend client/typecheck sweep;
- selector or documentation manifest checks.

## Deployment/Live Evidence Add-On

The static gate is the PR/merge stability floor. It is not a prod/deploy pass by itself.

For deployment or production readiness claims, additionally run the target-lane gates:

```bash
cd /root/work/agentic-selfcheck
scripts/requirement-gate.sh platform-ops-visible-baseline api,evidence requirement.changed.v.platform-ops-visible-baseline
scripts/requirement-gate.sh ecommerce-critical-journey-release-gate static,api,browser,evidence release.v.ecommerce
```

For Menu/KYC release scope, use their corresponding live smoke/runbook once added. Until such product gates exist, report the gap as PASS_WITH_NOTES or BLOCKED, never as full stability closure.

## Selector Policy

Any change touching Platform shared surfaces selects `platform-stability-closed-loop-gates` in addition to narrower gates:

- `platform-backend/internal/modules/runtime/**`
- `platform-backend/internal/modules/wallet/**`
- `platform-backend/internal/modules/metering/**`
- `platform-backend/internal/modules/control/**`
- `platform-backend/internal/modules/commercial/**`
- `platform-backend/internal/modules/catalog/**`
- `platform-backend/internal/modules/incentive/**`
- `platform-backend/internal/modules/assetstorage/**`
- `platform-backend/internal/modules/audit/**`
- `platform-backend/internal/modules/diagnostics/**`
- `platform-backend/internal/observability/**`
- `platform-backend/internal/telemetry/**`
- `platform-backend/docs/architecture/**`
- Platform route/OpenAPI/internal-contract files.

Unknown or newly added Platform shared capability paths should default to this composite gate until a narrower selector exists.
