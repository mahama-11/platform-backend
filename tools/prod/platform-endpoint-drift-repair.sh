#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=tools/prod/lib/common.sh
source "$SCRIPT_DIR/lib/common.sh"

DRY_RUN=0
APPLY=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --env) shift; [ "${1:-}" = "prod" ] || fail "unsupported_env env=${1:-}" ;;
    --dry-run) DRY_RUN=1 ;;
    --apply) APPLY=1 ;;
    --help|-h)
      cat <<'USAGE'
Usage: tools/prod/platform-endpoint-drift-repair.sh [--env prod] [--dry-run] [--apply]

Repairs read-only-detected prod runtime_product_endpoints base_url drift for
Menu/Ecommerce callback endpoints. Prints no secrets and only updates base_url/status/updated_at.

Safety:
  - dry-run prints current/expected URLs only;
  - --apply is required for mutation;
  - expected URLs come from topology/env defaults and must be Docker-network service URLs;
  - follow with platform-drift-check --fail-on-critical.
USAGE
      exit 0
      ;;
    *) fail "unknown_arg arg=$1" ;;
  esac
  shift || true
done

load_topology
if [ "$DRY_RUN" = "1" ]; then
  log "DRY_RUN platform-endpoint-drift-repair env=${ENV_NAME:-prod} remote=$REMOTE ecommerce=${ECOMMERCE_INTERNAL_URL:-http://v-ecommerce-backend:8296} menu=${MENU_INTERNAL_URL:-http://v-menu-backend:8096}"
  exit 0
fi
if [ "$APPLY" != "1" ]; then
  log "PLAN platform-endpoint-drift-repair env=${ENV_NAME:-prod} remote=$REMOTE"
  log "No mutation performed. Re-run with --apply to update runtime_product_endpoints base_url."
fi

set +e
remote_cmd "APPLY=$(printf '%q' "$APPLY") DB_CONTAINER=$(printf '%q' "${DB_CONTAINER:-kong-database}") DB_USER=$(printf '%q' "${DB_USER:-kong}") DB_NAME=$(printf '%q' "${DB_NAME:-kong}") ECOMMERCE_INTERNAL_URL=$(printf '%q' "${ECOMMERCE_INTERNAL_URL:-http://v-ecommerce-backend:8296}") MENU_INTERNAL_URL=$(printf '%q' "${MENU_INTERNAL_URL:-http://v-menu-backend:8096}") bash -s" <<'REMOTE' | redact_stream
set -euo pipefail
apply="${APPLY:-0}"
db_container="${DB_CONTAINER:-kong-database}"
db_user="${DB_USER:-kong}"
db_name="${DB_NAME:-kong}"
ecommerce_url="${ECOMMERCE_INTERNAL_URL:-http://v-ecommerce-backend:8296}"
menu_url="${MENU_INTERNAL_URL:-http://v-menu-backend:8096}"

if ! docker exec "$db_container" sh -lc 'command -v psql >/dev/null 2>&1'; then
  echo "FAIL db_psql_missing container=$db_container"; exit 1
fi

before=$(docker exec -i "$db_container" psql -U "$db_user" -d "$db_name" -At <<'SQL'
select product_code || '|' || base_url || '|' || status from runtime_product_endpoints where product_code in ('ecommerce','menu') order by product_code;
SQL
)
echo "$before" | while IFS='|' read -r product base status; do
  [ -n "${product:-}" ] || continue
  expected=""
  [ "$product" = ecommerce ] && expected="$ecommerce_url"
  [ "$product" = menu ] && expected="$menu_url"
  if [ "$base" = "$expected" ] && [ "$status" = active ]; then
    echo "PASS before endpoint product=$product base_url=$base status=$status"
  else
    echo "NEEDS_APPLY endpoint product=$product base_url=$base expected=$expected status=$status"
  fi
done

if [ "$apply" != "1" ]; then
  exit 0
fi

ECOMMERCE_SQL_URL="$ecommerce_url" MENU_SQL_URL="$menu_url" python3 - <<'PY' | docker exec -i "$db_container" psql -U "$db_user" -d "$db_name" -v ON_ERROR_STOP=1 >/dev/null
import os
for product, env_name in [('ecommerce', 'ECOMMERCE_SQL_URL'), ('menu', 'MENU_SQL_URL')]:
    url = os.environ[env_name].replace("'", "''")
    print("update runtime_product_endpoints set base_url='%s', status='active', updated_at=now() where product_code='%s';" % (url, product))
PY

after=$(docker exec -i "$db_container" psql -U "$db_user" -d "$db_name" -At <<'SQL'
select product_code || '|' || base_url || '|' || status from runtime_product_endpoints where product_code in ('ecommerce','menu') order by product_code;
SQL
)
echo "$after" | while IFS='|' read -r product base status; do
  [ -n "${product:-}" ] || continue
  expected=""
  [ "$product" = ecommerce ] && expected="$ecommerce_url"
  [ "$product" = menu ] && expected="$menu_url"
  if [ "$base" = "$expected" ] && [ "$status" = active ]; then
    echo "PASS after endpoint product=$product base_url=$base status=$status"
  else
    echo "FAIL after endpoint product=$product base_url=$base expected=$expected status=$status"
    exit 1
  fi
done
REMOTE
status=${PIPESTATUS[0]}
set -e
exit "$status"
