#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mode="${1:-infra}"
cache_root="${WEKNORA_GO_CACHE_ROOT:-/Volumes/extfastdata01/.cache/weknora-go}"

cd "${repo_dir}"

case "${mode}" in
  infra)
    exec docker compose -f docker-compose.dev.yml --profile neo4j up -d postgres redis docreader neo4j
    ;;
  app)
    mkdir -p "${cache_root}/mod" "${cache_root}/build" "${cache_root}/tmp"
    export GOMODCACHE="${cache_root}/mod"
    export GOCACHE="${cache_root}/build"
    export GOTMPDIR="${cache_root}/tmp"
    export DASHSCOPE_API_KEY
    DASHSCOPE_API_KEY="$(security find-generic-password -a oldshipmaster -s weknora-dashscope-api-key -w)"
    export SYSTEM_AES_KEY
    SYSTEM_AES_KEY="$(security find-generic-password -a oldshipmaster -s weknora-system-aes-key -w)"
    exec ./scripts/dev.sh app
    ;;
  frontend)
    exec ./scripts/dev.sh frontend
    ;;
  *)
    printf 'usage: %s [infra|app|frontend]\n' "$0" >&2
    exit 2
    ;;
esac
