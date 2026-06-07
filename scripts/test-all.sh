#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

go test ./...

if [[ "${RUN_COVERAGE_GATE:-0}" == "1" ]]; then
  bash scripts/coverage-gate.sh
fi
