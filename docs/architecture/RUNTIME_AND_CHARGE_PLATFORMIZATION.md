# Runtime And Charge Platformization

## Goal

Move shared provider execution and charge orchestration into `v-platform-backend`, while keeping product-specific Studio workflow and read models inside `v-menu-backend`.

This document records the first engineering landing:

- shared runtime provider registry
- shared runtime job truth model
- shared runtime attempt evidence
- shared charge session truth model
- internal APIs for Menu and future products

## Boundaries

### Stay In Menu

- Studio assets, variants, style presets
- Menu-specific job modes and front-end read models
- Product-facing APIs under `/api/v1/menu/studio/*`

### Move To Platform

- provider registry and provider capabilities
- runtime jobs and attempts
- charge sessions
- internal execution and charge orchestration APIs

## New Truth Models

Implemented in `internal/models/runtime.go`:

- `RuntimeProviderDefinition`
- `RuntimeJob`
- `RuntimeAttempt`
- `ChargeSession`

These models are now included in platform auto-migrate bootstrap.

## Internal APIs

Implemented in `internal/modules/runtime` and wired into `/internal/v1`:

- `POST /internal/v1/runtime/providers`
- `GET /internal/v1/runtime/providers`
- `POST /internal/v1/runtime/jobs`
- `GET /internal/v1/runtime/jobs/:runtimeJobID`
- `PUT /internal/v1/runtime/jobs/:runtimeJobID`
- `POST /internal/v1/runtime/jobs/:runtimeJobID/cancel`
- `POST /internal/v1/runtime/jobs/:runtimeJobID/attempts`
- `POST /internal/v1/runtime/charge-sessions`
- `GET /internal/v1/runtime/charge-sessions/:chargeSessionID`
- `PUT /internal/v1/runtime/charge-sessions/:chargeSessionID`

## Current Engineering Status

This landing is intentionally foundational:

- platform now owns canonical persistence and internal APIs
- menu platform client now includes runtime job and charge session client methods
- menu workflow has not yet switched from local Studio execution to platform runtime

## Next Engineering Steps

1. Switch `v-menu-backend` Studio charge flow from primitive reserve/finalize calls to `ChargeSession`.
2. Create `RuntimeJob` when Menu creates a Studio job, and persist `runtime_job_id` on the product job.
3. Move provider dispatch and retry execution from Menu into Platform runtime workers.
4. Keep Menu as the owner of product variants/assets, but materialize them from runtime output manifest.
5. Replace local Studio charge history stitching with platform charge session read model.

## Cross-Service IDs

The migration should standardize these anchors:

- `product_job_id`
- `runtime_job_id`
- `charge_session_id`
- `event_id`
- `settlement_id`

All later workflow changes should preserve these IDs end-to-end.
