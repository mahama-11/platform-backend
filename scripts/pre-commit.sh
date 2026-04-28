#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
PROJECT_DIR="$ROOT_DIR/v-platform-backend"

echo "======================================"
echo "⚡ Platform Backend Pre-commit"
echo "======================================"

echo "[1/3] Running gofmt on changed files..."
STAGED_GO_FILES=$(git -C "$ROOT_DIR" diff --cached --name-only --diff-filter=d | grep -E '^v-platform-backend/.*\.go$' || true)
if [[ -n "$STAGED_GO_FILES" ]]; then
  while IFS= read -r FILE; do
    [[ -z "$FILE" ]] && continue
    gofmt -w "$ROOT_DIR/$FILE"
    git -C "$ROOT_DIR" add "$FILE"
  done <<< "$STAGED_GO_FILES"
fi
echo "✅ gofmt passed."

echo "[2/3] Running targeted Go tests..."
bash "$PROJECT_DIR/scripts/test-quick.sh"
echo "✅ Go tests passed."

echo "[3/3] Verifying build..."
bash "$PROJECT_DIR/scripts/test-all.sh"
echo "✅ Platform backend pre-commit passed!"
