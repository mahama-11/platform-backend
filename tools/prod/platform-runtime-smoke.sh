#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=tools/prod/lib/common.sh
source "$SCRIPT_DIR/lib/common.sh"

DRY_RUN=0
MODE=auto
EXPLICIT_PROVIDER=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --env) shift; [ "${1:-}" = "prod" ] || fail "unsupported_env env=${1:-}" ;;
    --dry-run) DRY_RUN=1 ;;
    --auto-route) MODE=auto; EXPLICIT_PROVIDER="" ;;
    --explicit-provider) shift; MODE=explicit; EXPLICIT_PROVIDER="${1:-}"; [ -n "$EXPLICIT_PROVIDER" ] || fail "missing_provider" ;;
    --help|-h)
      cat <<'USAGE'
Usage: tools/prod/platform-runtime-smoke.sh [--env prod] [--dry-run] [--auto-route|--explicit-provider PROVIDER]
Creates a small text_reasoning runtime job and polls until terminal. Prints no secrets.
USAGE
      exit 0
      ;;
    *) fail "unknown_arg arg=$1" ;;
  esac
  shift || true
done

load_topology
if [ "$DRY_RUN" = "1" ]; then
  log "DRY_RUN platform-runtime-smoke env=${ENV_NAME:-prod} remote=$REMOTE mode=$MODE provider=${EXPLICIT_PROVIDER:-auto}"
  log "Would create regression_smoke text_reasoning job and poll for completed output manifest."
  exit 0
fi

EXPECTED_PROVIDER="${EXPLICIT_PROVIDER:-${EXPECTED_TEXT_PROVIDER:-}}"
remote_cmd "SMOKE_PROVIDER=$(printf '%q' "$EXPLICIT_PROVIDER") SMOKE_MODE=$(printf '%q' "$MODE") SMOKE_EXPECTED_PROVIDER=$(printf '%q' "$EXPECTED_PROVIDER") PLATFORM_BASE_URL=$(printf '%q' "${PLATFORM_LOCAL_URL:-http://127.0.0.1:${PLATFORM_HOST_PORT:-8095}}") PLATFORM_CONFIG_PATH=$(printf '%q' "${PLATFORM_CONFIG_PATH:-$REMOTE_DIR/config.prod.yaml}") bash -s" <<'REMOTE' | redact_stream
set -euo pipefail
BASE="${PLATFORM_BASE_URL:-http://127.0.0.1:8095}"
MODE="${SMOKE_MODE:-auto}"
PROVIDER="${SMOKE_PROVIDER:-}"
command -v jq >/dev/null 2>&1 || { echo "FAIL missing_remote_command command=jq"; exit 1; }
SECRET=$(python3 - <<'PY'
import yaml
import os
cfg=yaml.safe_load(open(os.environ.get('PLATFORM_CONFIG_PATH','/root/gk/platform-backend/config.prod.yaml')))
print((cfg.get('security') or {}).get('internal_service_secret') or '')
PY
)
if [ -z "$SECRET" ]; then echo "FAIL internal_secret_missing"; exit 1; fi
JOB_SOURCE="prod-runtime-smoke-${MODE}-$(date +%s)"
if [ -n "$PROVIDER" ]; then
  PROVIDER_JSON=$(jq -nc --arg provider "$PROVIDER" '$provider')
else
  PROVIDER_JSON='null'
fi
PAYLOAD=$(jq -nc --arg src "$JOB_SOURCE" --argjson provider "$PROVIDER_JSON" '
  {
    product_code:"ecommerce",
    task_type:"text_reasoning",
    provider_mode:"sync",
    organization_id:"org_prod_smoke",
    user_id:"user_prod_smoke",
    source_type:"regression_smoke",
    source_id:$src,
    idempotency_key:$src,
    input_manifest:( {
      input_mode:"prompt_snapshot",
      prompt_snapshot:{system_prompt:"Return compact JSON only.", user_prompt:"Return {\"ok\":true,\"task\":\"prod_smoke\"}"},
      params_snapshot:{prompt:"Return {\"ok\":true,\"task\":\"prod_smoke\"}"}
    } | tostring ),
    timeout_seconds:120,
    max_attempts:1
  } + (if $provider == null or $provider == "" then {} else {provider_code:$provider} end)
')
RESP=$(curl -fsS -H "X-Internal-Service-Secret: $SECRET" -H 'Content-Type: application/json' -d "$PAYLOAD" "$BASE/internal/v1/runtime/jobs")
JOB_ID=$(printf '%s' "$RESP" | jq -r '.data.id // .data.job.id // .id // empty')
if [ -z "$JOB_ID" ]; then echo "FAIL create_job_missing_id response_shape=unexpected"; exit 1; fi
for i in $(seq 1 60); do
  DETAIL=$(curl -fsS -H "X-Internal-Service-Secret: $SECRET" "$BASE/internal/v1/runtime/jobs/$JOB_ID")
  STATUS=$(printf '%s' "$DETAIL" | jq -r '.data.job.status // .data.status // empty')
  STAGE=$(printf '%s' "$DETAIL" | jq -r '.data.job.stage // .data.stage // empty')
  RESOLVED_PROVIDER=$(printf '%s' "$DETAIL" | jq -r '.data.job.provider_code // .data.provider_code // empty')
  ERR_CLASS=$(printf '%s' "$DETAIL" | jq -r '.data.job.error_class // .data.error_class // empty')
  ERR_CODE=$(printf '%s' "$DETAIL" | jq -r '.data.job.error_code // .data.error_code // empty')
  MANIFEST_LEN=$(printf '%s' "$DETAIL" | jq -r '((.data.job.output_manifest // .data.output_manifest // "") | length)')
  if [ "$STATUS" = completed ]; then
    if [ -n "${SMOKE_EXPECTED_PROVIDER:-}" ] && [ "$RESOLVED_PROVIDER" != "$SMOKE_EXPECTED_PROVIDER" ]; then
      echo "FAIL runtime_smoke_provider_mismatch mode=$MODE job_id=$JOB_ID expected_provider=$SMOKE_EXPECTED_PROVIDER actual_provider=$RESOLVED_PROVIDER"
      exit 1
    fi
    if [ "${MANIFEST_LEN:-0}" = "0" ]; then
      echo "FAIL runtime_smoke_empty_output_manifest mode=$MODE job_id=$JOB_ID provider=$RESOLVED_PROVIDER"
      exit 1
    fi
    echo "PASS runtime_smoke mode=$MODE job_id=$JOB_ID status=$STATUS stage=$STAGE provider=$RESOLVED_PROVIDER output_manifest_len=$MANIFEST_LEN"
    exit 0
  fi
  if [ "$STATUS" = failed ] || [ "$STATUS" = canceled ]; then
    echo "FAIL runtime_smoke mode=$MODE job_id=$JOB_ID status=$STATUS stage=$STAGE provider=$RESOLVED_PROVIDER error_class=$ERR_CLASS error_code=$ERR_CODE"
    exit 1
  fi
  sleep 2
done
echo "FAIL runtime_smoke_timeout mode=$MODE job_id=$JOB_ID last_status=$STATUS stage=$STAGE provider=$RESOLVED_PROVIDER"
exit 1
REMOTE
exit ${PIPESTATUS[0]}
