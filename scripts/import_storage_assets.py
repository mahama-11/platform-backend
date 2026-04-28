#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any
from urllib import error, request


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Import local files into platform asset storage.")
    parser.add_argument("--manifest", required=True, help="Path to import manifest JSON")
    parser.add_argument("--base-url", required=True, help="Platform base URL, e.g. http://127.0.0.1:8195")
    parser.add_argument("--secret", required=True, help="X-Internal-Service-Secret value")
    parser.add_argument(
        "--source-root",
        default="",
        help="Optional root directory for resolving relative sourcePath values",
    )
    parser.add_argument(
        "--output",
        default="",
        help="Optional output file for resolved manifest JSON; defaults to stdout only",
    )
    return parser.parse_args()


def resolve_source_path(source_path: str, source_root: Path | None) -> str:
    path = Path(source_path)
    if path.is_absolute() or source_root is None:
        return str(path)
    return str((source_root / path).resolve())


def post_json(url: str, secret: str, payload: dict[str, Any]) -> dict[str, Any]:
    req = request.Request(
        url,
        data=json.dumps(payload).encode("utf-8"),
        headers={
            "Content-Type": "application/json",
            "X-Internal-Service-Secret": secret,
        },
        method="POST",
    )
    try:
        with request.urlopen(req) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        raise SystemExit(f"HTTP {exc.code} {exc.reason}: {body}") from exc


def main() -> None:
    args = parse_args()
    manifest_path = Path(args.manifest).resolve()
    source_root = Path(args.source_root).resolve() if args.source_root else None
    manifest = json.loads(manifest_path.read_text())
    items = manifest.get("items", [])
    if not isinstance(items, list):
        raise SystemExit("manifest.items must be a list")

    resolved_items: list[dict[str, Any]] = []
    endpoint = args.base_url.rstrip("/") + "/internal/v1/storage/assets/import-local"

    for item in items:
        if not isinstance(item, dict):
            raise SystemExit("each manifest item must be an object")
        source_path = item.get("sourcePath") or item.get("assetRef")
        if not isinstance(source_path, str) or not source_path.strip():
            raise SystemExit("manifest item missing sourcePath/assetRef")

        payload = {
            "product_code": item["productCode"],
            "category": item["category"],
            "source_type": item.get("sourceType", "template_example"),
            "source_ref": item["sourceRef"],
            "source_path": resolve_source_path(source_path, source_root),
            "storage_file_name": item.get("storageFileName", ""),
            "mime_type": item.get("mimeType", ""),
            "title": item.get("title", ""),
            "description": item.get("description", ""),
            "tags": item.get("tags", []),
            "metadata": item.get("metadata", {}),
        }
        response = post_json(endpoint, args.secret, payload)
        resolved_items.append({
            **item,
            "sourcePath": payload["source_path"],
            "importResult": response.get("data", response),
        })

    output = {
        "version": manifest.get("version", 1),
        "generatedBy": "import_storage_assets.py",
        "items": resolved_items,
    }
    if args.output:
        Path(args.output).write_text(json.dumps(output, ensure_ascii=False, indent=2))
    print(json.dumps(output, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)
