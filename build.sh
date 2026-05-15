#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
cd "$SCRIPT_DIR"
CMD="${1:-}"
IMAGE_NAME="${IMAGE_NAME:-ver/v-platform-backend}"
REMOTE="${REMOTE:-root@159.138.228.40}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/KeyPair-v2.pem}"
REMOTE_DIR="${REMOTE_DIR:-/root/gk/platform-backend}"
TS=$(date +%Y%m%d-%H%M%S)
REMOTE_BASE="${REMOTE_DIR%/*}"

LOCAL_DEV_DIR="artifacts/dev"

remote() { ssh -i "$SSH_KEY" "$REMOTE" "$1"; }
health_wait() {
  NAME="$1"; LIM="$2"
  remote "for i in \$(seq 1 $LIM); do s=\$(docker inspect -f '{{.State.Health.Status}}' $NAME 2>/dev/null || echo none); [ \"\$s\" = healthy ] && echo HEALTHY && exit 0; sleep 2; done; echo HEALTH_CHECK_FAILED; exit 1"
}

send_dev_files() {
  OUT_FILE="$1"
  ssh -i "$SSH_KEY" "$REMOTE" "mkdir -p '$REMOTE_DIR' '$REMOTE_BASE'"
  scp -i "$SSH_KEY" "$OUT_FILE" "$REMOTE:$REMOTE_BASE/"
  scp -i "$SSH_KEY" docker-compose.yml "$REMOTE:$REMOTE_DIR/"
  [ -f config.dev.yaml ] && scp -i "$SSH_KEY" config.dev.yaml "$REMOTE:$REMOTE_DIR/config.dev.yaml"
}

case "$CMD" in
  dev)
    DEV_TAG="${DEV_TAG:-dev}"
    IMG="$IMAGE_NAME:$DEV_TAG"
    mkdir -p "$LOCAL_DEV_DIR"
    OUT="$LOCAL_DEV_DIR/${IMAGE_NAME##*/}_dev_$TS.tar.gz"
    REMOTE_OUT="$REMOTE_BASE/$(basename "$OUT")"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o platform-service-linux ./cmd/server
    docker buildx build --platform linux/amd64 -f Dockerfile.dev -t "$IMG" .
    docker save "$IMG" | gzip > "$OUT"
    send_dev_files "$OUT"
    remote "docker load -i $REMOTE_OUT"
    remote "mkdir -p $REMOTE_BASE/backups/platform-dev; mv -f $REMOTE_OUT $REMOTE_BASE/backups/platform-dev/"
    remote "cd $REMOTE_DIR; DEV_TAG=$DEV_TAG docker compose up -d dev-backend"
    health_wait "v-platform-backend-dev" 60
    ;;
  prod)
    exec "$SCRIPT_DIR/tools/prod/platform-deploy.sh" all --env prod "${@:2}"
    ;;
  prod-build|prod-upload|prod-restart|prod-drift-check|prod-smoke|prod-evidence)
    phase="${CMD#prod-}"
    exec "$SCRIPT_DIR/tools/prod/platform-deploy.sh" "$phase" --env prod "${@:2}"
    ;;
  *)
    echo "Usage: ./build.sh [dev|prod|prod-build|prod-upload|prod-restart|prod-drift-check|prod-smoke|prod-evidence]"
    echo "Prod deploy defaults preserve remote config; use UPLOAD_PROD_CONFIG=1 only after placeholder scan."
    exit 1
    ;;
esac
