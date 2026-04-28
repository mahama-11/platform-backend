# Template Ops Import Rules

## Goal

Standardize single-template onboarding and batch onboarding for platform template operations.

This contract is designed for:

- Platform operators editing one template directly in the web UI
- Operations teams maintaining many templates in Excel and importing them as CSV
- Product backends consuming the published platform projection instead of editing seeds directly

## Operating Modes

### 1. Single Onboarding

Use the platform console `Template Ops Center` form when:

- Creating one new template
- Editing one existing template projection
- Reviewing raw/detail payload before publish

Rules:

- Platform is the editing entry
- Save creates or updates a draft projection
- Publish promotes the projection to the product-facing read path

### 2. Batch Onboarding

Use CSV import when:

- Creating many templates at once
- Updating many prompt summaries, tags, or metadata rows
- Performing repeated operations by operations teams in Excel

Recommended workflow:

1. Download CSV template from platform console
2. Edit in Excel
3. Save as UTF-8 CSV
4. Upload or paste CSV into platform console
5. Optionally publish immediately after import

## CSV Contract

Header order:

```text
product_code,template_id,slug,name,summary,status,scope,managed_source,cover_asset_url,cover_asset_id,recommend_score,platforms_json,tags_json,series,capability_type,modality,raw_json,detail_json
```

Required columns:

- `product_code`
- `template_id`
- `name`

Upsert key:

- `product_code + template_id`

Behavior:

- Existing key: update projection
- Missing key: create projection

## Column Semantics

- `product_code`: `menu` or `ecommerce`
- `template_id`: stable business template identifier
- `slug`: human-readable route identifier
- `name`: operator-facing template name
- `summary`: brief description
- `status`: recommended `active`
- `scope`: recommended `official`
- `managed_source`: recommended `ops_manual` for manual platform maintenance, `external_sync` for synced rows
- `cover_asset_url`: preview asset URL if available
- `cover_asset_id`: asset id if managed by platform asset system
- `recommend_score`: integer
- `platforms_json`: JSON array string, example `["xiaohongshu","instagram"]`
- `tags_json`: JSON array string, example `["hero","summer"]`
- `series`: business grouping field
- `capability_type`: business capability field
- `modality`: example `image`
- `raw_json`: flattened business metadata
- `detail_json`: full detail payload used by product-facing display

## JSON Column Rules

The following columns must contain valid JSON strings:

- `platforms_json`
- `tags_json`
- `raw_json`
- `detail_json`

Examples:

```json
["xiaohongshu","instagram"]
```

```json
{"cuisine":"fusion","moods":["fresh"]}
```

```json
{"prompt_templates":{"hero":"demo prompt"}}
```

## Publish Rules

- Imported rows are created/updated in platform projection storage
- If `publish=true`, rows are also published immediately
- Product backends read published platform projections first
- Legacy local template-center storage is currently retained as fallback only

## Real Data Workflow

When onboarding the current real business sources, do not manually assemble CSV rows one by one.

Use the platform export helper first:

```bash
cd /Users/bytedance/Documents/project/go/v/v-platform-backend
python3 scripts/export_real_template_ops_import.py
```

Default outputs:

- `testdata/templateops/real-import/template_ops_real_import.csv`
- `testdata/templateops/real-import/template_ops_real_asset_manifest.json`
- `testdata/templateops/real-import/template_ops_real_import_summary.json`

Current source coverage:

- `v-menu-backend/internal/modules/templatecenter/template_library.seed.json`
- `v-ecommerce-backend/internal/modules/templatecenter/generated_seed_definitions.json`
- `v-ecommerce-backend/internal/modules/templatecenter/example_asset_manifest.json`
- `docs/TEMPLATE_LIBRARY_DOC.md`
- `docs/agent_ecommerce_prompt_模特图系列.md`
- `docs/agent_ecommerce_prompt商品图&套图系列.md`

## Asset Import Rules

CSV is for template projection metadata. Image binaries must be imported through the platform asset registry.

Recommended sequence:

1. Generate real template CSV and asset manifest
2. Import asset manifest into platform storage
3. Import template CSV into `Template Ops Center`
4. Publish after verification

Use the existing asset import helper:

```bash
cd /Users/bytedance/Documents/project/go/v/v-platform-backend
python3 scripts/import_storage_assets.py \
  --manifest ./testdata/templateops/real-import/template_ops_real_asset_manifest.json \
  --base-url http://127.0.0.1:8095 \
  --secret platform-internal-secret \
  --source-root /Users/bytedance/Documents/project/go/v \
  --output ./testdata/templateops/real-import/template_ops_real_asset_manifest.resolved.json
```

Required manifest semantics:

- `assetRef`: source-side relative path or provenance ref
- `sourcePath`: resolved absolute local file path used only during import
- `sourceRef`: stable runtime lookup key, for example `templates/changing-model/M1-T01/example-1`
- `storageFileName`: target file name fragment under platform storage
- `storageKey`: runtime storage reference, returned after import

## Dynamic Path Rules

Do not persist repository-local absolute paths into template runtime payloads.

Rules:

- `docs/*.md#section` is source provenance only and should stay in `raw_json` or `detail_json`
- `infra/examples/...` is import-time asset provenance only
- absolute local filesystem paths such as `/Users/...` may appear only in import manifest `sourcePath`
- product runtime should resolve preview/example images by `sourceRef -> storageKey`, not by repository path

For `ecommerce` examples:

- category: `template-examples`
- source_type: `template_example`
- source_ref: `templates/<toolSlug>/<templateCode>/example-1`

For `menu`:

- current real seed has no example assets
- CSV import can proceed immediately for metadata and prompt payload
- image manifest can be added later if menu begins to maintain example assets in platform storage

## Migration Rules

- New operations entry: platform console
- New read truth for product display: published platform projection
- Legacy `menu/ecommerce` seed or local template store remains fallback during migration
- Do not directly edit old seed files as the primary operating path once a template is managed by platform

## Recommended Team Usage

- Product/operations PM:
  - maintain metadata and prompt payloads in CSV/console
- Designers:
  - maintain cover assets and example payload references
- Engineers:
  - only adjust adapters, schema mapping, and fallback logic

## Next Step

After the batch contract stabilizes:

- add validation report download
- add import preview and diff
- add xlsx adapter if CSV proves insufficient
- deprecate old seed-driven manual editing paths in product backends
