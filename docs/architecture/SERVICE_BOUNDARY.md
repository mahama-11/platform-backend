# Platform Service Boundary

## 1. Platformized Layers

These concerns should be platformized when reused by multiple products:

- Auth identity truth
- Organization and membership truth
- RBAC foundation
- Entitlement and subscription base
- Payment channel adapters
- Refund base workflow
- Wallet / points / coupon ledgers
- Metering event ingestion
- Billing statement generation base
- Analytics collection and shared event pipeline

## 2. Product-Owned Layers

These remain product-owned:

- Price strategy
- Refund policy
- Promotion campaign rules
- Referral commission formulas
- Reward issuance rules
- KPI definition and business reporting semantics

## 3. Migration Principle

Do not keep two long-lived identity truths.
Use the platform service as the current shared source of truth for identity, org membership, wallet, metering, and reusable commercialization capabilities.

## 4. Public vs Internal APIs

Frontend-direct public APIs should stay in `/api/v1/*`, for example:

- `/api/v1/auth/login`
- `/api/v1/auth/register`
- `/api/v1/auth/me`
- `/api/v1/orgs`
- `/api/v1/orgs/switch`
- `/api/v1/access/permissions/me`

Service-to-service internal APIs should stay in `/internal/v1/*` and require an internal service secret, for example:

- `/internal/v1/users/:userID/profile`
- `/internal/v1/access/users/:userID/orgs/:orgID`

Rule:

- Browsers may call public APIs.
- Product backends may call internal APIs.
- Internal APIs must not be exposed by the public gateway unless explicitly intended.
