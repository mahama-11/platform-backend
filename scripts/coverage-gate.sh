#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

OUT_DIR=${COVERAGE_OUT_DIR:-"$ROOT_DIR/reports/coverage"}
# Defaults come from coverage-baseline.json. Env vars are explicit local overrides.
TOTAL_FLOOR=${TOTAL_COVERAGE_FLOOR:-}
STRICT_TOTAL_TARGET=${STRICT_TOTAL_COVERAGE_TARGET:-}
BASELINE_FILE=${COVERAGE_BASELINE_FILE:-"$ROOT_DIR/scripts/coverage-baseline.json"}

mkdir -p "$OUT_DIR"
PROFILE="$OUT_DIR/coverage.out"
LOG="$OUT_DIR/go-test-cover.log"
FUNC="$OUT_DIR/coverage-func.txt"
REPORT="$OUT_DIR/coverage-gate-report.json"

printf '[coverage-gate] running go test packages-with-tests with coverprofile\n'
mapfile -t TEST_PACKAGES < <(go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./... | sed '/^$/d')
if [[ ${#TEST_PACKAGES[@]} -eq 0 ]]; then
  echo '[coverage-gate] no test packages found' >&2
  exit 1
fi
# Keep stderr in the log as Go may print package-level coverage diagnostics there.
go test "${TEST_PACKAGES[@]}" -count=1 -covermode=atomic -coverprofile="$PROFILE" 2>&1 | tee "$LOG"
go tool cover -func="$PROFILE" > "$FUNC"

python3 - "$LOG" "$FUNC" "$BASELINE_FILE" "$REPORT" "$TOTAL_FLOOR" "$STRICT_TOTAL_TARGET" <<'PY'
import json
import re
import sys
from pathlib import Path

log_path, func_path, baseline_path, report_path, total_floor_arg, strict_total_target_arg = sys.argv[1:]

baseline = json.loads(Path(baseline_path).read_text())
total_floor = float(total_floor_arg) if total_floor_arg else float(baseline.get("total_statement_floor", 70.0))
strict_total_target = float(strict_total_target_arg) if strict_total_target_arg else float(baseline.get("strict_total_target", total_floor))
module_floors = {k: float(v) for k, v in baseline.get("module_statement_floors", {}).items()}
package_aliases = baseline.get("package_aliases", {})

package_coverage = {}
for raw in Path(log_path).read_text(errors="ignore").splitlines():
    m = re.match(r"ok\s+\S+/(.*?)\s+.*coverage:\s+([0-9.]+)%", raw.strip())
    if not m:
        continue
    package_coverage[m.group(1)] = float(m.group(2))

total = None
for raw in Path(func_path).read_text(errors="ignore").splitlines():
    if raw.startswith("total:"):
        m = re.search(r"([0-9.]+)%", raw)
        if m:
            total = float(m.group(1))
        break
if total is None:
    raise SystemExit("coverage total not found in go tool cover output")

failures = []
if total < total_floor:
    failures.append({"scope": "total", "actual": total, "floor": total_floor})
module_results = {}
for module, floor in module_floors.items():
    pkg = package_aliases.get(module, module)
    actual = package_coverage.get(pkg)
    module_results[module] = {"package": pkg, "actual": actual, "floor": floor}
    if actual is None:
        failures.append({"scope": module, "actual": None, "floor": floor, "reason": "package coverage missing"})
    elif actual + 1e-9 < floor:
        failures.append({"scope": module, "actual": actual, "floor": floor})

report = {
    "status": "FAIL" if failures else "PASS",
    "total_statement_coverage": total,
    "total_floor": total_floor,
    "strict_total_target": strict_total_target,
    "strict_target_met": total >= strict_total_target,
    "module_results": module_results,
    "failures": failures,
    "note": "Default gate enforces the 80% total statement floor plus verified no-regression floors for core modules. TOTAL_COVERAGE_FLOOR can be set explicitly only for a temporary local override."
}
Path(report_path).write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n")
print(json.dumps(report, indent=2, ensure_ascii=False))
if failures:
    raise SystemExit(1)
PY

printf '[coverage-gate] PASS report=%s\n' "$REPORT"
