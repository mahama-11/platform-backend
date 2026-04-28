# Asset Storage Registry And Import

## Goal

Platform-managed static assets such as template example images must not depend on source-repo filesystem paths at runtime.

The platform should own:

- storage bindings
- binary ingestion
- asset metadata registry
- stable runtime references

## Runtime Reference Rules

- `assetRef`: source-side provenance only; useful for re-import and auditing
- `sourceRef`: stable logical identifier used by platform metadata registry
- `storageKey`: runtime storage reference returned by platform storage

Template systems should eventually depend on `storageKey`, not repository-local paths.

## Metadata Registry

The platform persists imported assets in `storage_assets`.

Recommended values for template example imports:

- `product_code = ecommerce`
- `category = template-examples`
- `source_type = template_example`
- `source_ref = templates/<toolSlug>/<templateCode>/example-1`

## Import API

Internal endpoint:

- `POST /internal/v1/storage/assets/import-local`

Request body:

```json
{
  "product_code": "ecommerce",
  "category": "template-examples",
  "source_type": "template_example",
  "source_ref": "templates/changing-model/M1-T01/example-1",
  "source_path": "/data/examples/Model/ModelSwap/欧美白人女模特.png",
  "storage_file_name": "changing-model/m1-t01-example-1.png",
  "title": "欧美白人女模特",
  "description": "template example asset",
  "tags": ["template-example", "changing-model", "M1-T01"],
  "metadata": {
    "templateCode": "M1-T01",
    "toolSlug": "changing-model"
  }
}
```

## Manifest-driven Import

Use `scripts/import_storage_assets.py` with a manifest such as:

```json
{
  "version": 1,
  "items": [
    {
      "productCode": "ecommerce",
      "category": "template-examples",
      "sourceType": "template_example",
      "sourceRef": "templates/changing-model/M1-T01/example-1",
      "assetRef": "infra/examples/Model/ModelSwap/欧美白人女模特.png",
      "storageFileName": "changing-model/m1-t01-example-1.png",
      "title": "欧美白人女模特",
      "description": "template example asset",
      "tags": ["template-example", "changing-model", "M1-T01"],
      "metadata": {
        "templateCode": "M1-T01",
        "toolSlug": "changing-model"
      }
    }
  ]
}
```

Example:

```bash
python3 scripts/import_storage_assets.py \
  --manifest ./example_asset_manifest.json \
  --base-url http://127.0.0.1:8195 \
  --secret platform-internal-secret \
  --source-root /workspace/project-root \
  --output ./example_asset_manifest.resolved.json
```

The resolved manifest can later be consumed by template seed generation to inject `storageKey` back into template examples.
