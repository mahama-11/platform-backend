#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=tools/prod/lib/common.sh
source "$SCRIPT_DIR/lib/common.sh"

DRY_RUN=0
APPLY=0
RESTART_ECOMMERCE=0
SECRET_MODE="rotate"
while [ "$#" -gt 0 ]; do
  case "$1" in
    --env) shift; [ "${1:-}" = "prod" ] || fail "unsupported_env env=${1:-}" ;;
    --dry-run) DRY_RUN=1 ;;
    --apply) APPLY=1 ;;
    --restart-ecommerce) RESTART_ECOMMERCE=1 ;;
    --mode) shift; SECRET_MODE="${1:-}"; case "$SECRET_MODE" in rotate|sync-existing) ;; *) fail "unsupported_mode mode=$SECRET_MODE" ;; esac ;;
    --help|-h)
      cat <<'USAGE'
Usage: tools/prod/platform-callback-secret-sync.sh [--env prod] [--dry-run] [--apply] [--restart-ecommerce] [--mode rotate|sync-existing]

Safely aligns the Platform runtime_product_endpoints.ecommerce callback secret with
Ecommerce backend security.service_secret_key, while preserving Ecommerce outbound
platform.internal_service_secret alignment with Platform. Prints only booleans/hash prefixes.

Modes:
  rotate        Generate a new high-entropy Ecommerce inbound callback secret remotely,
                write it to Ecommerce security.service_secret_key, restore Ecommerce
                platform.internal_service_secret from Platform config, then update
                Platform runtime_product_endpoints.ecommerce.secret.
  sync-existing Keep Ecommerce security.service_secret_key unchanged and copy it into
                Platform runtime_product_endpoints.ecommerce.secret, while still restoring
                Ecommerce outbound platform.internal_service_secret from Platform config.

Safety:
  - dry-run never reads or prints the raw secret;
  - --apply is required for mutation;
  - config backup is created before editing;
  - Ecommerce restart is opt-in with --restart-ecommerce;
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
  log "DRY_RUN platform-callback-secret-sync env=${ENV_NAME:-prod} remote=$REMOTE mode=$SECRET_MODE apply=$APPLY restart_ecommerce=$RESTART_ECOMMERCE"
  log "Would compare Ecommerce config secret hash with Platform runtime endpoint hash, then align on --apply."
  exit 0
fi
if [ "$APPLY" != "1" ]; then
  log "PLAN platform-callback-secret-sync env=${ENV_NAME:-prod} remote=$REMOTE mode=$SECRET_MODE"
  log "No mutation performed. Re-run with --apply to update remote config/DB."
fi

set +e
remote_cmd "APPLY=$(printf '%q' "$APPLY") RESTART_ECOMMERCE=$(printf '%q' "$RESTART_ECOMMERCE") SECRET_MODE=$(printf '%q' "$SECRET_MODE") PLATFORM_CONTAINER=$(printf '%q' "${PLATFORM_CONTAINER:-v-platform-backend}") ECOMMERCE_CONTAINER=$(printf '%q' "${ECOMMERCE_CONTAINER:-v-ecommerce-backend}") ECOMMERCE_SERVICE=$(printf '%q' "${ECOMMERCE_SERVICE:-prod-backend}") ECOMMERCE_REMOTE_DIR=$(printf '%q' "${ECOMMERCE_REMOTE_DIR:-/root/gk/ecommerce-backend}") PLATFORM_CONFIG_PATH=$(printf '%q' "${PLATFORM_CONFIG_PATH:-$REMOTE_DIR/config.prod.yaml}") ECOMMERCE_CONFIG_PATH=$(printf '%q' "${ECOMMERCE_CONFIG_PATH:-/root/gk/ecommerce-backend/config.prod.yaml}") DB_CONTAINER=$(printf '%q' "${DB_CONTAINER:-kong-database}") DB_USER=$(printf '%q' "${DB_USER:-kong}") DB_NAME=$(printf '%q' "${DB_NAME:-kong}") bash -s" <<'REMOTE' | redact_stream
set -euo pipefail
APPLY="${APPLY:-0}"
RESTART_ECOMMERCE="${RESTART_ECOMMERCE:-0}"
SECRET_MODE="${SECRET_MODE:-rotate}"
ecommerce_cfg="${ECOMMERCE_CONFIG_PATH:-/root/gk/ecommerce-backend/config.prod.yaml}"
db_container="${DB_CONTAINER:-kong-database}"
db_user="${DB_USER:-kong}"
db_name="${DB_NAME:-kong}"

command -v python3 >/dev/null 2>&1 || { echo "FAIL missing_remote_command command=python3"; exit 1; }
[ -f "$ecommerce_cfg" ] || { echo "FAIL ecommerce_config_missing path=$ecommerce_cfg"; exit 1; }
if ! docker exec "$db_container" sh -lc 'command -v psql >/dev/null 2>&1'; then
  echo "FAIL db_psql_missing container=$db_container"; exit 1
fi

read_hashes() {
  ECOM_HASH=$(python3 - <<'PY'
import os, yaml, hashlib
path=os.environ.get('ECOMMERCE_CONFIG_PATH','/root/gk/ecommerce-backend/config.prod.yaml')
cfg=yaml.safe_load(open(path)) or {}
secret=((cfg.get('security') or {}).get('service_secret_key') or '')
print(hashlib.sha256(secret.encode()).hexdigest()[:12] if secret else '')
PY
)
  ENDPOINT_HASH=$(docker exec -i "$db_container" psql -U "$db_user" -d "$db_name" -At <<'SQL'
select left(encode(sha256(coalesce(secret,'')::bytea),'hex'),12) from runtime_product_endpoints where product_code='ecommerce' limit 1;
SQL
)
  ECOM_EMPTY=$(python3 - <<'PY'
import os, yaml
path=os.environ.get('ECOMMERCE_CONFIG_PATH','/root/gk/ecommerce-backend/config.prod.yaml')
cfg=yaml.safe_load(open(path)) or {}
secret=((cfg.get('security') or {}).get('service_secret_key') or '')
print('true' if not secret else 'false')
PY
)
  ECOM_PLACEHOLDER=$(python3 - <<'PY'
import os, yaml, re
path=os.environ.get('ECOMMERCE_CONFIG_PATH','/root/gk/ecommerce-backend/config.prod.yaml')
cfg=yaml.safe_load(open(path)) or {}
secret=((cfg.get('security') or {}).get('service_secret_key') or '')
print('true' if re.search(r'(change-me|changeme|placeholder|example-secret|your-secret)', secret or '', re.I) else 'false')
PY
)
}
read_hashes
echo "BEFORE ecommerce_secret_empty=$ECOM_EMPTY ecommerce_secret_placeholder=$ECOM_PLACEHOLDER ecommerce_hash_prefix=$ECOM_HASH endpoint_hash_prefix=$ENDPOINT_HASH"

if [ "$APPLY" != "1" ]; then
  if [ "$ECOM_HASH" = "$ENDPOINT_HASH" ] && [ "$ECOM_EMPTY" = false ] && [ "$ECOM_PLACEHOLDER" = false ]; then
    echo "PASS callback_secret_already_aligned=true hash_prefix=$ECOM_HASH"
  else
    echo "NEEDS_APPLY callback_secret_aligned=false mode=$SECRET_MODE"
  fi
  exit 0
fi

backup_dir="$(dirname "$ecommerce_cfg")/backups/callback-secret"
mkdir -p "$backup_dir"
backup_file="$backup_dir/config.prod.yaml.$(date +%Y%m%d-%H%M%S).bak"
cp "$ecommerce_cfg" "$backup_file"
chmod 600 "$backup_file" || true

PLATFORM_OUTBOUND_SECRET=$(python3 - <<'PY'
import os, yaml
path=os.environ.get('PLATFORM_CONFIG_PATH','/root/gk/platform-backend/config.prod.yaml')
cfg=yaml.safe_load(open(path)) or {}
print(((cfg.get('security') or {}).get('internal_service_secret') or ''))
PY
)
if [ -z "$PLATFORM_OUTBOUND_SECRET" ]; then echo "FAIL platform_internal_secret_empty"; exit 1; fi
export PLATFORM_OUTBOUND_SECRET
if python3 - <<'PY'
import os, re, sys
secret=os.environ.get('PLATFORM_OUTBOUND_SECRET','')
sys.exit(0 if re.search(r'(change-me|changeme|placeholder|example-secret|your-secret)', secret or '', re.I) else 1)
PY
then
  echo "WARN platform_internal_secret_placeholder=true follow_up=rotate_platform_internal_secret"
fi

if [ "$SECRET_MODE" = rotate ]; then
  NEW_SECRET=$(python3 - <<'PY'
import secrets
print(secrets.token_urlsafe(48))
PY
)
  TARGET_SECRET="$NEW_SECRET"
else
  TARGET_SECRET=$(python3 - <<'PY'
import os, yaml
path=os.environ.get('ECOMMERCE_CONFIG_PATH','/root/gk/ecommerce-backend/config.prod.yaml')
cfg=yaml.safe_load(open(path)) or {}
print(((cfg.get('security') or {}).get('service_secret_key') or ''))
PY
)
  if [ -z "$TARGET_SECRET" ]; then echo "FAIL cannot_sync_empty_ecommerce_secret"; exit 1; fi
fi
export TARGET_SECRET
python3 - <<'PY'
import os, yaml
path=os.environ.get('ECOMMERCE_CONFIG_PATH','/root/gk/ecommerce-backend/config.prod.yaml')
callback_secret=os.environ['TARGET_SECRET']
outbound_secret=os.environ['PLATFORM_OUTBOUND_SECRET']
with open(path) as f:
    cfg=yaml.safe_load(f) or {}
cfg.setdefault('platform', {})['internal_service_secret']=outbound_secret
cfg.setdefault('security', {})['service_secret_key']=callback_secret
tmp=path + '.tmp'
with open(tmp, 'w') as f:
    yaml.safe_dump(cfg, f, sort_keys=False, allow_unicode=True)
os.replace(tmp, path)
PY

export TARGET_SECRET
# Generate SQL locally inside the remote shell without printing the secret. The SQL is piped
# directly into psql and never echoed to agent output.
python3 - <<'PY' | docker exec -i "$db_container" psql -U "$db_user" -d "$db_name" -v ON_ERROR_STOP=1 >/dev/null
import os
secret=os.environ.get('TARGET_SECRET','')
if not secret:
    raise SystemExit('empty target secret')
escaped=secret.replace("'", "''")
print("update runtime_product_endpoints set secret='%s', updated_at=now() where product_code='ecommerce';" % escaped)
PY
unset NEW_SECRET TARGET_SECRET

if [ "$RESTART_ECOMMERCE" = "1" ]; then
  # Restart the existing prod container without rebuilding/pulling. This script only rotates
  # a mounted config secret; image deployment remains owned by ecommerce deploy flow.
  docker restart "${ECOMMERCE_CONTAINER:-v-ecommerce-backend}" >/dev/null
  for i in $(seq 1 60); do
    health=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "${ECOMMERCE_CONTAINER:-v-ecommerce-backend}" 2>/dev/null || echo missing)
    [ "$health" = healthy ] && echo "PASS ecommerce_restarted health=healthy" && break
    sleep 2
    if [ "$i" = 60 ]; then echo "FAIL ecommerce_restart_health_timeout health=$health"; exit 1; fi
  done
else
  echo "WARN ecommerce_restart_skipped config_change_requires_restart=true"
fi

read_hashes
echo "AFTER ecommerce_secret_empty=$ECOM_EMPTY ecommerce_secret_placeholder=$ECOM_PLACEHOLDER ecommerce_hash_prefix=$ECOM_HASH endpoint_hash_prefix=$ENDPOINT_HASH backup_created=true"
if [ "$ECOM_EMPTY" != false ] || [ "$ECOM_PLACEHOLDER" != false ] || [ "$ECOM_HASH" != "$ENDPOINT_HASH" ]; then
  echo "FAIL callback_secret_aligned=false"
  exit 1
fi
echo "PASS callback_secret_aligned=true hash_prefix=$ECOM_HASH"
REMOTE
status=${PIPESTATUS[0]}
set -e
exit "$status"
