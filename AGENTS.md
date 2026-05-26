# V-Platform Backend - Agent Context

> **CRITICAL INSTRUCTION**: This repository owns shared platform capabilities. Do not leak product-specific business semantics into this codebase.

## 1. Purpose

`v-platform-backend` is the shared foundation backend for multi-product use.

It should host reusable platform capabilities such as:

- Identity / Auth
- Organization / Membership
- RBAC foundation
- Entitlement / Subscription foundation
- Payment / Refund / Wallet / Coupon infrastructure
- Metering / Billing / Analytics base

It must not absorb product-specific workflows from KYC, Attendance, or Menu AI.

## 1.1 Current Runtime Choice

- Language/runtime: Go
- First implementation stack: Gin + Gorm + JWT
- Current implemented slice: auth, identity, org context, RBAC/access, catalog, commercial routing, quota/credits controls, wallet lifecycle, incentive/referral, metering settlement, audit/docs, and internal service APIs
- Ongoing expansion area: broader subscription/payment coverage, richer analytics base, and deeper OpenAPI / operational coverage across all modules

## 2. Boundary Rules

- Shared execution capability belongs here.
- Product pricing rules, campaign rules, and domain workflows do not.
- Prefer stable APIs and contracts over direct DB sharing.
- Keep product semantics out of platform models and route names.

## 3. Documentation Index

- [**Backend Guide**](docs/BACKEND_GUIDE.md): Entry guide for platform service development.
- [**Internal API Contract**](docs/INTERNAL_API_CONTRACT.md): Internal service access contract, response envelope, idempotency, and retry guidance.
- [**OpenAPI README**](docs/openapi/README.md): Internal Swagger/OpenAPI generation entry and current coverage scope.
- [**Prod Deploy Runbook**](docs/PROD_DEPLOY_RUNBOOK.md): Production deploy, drift-check, runtime smoke, and evidence automation.
- [**Workspace Cloud Dev Deploy Runbook**](../tools/dev/README.md): Cloud dev 部署固定入口；不要用本 repo 旧 `build.sh dev` 或 prod deploy script 伪装 dev 部署。
- [**Observability, Audit, and Diagnostics**](docs/architecture/OBSERVABILITY_AUDIT_DIAGNOSTICS.md): Platform audit API, request log boundaries, metrics, trace backend enablement, and Console diagnostics UX.
- [**Service Boundary**](docs/architecture/SERVICE_BOUNDARY.md): What belongs here and what does not.
- [**Runtime Product Callback Abstraction**](docs/architecture/RUNTIME_PRODUCT_CALLBACK_ABSTRACTION.md): How platform runtime delivers normalized status/result callbacks to multiple product backends without Menu-specific final-hop coupling.
- [**Runtime Configuration Reference**](docs/architecture/RUNTIME_CONFIGURATION_REFERENCE.md): Supported runtime config values, callback kinds, provider codes, metadata keys, and local ComfyUI Bridge setup guidance.
- [**Runtime And Charge Platformization**](docs/architecture/RUNTIME_AND_CHARGE_PLATFORMIZATION.md): First landing scope for shared runtime jobs, charge sessions, and product-facing internal runtime APIs.
- [**Asset Storage Registry And Import**](docs/architecture/ASSET_STORAGE_REGISTRY_AND_IMPORT.md): Platform-owned storage asset registry, import-local contract, and manifest-driven asset ingestion guidance.
- [**Commercialization Boundary**](docs/architecture/COMMERCIALIZATION_BOUNDARY.md): Shared monetization infrastructure versus product-owned business policy.
- [**Commercial Catalog Model**](docs/architecture/COMMERCIAL_CATALOG_MODEL.md): Product, SKU, package, billable item, and rate card model.
- [**Commercial Routing Model**](docs/architecture/COMMERCIAL_ROUTING_MODEL.md): Commercial entity, merchant account, billing profile, and routing policy design.
- [**Metering And Usage SSOT**](docs/architecture/METERING_AND_USAGE_SSOT.md): Metering event contract, usage truth model, and async usage pipeline.
- [**Channel Partner Model And States**](docs/architecture/CHANNEL_PARTNER_MODEL_AND_STATES.md): Detailed platform model, state machines, and event contract for channel revenue share.
- [**Channel Profit Share And Policy Resolution**](docs/architecture/CHANNEL_PROFIT_SHARE_AND_POLICY_RESOLUTION.md): Net-profit revenue share model, policy versioning, assignment override, and conflict/audit rules.
- [**Commercialization Full View And Phases**](docs/architecture/COMMERCIALIZATION_FULL_VIEW_AND_PHASES.md): Full perspective checklist and P0/P1/P2 phased roadmap.
- [**Platform Review For LinkFox Replication**](docs/specs/PLATFORM_REVIEW_FOR_LINKFOX_REPLICATION.md): Capability audit against LinkFox-style SaaS, gap analysis, identity switching design, material system proposal, and prioritized remediation roadmap.
