#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=tools/prod/lib/common.sh
source "$SCRIPT_DIR/lib/common.sh"

DRY_RUN=0
FAIL_ON_CRITICAL=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --env) shift; [ "${1:-}" = "prod" ] || fail "unsupported_env env=${1:-}" ;;
    --dry-run) DRY_RUN=1 ;;
    --fail-on-critical) FAIL_ON_CRITICAL=1 ;;
    --help|-h)
      cat <<'USAGE'
Usage: tools/prod/platform-drift-check.sh [--env prod] [--dry-run] [--fail-on-critical]
Read-only prod topology/config/DB drift check. Prints no secrets.
USAGE
      exit 0
      ;;
    *) fail "unknown_arg arg=$1" ;;
  esac
  shift || true
done

load_topology
if [ "$DRY_RUN" = "1" ]; then
  log "DRY_RUN platform-drift-check env=${ENV_NAME:-prod} remote=$REMOTE remote_dir=$REMOTE_DIR"
  log "Would check: topology, config flags, DB endpoints, secret booleans/hash match, provider bindings, storage bindings."
  exit 0
fi

set +e
remote_cmd "PLATFORM_CONTAINER=$(printf '%q' "${PLATFORM_CONTAINER:-v-platform-backend}") ECOMMERCE_CONTAINER=$(printf '%q' "${ECOMMERCE_CONTAINER:-v-ecommerce-backend}") PLATFORM_CONFIG_PATH=$(printf '%q' "${PLATFORM_CONFIG_PATH:-$REMOTE_DIR/config.prod.yaml}") ECOMMERCE_CONFIG_PATH=$(printf '%q' "${ECOMMERCE_CONFIG_PATH:-/root/gk/ecommerce-backend/config.prod.yaml}") ECOMMERCE_INTERNAL_URL=$(printf '%q' "${ECOMMERCE_INTERNAL_URL:-http://v-ecommerce-backend:8296}") MENU_INTERNAL_URL=$(printf '%q' "${MENU_INTERNAL_URL:-http://v-menu-backend:8096}") DEV_ECOMMERCE_HOST_PORT=$(printf '%q' "${DEV_ECOMMERCE_HOST_PORT:-8396}") DEV_PLATFORM_HOST_PORT=$(printf '%q' "${DEV_PLATFORM_HOST_PORT:-8195}") DB_CONTAINER=$(printf '%q' "${DB_CONTAINER:-kong-database}") DB_USER=$(printf '%q' "${DB_USER:-kong}") DB_NAME=$(printf '%q' "${DB_NAME:-kong}") EXPECTED_TEXT_PROVIDER=$(printf '%q' "${EXPECTED_TEXT_PROVIDER:-kimi_coding_text}") EXPECTED_TEXT_TASKS=$(printf '%q' "${EXPECTED_TEXT_TASKS:-text_reasoning intent_planning prompt_planning strategy_report}") EXPECTED_STORAGE_BINDINGS=$(printf '%q' "${EXPECTED_STORAGE_BINDINGS:-menu:studio-assets ecommerce:ecommerce-assets ecommerce:template-examples}") EXPECTED_RUNTIME_QUEUE_NAME=$(printf '%q' "${PLATFORM_RUNTIME_QUEUE_NAME:-runtime:prod}") bash -s" <<'REMOTE' | redact_stream
set -euo pipefail
critical=0
warns=0
pass() { echo "PASS $*"; }
warn() { echo "WARN $*"; warns=$((warns+1)); }
crit() { echo "CRITICAL $*"; critical=$((critical+1)); }

platform_cfg="${PLATFORM_CONFIG_PATH:-/root/gk/platform-backend/config.prod.yaml}"
ecommerce_cfg="${ECOMMERCE_CONFIG_PATH:-/root/gk/ecommerce-backend/config.prod.yaml}"
[ -f "$platform_cfg" ] || crit "platform_config_missing path=$platform_cfg"
[ -f "$ecommerce_cfg" ] || warn "ecommerce_config_missing path=$ecommerce_cfg"

for name in "${PLATFORM_CONTAINER:-v-platform-backend}" "${ECOMMERCE_CONTAINER:-v-ecommerce-backend}"; do
  if docker inspect "$name" >/dev/null 2>&1; then
    image=$(docker inspect -f '{{.Config.Image}}' "$name" 2>/dev/null || true)
    health=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$name" 2>/dev/null || true)
    if [ "$health" = healthy ]; then
      pass "container name=$name image=$image health=$health"
    else
      crit "container_unhealthy name=$name image=$image health=$health"
    fi
  else
    crit "container_missing name=$name"
  fi
done

python3 - <<'PY' > /tmp/platform_drift_env.sh
import yaml, shlex, hashlib
pcfg=yaml.safe_load(open(__import__('os').environ.get('PLATFORM_CONFIG_PATH','/root/gk/platform-backend/config.prod.yaml')))
print('BOOTSTRAP_SYNC=%s' % shlex.quote(str((pcfg.get('bootstrap') or {}).get('sync_enabled'))))
print('AUTO_MIGRATE=%s' % shlex.quote(str((pcfg.get('database') or {}).get('auto_migrate_enabled'))))
for name,path in [('KIMI','kimi_coding'),('MINIMAX','minimax'),('VOLCENGINE','volcengine'),('COMFY','comfyui_bridge')]:
    v=pcfg.get(path) or {}
    print('%s_CONFIG_PRESENT=%s' % (name, bool(v.get('base_url') or v.get('enabled'))))
    print('%s_KEY_PRESENT=%s' % (name, bool(v.get('api_key'))))
try:
    ecfg=yaml.safe_load(open(__import__('os').environ.get('ECOMMERCE_CONFIG_PATH','/root/gk/ecommerce-backend/config.prod.yaml')))
except Exception:
    ecfg={}
platform_internal=((pcfg.get('security') or {}).get('internal_service_secret') or '')
ecom_callback_secret=((ecfg.get('security') or {}).get('service_secret_key') or '')
ecom_platform_secret=((ecfg.get('platform') or {}).get('internal_service_secret') or '')
print('PLATFORM_INTERNAL_HASH=%s' % (hashlib.sha256(platform_internal.encode()).hexdigest()[:12] if platform_internal else ''))
print('PLATFORM_INTERNAL_PLACEHOLDER=%s' % bool(__import__('re').search(r'(change-me|changeme|placeholder|example-secret|your-secret)', platform_internal or '', __import__('re').I)))
runtime_cfg=pcfg.get('runtime') or {}
print('ACTUAL_RUNTIME_QUEUE_NAME=%s' % shlex.quote(runtime_cfg.get('queue_name') or ''))
print('ECOM_SECRET_EMPTY=%s' % (not bool(ecom_callback_secret)))
print('ECOM_SECRET_PLACEHOLDER=%s' % bool(__import__('re').search(r'(change-me|changeme|placeholder|example-secret|your-secret)', ecom_callback_secret or '', __import__('re').I)))
print('ECOM_SECRET_HASH=%s' % (hashlib.sha256(ecom_callback_secret.encode()).hexdigest()[:12] if ecom_callback_secret else ''))
print('ECOM_PLATFORM_SECRET_HASH=%s' % (hashlib.sha256(ecom_platform_secret.encode()).hexdigest()[:12] if ecom_platform_secret else ''))
PY
# shellcheck disable=SC1091
source /tmp/platform_drift_env.sh
[ "$BOOTSTRAP_SYNC" = "True" ] && pass "config bootstrap.sync_enabled=true" || crit "config bootstrap.sync_enabled=$BOOTSTRAP_SYNC expected=True"
[ "$AUTO_MIGRATE" = "False" ] && pass "config auto_migrate_enabled=false" || crit "config auto_migrate_enabled=$AUTO_MIGRATE expected=False"
[ "$KIMI_CONFIG_PRESENT" = "True" ] && [ "$KIMI_KEY_PRESENT" = "True" ] && pass "provider_config kimi=configured" || crit "provider_config kimi_missing_or_key_absent"
[ "$MINIMAX_CONFIG_PRESENT" = "True" ] && [ "$MINIMAX_KEY_PRESENT" = "True" ] && pass "provider_config minimax=configured" || warn "provider_config minimax_missing_or_key_absent"
expected_queue="${EXPECTED_RUNTIME_QUEUE_NAME:-runtime:prod}"
actual_queue="${ACTUAL_RUNTIME_QUEUE_NAME:-}"
if [ -n "${expected_queue:-}" ]; then
  [ "$actual_queue" = "$expected_queue" ] && pass "platform runtime_queue_name=$actual_queue" || crit "platform runtime_queue_mismatch expected=$expected_queue actual=${actual_queue:-default} risk=prod_dev_worker_collision"
else
  [ -n "$actual_queue" ] && pass "platform runtime_queue_name=$actual_queue" || crit "platform runtime_queue_default=true risk=prod_dev_worker_collision"
fi

cat >/tmp/platform_drift.sql <<'SQL'
select 'endpoint|' || product_code || '|' || base_url || '|' || status || '|' || (length(coalesce(secret,$q$$q$))=0)::text || '|' || (left(coalesce(secret,$q$$q$),9)=$q$change-me$q$)::text || '|' || left(encode(sha256(coalesce(secret,$q$$q$)::bytea),'hex'),12) from runtime_product_endpoints where product_code in ($q$menu$q$,$q$ecommerce$q$) order by product_code;
select 'binding|' || product_code || '|' || task_type || '|' || provider_code || '|' || priority::text || '|' || enabled::text from runtime_provider_bindings where product_code=$q$ecommerce$q$ and task_type in ($q$text_reasoning$q$,$q$intent_planning$q$,$q$prompt_planning$q$,$q$strategy_report$q$) order by task_type, priority desc, provider_code;
select 'definition|' || code || '|' || status from runtime_provider_definitions where code in ($q$kimi_coding_text$q$,$q$minimax_text$q$,$q$volcengine$q$,$q$comfyui_bridge$q$) order by code;
select 'storage|' || product_code || '|' || category || '|' || provider_code || '|' || enabled::text from storage_bindings where (product_code=$q$menu$q$ and category=$q$studio-assets$q$) or (product_code=$q$ecommerce$q$ and category in ($q$ecommerce-assets$q$,$q$template-examples$q$)) order by product_code, category;
SQL
DB_CONTAINER="${DB_CONTAINER:-kong-database}"
DB_USER="${DB_USER:-kong}"
DB_NAME="${DB_NAME:-kong}"
if ! docker exec "$DB_CONTAINER" sh -lc 'command -v psql >/dev/null 2>&1'; then
  crit "db_psql_missing container=$DB_CONTAINER"
  rows=""
else
  rows=$(docker exec -i "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -At < /tmp/platform_drift.sql)
fi
rm -f /tmp/platform_drift.sql /tmp/platform_drift_env.sh

while IFS='|' read -r kind a b c d e f; do
  [ -n "${kind:-}" ] || continue
  case "$kind" in
    endpoint)
      product=$a; base=$b; status=$c; secret_empty=$d; secret_placeholder=$e; hash=$f
      expected_url=""
      [ "$product" = "ecommerce" ] && expected_url="${ECOMMERCE_INTERNAL_URL:-http://v-ecommerce-backend:8296}"
      [ "$product" = "menu" ] && expected_url="${MENU_INTERNAL_URL:-http://v-menu-backend:8096}"
      if [ -n "$expected_url" ] && [ "$base" != "$expected_url" ]; then
        crit "endpoint product=$product base_url_mismatch actual=$base expected=$expected_url"
      elif echo "$base" | grep -Eq -- "-dev|:${DEV_ECOMMERCE_HOST_PORT:-8396}(/|$)|:${DEV_PLATFORM_HOST_PORT:-8195}(/|$)"; then
        crit "endpoint product=$product points_to_dev base_url=$base"
      else
        pass "endpoint product=$product base_url=$base"
      fi
      [ "$status" = "active" ] && pass "endpoint product=$product active" || crit "endpoint product=$product status=$status"
      [ "$secret_empty" = "false" ] && pass "endpoint product=$product secret_non_empty=true" || crit "endpoint product=$product secret_empty=true"
      [ "$secret_placeholder" = "false" ] && pass "endpoint product=$product endpoint_secret_placeholder=false" || crit "endpoint product=$product endpoint_secret_placeholder=true"
      if [ "$product" = "ecommerce" ]; then
        [ "${ECOM_SECRET_EMPTY:-True}" = "False" ] && pass "ecommerce callback_secret_non_empty=true" || crit "ecommerce callback_secret_empty=true"
        [ "${ECOM_SECRET_PLACEHOLDER:-True}" = "False" ] && pass "ecommerce callback_secret_placeholder=false" || crit "ecommerce callback_secret_placeholder=true"
        [ "${ECOM_PLATFORM_SECRET_HASH:-}" = "${PLATFORM_INTERNAL_HASH:-}" ] && pass "ecommerce outbound_platform_secret_matches_platform=true hash_prefix=${ECOM_PLATFORM_SECRET_HASH:-}" || crit "ecommerce outbound_platform_secret_matches_platform=false ecommerce_hash=${ECOM_PLATFORM_SECRET_HASH:-} platform_hash=${PLATFORM_INTERNAL_HASH:-}"
        [ "${PLATFORM_INTERNAL_PLACEHOLDER:-True}" = "False" ] && pass "platform internal_secret_placeholder=false" || warn "platform internal_secret_placeholder=true follow_up=rotate_platform_internal_secret"
        if [ "${ECOM_SECRET_EMPTY:-True}" = "False" ]; then
          [ "$hash" = "${ECOM_SECRET_HASH:-}" ] && pass "endpoint product=ecommerce callback_secret_hash_matches_backend=true hash_prefix=$hash" || crit "endpoint product=ecommerce callback_secret_hash_matches_backend=false endpoint_hash=$hash backend_hash=${ECOM_SECRET_HASH:-}"
        fi
      fi
      ;;
    binding)
      pass "provider_binding product=$a task=$b provider=$c priority=$d enabled=$e"
      ;;
    definition)
      [ "$b" = "active" ] && pass "provider_definition code=$a status=$b" || warn "provider_definition code=$a status=$b"
      ;;
    storage)
      [ "$d" = "true" ] && pass "storage_binding product=$a category=$b provider=$c enabled=true" || crit "storage_binding product=$a category=$b enabled=$d"
      ;;
  esac
done <<< "$rows"

for task in ${EXPECTED_TEXT_TASKS:-text_reasoning intent_planning prompt_planning strategy_report}; do
  expected="binding|ecommerce|$task|${EXPECTED_TEXT_PROVIDER:-kimi_coding_text}|120|true"
  if ! echo "$rows" | grep -q "$expected"; then crit "provider_binding_missing expected=$expected"; fi
done
for item in ${EXPECTED_STORAGE_BINDINGS:-menu:studio-assets ecommerce:ecommerce-assets ecommerce:template-examples}; do
  product="${item%%:*}"
  category="${item#*:}"
  if ! echo "$rows" | grep -q "storage|$product|$category|"; then crit "storage_binding_missing expected=$item"; fi
done

echo "SUMMARY critical=$critical warnings=$warns"
exit "$critical"
REMOTE
status=${PIPESTATUS[0]}
set -e
if [ "$status" -ne 0 ] && [ "$FAIL_ON_CRITICAL" = "1" ]; then
  exit "$status"
fi
exit 0
