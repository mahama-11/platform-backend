#!/usr/bin/env bash
set -euo pipefail

script_dir() {
  CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd
}

TOOLS_PROD_DIR="$(CDPATH= cd -- "$(script_dir)/.." && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$TOOLS_PROD_DIR/../.." && pwd)"
TOPOLOGY_FILE_DEFAULT="$REPO_ROOT/ops/topology/prod.env"

load_topology() {
  local file="${TOPOLOGY_FILE:-$TOPOLOGY_FILE_DEFAULT}"
  if [ ! -f "$file" ]; then
    echo "FAIL topology_missing path=$file" >&2
    exit 1
  fi
  # shellcheck disable=SC1090
  source "$file"
  REMOTE="${REMOTE:-${REMOTE_DEFAULT:-root@159.138.228.40}}"
  SSH_KEY="${SSH_KEY:-${SSH_KEY_DEFAULT:-$HOME/.ssh/KeyPair-v2.pem}}"
  REMOTE_DIR="${REMOTE_DIR:-${REMOTE_DIR_DEFAULT:-/root/gk/platform-backend}}"
  REMOTE_BASE="${REMOTE_BASE:-${REMOTE_BASE_DEFAULT:-${REMOTE_DIR%/*}}}"
  PLATFORM_IMAGE="${IMAGE_NAME:-${PLATFORM_IMAGE:-ver/v-platform-backend}}"
}

log() { printf '%s\n' "$*"; }
warn() { printf 'WARN %s\n' "$*"; }
fail() { printf 'FAIL %s\n' "$*" >&2; exit 1; }

remote_cmd() {
  ssh -i "$SSH_KEY" "$REMOTE" "$@"
}

remote_bash() {
  ssh -i "$SSH_KEY" "$REMOTE" 'bash -s'
}

require_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || fail "missing_command command=$cmd"
}

redact_stream() {
  sed -E \
    -e 's/(api[_-]?key[[:space:]]*[:=][[:space:]]*)[^,}[:space:]]+/\1[REDACTED]/Ig' \
    -e 's/(authorization[[:space:]]*[:=][[:space:]]*)[^,}[:space:]]+/\1[REDACTED]/Ig' \
    -e 's/(secret[[:space:]]*[:=][[:space:]]*)[^,}[:space:]]+/\1[REDACTED]/Ig' \
    -e 's/(password[[:space:]]*[:=][[:space:]]*)[^,}[:space:]]+/\1[REDACTED]/Ig' \
    -e 's/(token[[:space:]]*[:=][[:space:]]*)[^,}[:space:]]+/\1[REDACTED]/Ig'
}

write_evidence_header() {
  local file="$1" title="$2"
  mkdir -p "$(dirname "$file")"
  {
    echo "# $title"
    echo
    echo "- generated_at: $(date -Iseconds)"
    echo "- git_sha: $(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
    echo
  } > "$file"
}
