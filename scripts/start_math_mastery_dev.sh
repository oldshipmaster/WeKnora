#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mode="${1:-infra}"

cd "${repo_dir}"

case "${mode}" in
  infra)
    runtime_dir="${WEKNORA_MATH_RUNTIME_DIR:-/Volumes/extfastdata01/WeKnora-runtime/math-mastery}"
    mkdir -p "${runtime_dir}/postgres" "${runtime_dir}/redis" "${runtime_dir}/neo4j"
    exec docker compose -f docker-compose.dev.yml -f docker-compose.math.yml --profile neo4j up -d postgres redis neo4j
    ;;
  infra-full)
    runtime_dir="${WEKNORA_MATH_RUNTIME_DIR:-/Volumes/extfastdata01/WeKnora-runtime/math-mastery}"
    mkdir -p "${runtime_dir}/postgres" "${runtime_dir}/redis" "${runtime_dir}/docreader" "${runtime_dir}/neo4j"
    exec docker compose -f docker-compose.dev.yml -f docker-compose.math.yml --profile neo4j up -d postgres redis docreader neo4j
    ;;
  app)
    export GOMODCACHE="${WEKNORA_GOMODCACHE:-/Volumes/extfastdata01/.cache/weknora-go-mod}"
    export GOCACHE="${WEKNORA_GOCACHE:-/Volumes/extfastdata01/.cache/weknora-go-build}"
    export GOTMPDIR="${WEKNORA_GOTMPDIR:-/Volumes/extfastdata01/.cache/weknora-go-tmp}"
    mkdir -p "${GOMODCACHE}" "${GOCACHE}" "${GOTMPDIR}"
    export RETRIEVE_DRIVER="${RETRIEVE_DRIVER:-postgres}"
    export DUCKDB_SKIP_EXTENSION_LOAD="${DUCKDB_SKIP_EXTENSION_LOAD:-1}"
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
    printf 'usage: %s [infra|infra-full|app|frontend]\n' "$0" >&2
    exit 2
    ;;
esac
