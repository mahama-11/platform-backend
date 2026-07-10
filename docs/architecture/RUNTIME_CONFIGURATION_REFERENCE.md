# Runtime Configuration Reference

Owner: `v-platform-backend` runtime module

## 1. Purpose

This document explains how to configure platform runtime product endpoints, provider bindings, storage output metadata, and async providers such as `comfyui_bridge` and `pai_video`.

It focuses on the practical question:

- which fields are hard-coded enums in runtime code
- which fields are stable contract strings
- which metadata keys are officially consumed by runtime logic

## 2. Config Sections

The main runtime-related local config lives under:

- `bootstrap.runtime.product_endpoints`
- `bootstrap.runtime.provider_bindings`
- `bootstrap.storage.bindings`
- `runtime`
- `comfyui_bridge`
- `volcengine`
- `pai_video`
- `security.max_body_bytes`
- `security.provider_upload_max_bytes`

Example local shape:

```yaml
bootstrap:
  runtime:
    product_endpoints:
      - product_code: "menu"
        callback_kind: "menu_internal"
        base_url: "http://127.0.0.1:8096"
        secret: "menu-service-secret"
        metadata: '{"output_storage_category":"studio-assets"}'
      - product_code: "ecommerce"
        callback_kind: "ecommerce_internal"
        base_url: "http://127.0.0.1:8296"
        secret: "ecommerce-service-secret"
        metadata: '{"output_storage_category":"ecommerce-assets"}'
    provider_bindings:
      - product_code: "ecommerce"
        task_type: "image_generation"
        provider_code: "comfyui_bridge"
        priority: 50
        enabled: true
        metadata: '{"objective_scores":{"quality":92,"speed":45,"cost":35},"fallback_on":["retryable_provider","provider_timeout","provider_unavailable"]}'
      - product_code: "ecommerce"
        task_type: "image_generation"
        provider_code: "volcengine"
        priority: 100
        enabled: true
        metadata: '{"objective_scores":{"quality":85,"speed":50,"cost":45},"fallback_on":["retryable_provider","provider_timeout","provider_unavailable"]}'

comfyui_bridge:
  enabled: true
  base_url: "http://127.0.0.1:8000"
  api_key: ""
  request_timeout: 60s
  callback_base_url: "http://127.0.0.1:8095"
  default_workflow_id: ""
  default_output_format: "png"
```

## 3. Hard-Coded Enums

These values are explicitly branched in runtime code and should be treated as supported enums.

### 3.1 `callback_kind`

Supported values:

- `menu_internal`
- `ecommerce_internal`

Meaning:

- controls which product callback route contract runtime will call
- maps product code to product-owned internal callback paths

Code source:

- `internal/modules/runtime/product_callback.go`

Important rule:

- `callback_kind` must be explicit
- unsupported values will silently produce no callback client

### 3.2 `provider_code`

Current provider registry names:

- `manual`
- `mock`
- `volcengine`
- `comfyui_bridge` when `comfyui_bridge.enabled=true`

Code source:

- `internal/modules/runtime/provider.go`

Important rule:

- runtime binding may only reference a provider code that the registry actually registered

## 4. Stable Contract Strings

These are not implemented as a central enum table, but they are still runtime contracts and should not be changed casually.

### 4.1 `product_code`

Examples in current environments:

- `menu`
- `ecommerce`

Meaning:

- joins runtime jobs, product endpoints, provider bindings, and storage bindings

### 4.2 `task_type`

P0 known platform runtime task types:

- `image_understanding` — generic visual analysis; P0 contract status is `contract-needed`.
- `ocr` — generic text/region extraction; P0 contract status is `contract-needed`.
- `image_generation` — generate image variants from prompt/source assets; P0 contract status is `ready` when configured.
- `image_inpainting` — masked image edit/fill; P0 contract status is `contract-needed`.
- `video_keyframe` — representative video frame extraction/derivation; P0 contract status is `contract-needed`.

Current configured production-capable value:

- `image_generation`

Meaning:

- the product-side runtime use case that provider bindings match against

Important rules:

- product backends must create runtime jobs using the exact same `task_type` value configured in provider bindings
- task types are generic platform runtime capabilities and must not encode product workflow stages or SKU semantics

### 4.3 Runtime capability matrix API

Internal read API:

```text
GET /internal/v1/runtime/capabilities?product_code=ecommerce
GET /internal/v1/runtime/capabilities?product_code=ecommerce&task_type=image_generation
```

The API derives availability from existing runtime/provider/storage/commercial records without a DB migration:

- `RuntimeProductEndpoint` and supported `callback_kind`
- `StorageBinding` for the resolved output category (`output_storage_category` metadata first, then runtime/fallback bindings)
- `BillableItem` for expected runtime task billing codes such as `ecommerce_runtime_image_generation`
- `RuntimeProviderBinding`, runtime provider registry, and provider definition status
- code-level P0 task contract definitions

Stable reason codes include:

- `contract-needed`
- `provider_binding_missing`
- `provider_binding_disabled`
- `provider_not_registered`
- `provider_inactive`
- `callback_endpoint_missing`
- `callback_kind_unsupported`
- `storage_binding_missing`
- `billable_item_missing`

`contract-needed` is intentionally returned first for P0 draft task types so product backends can branch on a stable unsupported-contract reason instead of treating the gap as only a missing provider configuration.

## 5. Metadata Keys Consumed by Runtime

### 5.1 Product Endpoint Metadata

Current official key:

- `output_storage_category`

Meaning:

- controls which product-oriented storage category runtime should prefer when persisting final generated outputs for that product

Example:

```json
{"output_storage_category":"ecommerce-assets"}
```

Code source:

- `internal/modules/runtime/executor.go`

### 5.2 Provider Binding Metadata

Current official keys:

- `objective_scores`
- `fallback_on`

#### `objective_scores`

Recommended keys:

- `quality`
- `speed`
- `cost`

Default route concept:

- `balanced`

Meaning:

- runtime route ranking uses these scores when a product runtime job declares an `objective` in `route_snapshot`

Example:

```json
{"objective_scores":{"quality":92,"speed":45,"cost":35}}
```

Code source:

- `internal/modules/runtime/route_policy.go`

#### `fallback_on`

Recommended values:

- `retryable_provider`
- `provider_timeout`
- `provider_unavailable`

Meaning:

- if the current provider fails with an allowed error class, runtime may advance to the next provider binding in the candidate chain

Example:

```json
{"fallback_on":["retryable_provider","provider_timeout","provider_unavailable"]}
```

Code source:

- `internal/modules/runtime/route_policy.go`

## 6. Runtime Error Class Values

Current stable runtime/provider error-class values visible to fallback policy:

- `retryable_provider`
- `non_retryable_provider`

Other runtime-owned failure classifications that may appear on jobs:

- `input_manifest_invalid`
- `source_asset_invalid`
- `result_persist_failed`
- `provider_binding_not_found`
- `provider_not_found`

Important rule:

- `fallback_on` should generally only use provider-side retry/fail classes, not product/runtime validation classes

## 7. ComfyUI Bridge Local Defaults

Based on the current API doc and sample requests:

- Bridge API base URL: `http://127.0.0.1:8000`
- Bridge local health payload may report ComfyUI itself behind `http://127.0.0.1:8188`
- Platform callback base URL for local runtime: `http://127.0.0.1:8095`

That means the common local runtime setup is:

- platform runtime calls `http://127.0.0.1:8000`
- bridge service itself talks to ComfyUI internals
- bridge callbacks return to platform runtime on `http://127.0.0.1:8095`

## 8. Recommended Ecommerce Local Setup

Recommended local provider order for ecommerce:

1. `comfyui_bridge`
2. `volcengine`

Reason:

- ecommerce quality-oriented image generation should use ComfyUI Bridge as primary local async provider
- volcengine stays available as a fallback / comparison path

Recommended local callback endpoint:

- `callback_kind: ecommerce_internal`
- `base_url: http://127.0.0.1:8296`

Recommended output storage category:

- `ecommerce-assets`

## 9. Pai Video Provider

`pai_video` reads its credential from `api_key` or `api_key_file`; production-like environments should use the file form and must not commit a real key.

```yaml
security:
  max_body_bytes: 16777216
  provider_upload_max_bytes: 134217728

pai_video:
  enabled: true
  base_url: "https://api.example.com"
  api_key_file: "/run/secrets/pai_video_api_key"
  request_timeout: 60s
```

The larger body limit is bounded to internal runtime provider capability routes. Provider files and remotely imported result assets are limited to 128 MiB by default; oversize inputs fail instead of being truncated.

Paid async execution rules:

- ambiguous submit failures are non-retryable because Platform cannot prove whether the upstream accepted and charged the request;
- after `provider_job_id` is stored, retryable poll failures enqueue another poll only;
- an existing job `timeout_at` is authoritative, and provider calls receive the worker context.

## 10. How To Find Future Supported Values

When unsure whether a config value is an enum, contract string, or free metadata:

1. read the config struct in `internal/config/config.go`
2. search for `switch` / `case` usage in runtime modules
3. search where metadata keys are decoded and consumed
4. treat repository lookup strings such as `product_code` and `task_type` as runtime contracts even if there is no explicit enum list

Practical rule:

- `switch/case` in code: hard enum
- direct DB lookup string: stable contract string
- decoded metadata key: officially consumed metadata key
- documented but not consumed: not a real runtime contract yet
