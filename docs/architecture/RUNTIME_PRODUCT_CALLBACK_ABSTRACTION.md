# Runtime Product Callback Abstraction

Owner: `v-platform-backend` runtime module

## 1. Purpose

This document explains the platform runtime callback abstraction after decoupling the final product callback hop from Menu-specific callback clients.

The goal is to keep provider orchestration inside platform runtime while allowing multiple product backends to receive normalized runtime status and result callbacks through product-owned internal routes.

## 2. Problem Being Solved

Historically, platform runtime used a `MenuClient` and Menu-specific callback payloads for the final callback hop:

- runtime status updates
- runtime result callbacks

That design made platform runtime provider logic reusable, but it prevented new product backends such as Agent Ecommerce from becoming first-class runtime consumers because the last callback hop was still coupled to Menu route paths and Menu DTO names.

## 3. Current Abstraction

Platform runtime now models the last callback hop as a product callback layer:

- `ProductRuntimeCallbackClient`
- `ProductUpdateRuntimeInput`
- `ProductRecordResultsInput`
- `ProductRecordResultVariant`
- `ProductRecordResultAsset`

Runtime executor only depends on these normalized callback contracts. Product-specific route paths are selected by `callback_kind`.

## 4. Callback Kind Routing

Current supported callback kinds:

- `menu_internal`
- `ecommerce_internal`

Each callback kind maps to:

- a product base URL
- an internal service secret
- a runtime status callback path
- a runtime result callback path

Examples:

- `menu_internal`
  - `/internal/v1/menu/studio/jobs/:jobID/runtime`
  - `/internal/v1/menu/studio/jobs/:jobID/results`
- `ecommerce_internal`
  - `/internal/v1/ecommerce/jobs/:jobID/runtime`
  - `/internal/v1/ecommerce/jobs/:jobID/results`

## 5. Runtime Ownership Boundary

Platform runtime owns:

- provider submit / poll / callback orchestration
- provider fallback chains
- result normalization
- platform storage persistence
- final product callback delivery

Product backends own:

- product job tables
- product asset tables
- product callback route semantics
- product-facing asset content routes

## 6. Bootstrap Rule

`RuntimeProductEndpoint.callback_kind` must be explicit for real product consumers.

If a product endpoint uses a callback kind that has no registered callback client factory, platform runtime will not deliver callbacks to that product. For that reason, bootstrap configuration should always declare a concrete supported value such as `menu_internal` or `ecommerce_internal`.

Current platform environment configs should keep `ecommerce_internal` declared alongside `menu_internal` for the Agent Ecommerce runtime consumer:

- local: `http://127.0.0.1:8296`
- dev: `http://v-ecommerce-backend-dev:8396`
- prod: `http://v-ecommerce-backend:8296`

## 7. Why This Matters

This abstraction allows platform runtime to:

- keep provider integration reusable
- support multiple product consumers
- add new product callback implementations without forking executor logic
- preserve a stable boundary where products own business semantics and platform owns execution infrastructure
