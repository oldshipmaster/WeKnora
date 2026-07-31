#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mode="${1:-infra}"
cache_root="${WEKNORA_GO_CACHE_ROOT:-/Volumes/extfastdata01/.cache/weknora-go}"

cd "${repo_dir}"

case "${mode}" in
  infra)
    runtime_dir="${WEKNORA_MATH_RUNTIME_DIR:-/Volumes/extfastdata01/WeKnora-runtime/math-mastery}"
    mkdir -p "${runtime_dir}/postgres" "${runtime_dir}/redis" "${runtime_dir}/docreader" "${runtime_dir}/neo4j"
    exec docker compose -f docker-compose.dev.yml -f docker-compose.math.yml --profile neo4j up -d postgres redis docreader neo4j
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
