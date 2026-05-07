#!/usr/bin/env python3
from __future__ import annotations

import argparse
import csv
import json
from collections import OrderedDict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


CSV_COLUMNS = [
    "product_code",
    "template_id",
    "slug",
    "name",
    "summary",
    "status",
    "scope",
    "managed_source",
    "cover_asset_url",
    "cover_asset_id",
    "recommend_score",
    "platforms_json",
    "tags_json",
    "series",
    "capability_type",
    "modality",
    "raw_json",
    "detail_json",
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Export real menu/ecommerce template sources into platform CSV and asset manifest."
    )
    parser.add_argument(
        "--workspace-root",
        default="",
        help="Workspace root. Defaults to the parent directory of v-platform-backend.",
    )
    parser.add_argument(
        "--output-dir",
        default="testdata/templateops/real-import",
        help="Output directory, relative to v-platform-backend unless absolute.",
    )
    return parser.parse_args()


def default_workspace_root(script_path: Path) -> Path:
    return script_path.resolve().parents[2]


def resolve_output_dir(base_dir: Path, output_dir: str) -> Path:
    path = Path(output_dir)
    if path.is_absolute():
        return path
    return (base_dir / path).resolve()


def read_json(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def resolve_source_file(workspace_root: Path, candidates: list[str]) -> Path:
    """Resolve a source fixture under the workspace.

    The workspace has used both current product directory names
    (menu-backend/ecommerce-backend) and legacy names
    (v-menu-backend/v-ecommerce-backend).  Preserve compatibility by trying
    explicit relative candidates in order and failing with the full search list.
    """
    attempted: list[Path] = []
    for candidate in candidates:
        path = Path(candidate)
        if not path.is_absolute():
            path = workspace_root / path
        path = path.resolve()
        attempted.append(path)
        if path.exists():
            return path
    attempted_text = "\n".join(f"  - {path}" for path in attempted)
    raise FileNotFoundError(f"source file not found; tried:\n{attempted_text}")


def json_text(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"))


def normalize_ref_path(ref: str) -> str:
    return ref.split("#", 1)[0].strip()


def resolve_workspace_path(workspace_root: Path, ref: str) -> str:
    normalized = normalize_ref_path(ref)
    if not normalized:
        return ""
    candidate = Path(normalized)
    if not candidate.is_absolute():
        candidate = workspace_root / candidate
    return str(candidate.resolve())


def resolve_existing_workspace_path(workspace_root: Path, ref: str) -> str:
    absolute = resolve_workspace_path(workspace_root, ref)
    if not absolute:
        return ""
    return absolute if Path(absolute).exists() else ""


def first_non_empty(*values: str) -> str:
    for value in values:
        if isinstance(value, str) and value.strip():
            return value.strip()
    return ""


def tool_slug_from_slug_and_code(slug: str, external_code: str) -> str:
    slug = (slug or "").strip()
    external_code = (external_code or "").strip().lower()
    if not slug or not external_code:
        return ""
    marker = f"-{external_code}-"
    index = slug.find(marker)
    if index <= 0:
        return ""
    return slug[:index]


def build_menu_rows(workspace_root: Path) -> list[dict[str, str]]:
    seed_path = resolve_source_file(
        workspace_root,
        [
            "menu-backend/internal/modules/templatecenter/template_library.seed.json",
            "v-menu-backend/internal/modules/templatecenter/template_library.seed.json",
        ],
    )
    library = read_json(seed_path)
    templates = library.get("templates", [])
    rows: list[dict[str, str]] = []
    for item in templates:
        tags = item.get("tags") or []
        raw = {
            "cuisine": item.get("cuisine", ""),
            "dish_type": item.get("dish_type", ""),
            "plan": item.get("plan", ""),
            "credits_cost": item.get("credits_cost", 0),
            "layout": item.get("layout", ""),
            "lighting": item.get("lighting", ""),
            "props": item.get("props") or [],
            "moods": item.get("moods") or [],
            "metadata": item.get("metadata") or {},
            "source_doc_ref": "docs/TEMPLATE_LIBRARY_DOC.md",
            "source_seed_ref": str(seed_path.relative_to(workspace_root)),
        }
        rows.append(
            {
                "product_code": "menu",
                "template_id": item.get("id", ""),
                "slug": item.get("slug", ""),
                "name": item.get("name", ""),
                "summary": item.get("description", ""),
                "status": "active",
                "scope": "official",
                "managed_source": "seed_import",
                "cover_asset_url": "",
                "cover_asset_id": "",
                "recommend_score": str(item.get("credits_cost", 0) or 0),
                "platforms_json": json_text(item.get("platforms") or []),
                "tags_json": json_text(tags),
                "series": item.get("cuisine", ""),
                "capability_type": "menu_template",
                "modality": "image",
                "raw_json": json_text(raw),
                "detail_json": json_text(item),
            }
        )
    return rows


def build_ecommerce_rows_and_manifest(workspace_root: Path) -> tuple[list[dict[str, str]], dict[str, Any]]:
    definitions_path = resolve_source_file(
        workspace_root,
        [
            "ecommerce-backend/internal/modules/templatecenter/generated_seed_definitions.json",
            "v-ecommerce-backend/internal/modules/templatecenter/generated_seed_definitions.json",
        ],
    )
    manifest_path = resolve_source_file(
        workspace_root,
        [
            "ecommerce-backend/internal/modules/templatecenter/example_asset_manifest.json",
            "v-ecommerce-backend/internal/modules/templatecenter/example_asset_manifest.json",
        ],
    )
    definitions = read_json(definitions_path)
    existing_manifest = read_json(manifest_path)

    manifest_items_by_source_ref: OrderedDict[str, dict[str, Any]] = OrderedDict()
    for item in existing_manifest.get("items", []):
        source_ref = item.get("sourceRef", "").strip()
        if source_ref:
            manifest_items_by_source_ref[source_ref] = dict(item)

    rows: list[dict[str, str]] = []
    missing_assets: list[dict[str, Any]] = []

    for definition in definitions:
        external_code = definition.get("externalCode", "")
        locale_zh = definition.get("localeZH") or {}
        locale_en = definition.get("localeEN") or {}
        examples = definition.get("examples") or []
        tool_slug = tool_slug_from_slug_and_code(definition.get("slug", ""), external_code)

        combined_tags = []
        for group in (
            definition.get("platformTags") or [],
            definition.get("industryTags") or [],
            definition.get("scenarioTags") or [],
        ):
            combined_tags.extend(group)

        example_source_refs = [example.get("sourceRef", "") for example in examples if example.get("sourceRef")]
        example_asset_refs = [example.get("assetRef", "") for example in examples if example.get("assetRef")]

        source_asset_ref = definition.get("sourceAssetRef", "")
        raw = {
            "external_code": external_code,
            "executor_type": definition.get("executorType", ""),
            "interaction_mode": definition.get("interactionMode", ""),
            "featured": bool(definition.get("featured", False)),
            "source_asset_ref": source_asset_ref,
            "source_doc_path": resolve_workspace_path(workspace_root, source_asset_ref) if source_asset_ref else "",
            "industry_tags": definition.get("industryTags") or [],
            "scenario_tags": definition.get("scenarioTags") or [],
            "tool_binding": definition.get("toolBinding") or {},
            "execution_schema": definition.get("executionSchema") or {},
            "example_source_refs": example_source_refs,
            "example_asset_refs": example_asset_refs,
        }

        rows.append(
            {
                "product_code": "ecommerce",
                "template_id": definition.get("id", ""),
                "slug": definition.get("slug", ""),
                "name": first_non_empty(locale_zh.get("name", ""), definition.get("externalCode", ""), definition.get("id", "")),
                "summary": first_non_empty(locale_zh.get("summary", ""), locale_en.get("summary", "")),
                "status": "active",
                "scope": "official",
                "managed_source": "seed_import",
                "cover_asset_url": "",
                "cover_asset_id": "",
                "recommend_score": str(definition.get("recommendScore", 0) or 0),
                "platforms_json": json_text(definition.get("platformTags") or []),
                "tags_json": json_text(combined_tags),
                "series": definition.get("series", ""),
                "capability_type": definition.get("capabilityType", ""),
                "modality": definition.get("modality", ""),
                "raw_json": json_text(raw),
                "detail_json": json_text(definition),
            }
        )

        for example in examples:
            source_ref = example.get("sourceRef", "").strip()
            asset_ref = example.get("assetRef", "").strip()
            if not source_ref or not asset_ref:
                continue

            manifest_item = dict(manifest_items_by_source_ref.get(source_ref, {}))
            manifest_item.setdefault("productCode", "ecommerce")
            manifest_item.setdefault("category", "template-examples")
            manifest_item.setdefault("sourceType", "template_example")
            manifest_item["sourceRef"] = source_ref
            manifest_item["assetRef"] = asset_ref
            manifest_item["storageFileName"] = first_non_empty(
                manifest_item.get("storageFileName", ""),
                example.get("storageFileName", ""),
            )
            manifest_item["title"] = first_non_empty(
                manifest_item.get("title", ""),
                example.get("title", ""),
                locale_zh.get("name", ""),
                external_code,
            )
            manifest_item["description"] = first_non_empty(
                manifest_item.get("description", ""),
                example.get("description", ""),
                "template example asset",
            )
            manifest_item["tags"] = manifest_item.get("tags") or [
                "template-example",
                tool_slug,
                external_code,
            ]
            metadata = dict(manifest_item.get("metadata") or {})
            metadata.update(
                {
                    "templateID": definition.get("id", ""),
                    "externalCode": external_code,
                    "templateName": first_non_empty(locale_zh.get("name", ""), external_code),
                    "toolSlug": tool_slug,
                    "exampleId": example.get("id", ""),
                    "assetRef": asset_ref,
                    "sourceAssetRef": source_asset_ref,
                }
            )
            source_path = resolve_existing_workspace_path(workspace_root, asset_ref)
            if source_path:
                metadata["resolvedAssetPath"] = source_path
                manifest_item["sourcePath"] = source_path
            else:
                missing_assets.append(
                    {
                        "sourceRef": source_ref,
                        "assetRef": asset_ref,
                        "expectedPath": resolve_workspace_path(workspace_root, asset_ref),
                    }
                )
            manifest_item["metadata"] = metadata
            manifest_items_by_source_ref[source_ref] = manifest_item

    manifest = {
        "version": 1,
        "generatedBy": "export_real_template_ops_import.py",
        "generatedAt": datetime.now(timezone.utc).isoformat(),
        "workspaceRoot": str(workspace_root),
        "items": list(manifest_items_by_source_ref.values()),
        "missingAssets": missing_assets,
    }
    return rows, manifest


def write_csv(path: Path, rows: list[dict[str, str]]) -> None:
    with path.open("w", encoding="utf-8", newline="") as fp:
        writer = csv.DictWriter(fp, fieldnames=CSV_COLUMNS)
        writer.writeheader()
        writer.writerows(rows)


def write_json(path: Path, payload: Any) -> None:
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def main() -> None:
    args = parse_args()
    script_path = Path(__file__)
    platform_backend_root = script_path.resolve().parents[1]
    workspace_root = (
        Path(args.workspace_root).resolve()
        if args.workspace_root
        else default_workspace_root(script_path)
    )
    output_dir = resolve_output_dir(platform_backend_root, args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    menu_rows = build_menu_rows(workspace_root)
    ecommerce_rows, asset_manifest = build_ecommerce_rows_and_manifest(workspace_root)
    all_rows = menu_rows + ecommerce_rows

    csv_path = output_dir / "template_ops_real_import.csv"
    manifest_path = output_dir / "template_ops_real_asset_manifest.json"
    summary_path = output_dir / "template_ops_real_import_summary.json"

    write_csv(csv_path, all_rows)
    write_json(manifest_path, asset_manifest)
    write_json(
        summary_path,
        {
            "generatedAt": datetime.now(timezone.utc).isoformat(),
            "workspaceRoot": str(workspace_root),
            "csvPath": str(csv_path),
            "assetManifestPath": str(manifest_path),
            "templateCount": len(all_rows),
            "menuTemplateCount": len(menu_rows),
            "ecommerceTemplateCount": len(ecommerce_rows),
            "assetManifestItemCount": len(asset_manifest.get("items", [])),
            "missingAssetCount": len(asset_manifest.get("missingAssets", [])),
        },
    )

    print(
        json.dumps(
            {
                "csvPath": str(csv_path),
                "assetManifestPath": str(manifest_path),
                "summaryPath": str(summary_path),
                "templateCount": len(all_rows),
                "assetManifestItemCount": len(asset_manifest.get("items", [])),
                "missingAssetCount": len(asset_manifest.get("missingAssets", [])),
            },
            ensure_ascii=False,
            indent=2,
        )
    )


if __name__ == "__main__":
    main()
