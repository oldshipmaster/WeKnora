#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mode="${1:-infra}"

external_root="${WEKNORA_EXTERNAL_ROOT:-/Volumes/extfastdata01}"
case "${external_root}" in
    /Volumes/*) ;;
    *)
        printf 'external data root must be mounted below /Volumes: %s\n' "${external_root}" >&2
        exit 1
        ;;
esac
if [[ ! -d "${external_root}" ]]; then
    printf 'external data volume is not mounted: %s\n' "${external_root}" >&2
    exit 1
fi
if [[ "$(df -P "${external_root}" | awk 'NR == 2 {print $1}')" == "$(df -P / | awk 'NR == 2 {print $1}')" ]]; then
    printf 'external data root resolves to the system disk: %s\n' "${external_root}" >&2
    exit 1
fi
runtime_root="${WEKNORA_RUNTIME_ROOT:-${external_root}/WeKnora-runtime}"
cache_root="${WEKNORA_CACHE_ROOT:-${external_root}/.cache}"

export GOPATH="${WEKNORA_GOPATH:-${cache_root}/weknora-go-path}"
export GOMODCACHE="${WEKNORA_GOMODCACHE:-${cache_root}/weknora-go-mod}"
export GOCACHE="${WEKNORA_GOCACHE:-${cache_root}/weknora-go-build}"
export GOTMPDIR="${WEKNORA_GOTMPDIR:-${cache_root}/weknora-go-tmp}"
export TMPDIR="${WEKNORA_TMPDIR:-${runtime_root}/tmp}"
export NPM_CONFIG_CACHE="${WEKNORA_NPM_CACHE:-${cache_root}/weknora-npm}"
export XDG_CACHE_HOME="${WEKNORA_XDG_CACHE_HOME:-${cache_root}/weknora-xdg}"

resolve_planned_directory() {
    local candidate="$1"
    local missing_suffix=""
    while [[ ! -e "${candidate}" ]]; do
        missing_suffix="/$(basename "${candidate}")${missing_suffix}"
        candidate="$(dirname "${candidate}")"
    done
    (cd "${candidate}" && printf '%s%s\n' "$(pwd -P)" "${missing_suffix}")
}

external_root_resolved="$(cd "${external_root}" && pwd -P)"
for writable_path in \
    "${GOPATH}" "${GOMODCACHE}" "${GOCACHE}" "${GOTMPDIR}" \
    "${TMPDIR}" "${NPM_CONFIG_CACHE}" "${XDG_CACHE_HOME}"; do
    if [[ "${writable_path}/" == *"/../"* ]]; then
        printf 'refusing writable path with parent traversal: %s\n' "${writable_path}" >&2
        exit 1
    fi
    case "${writable_path}" in
        "${external_root}"/*) ;;
        *)
            printf 'refusing writable path outside external data root: %s\n' "${writable_path}" >&2
            exit 1
            ;;
    esac
    writable_path_resolved="$(resolve_planned_directory "${writable_path}")"
    case "${writable_path_resolved}" in
        "${external_root_resolved}"/*) ;;
        *)
            printf 'refusing writable path that resolves outside external data root: %s -> %s\n' \
                "${writable_path}" "${writable_path_resolved}" >&2
            exit 1
            ;;
    esac
done
mkdir -p \
    "${GOPATH}" "${GOMODCACHE}" "${GOCACHE}" "${GOTMPDIR}" \
    "${TMPDIR}" "${NPM_CONFIG_CACHE}" "${XDG_CACHE_HOME}"

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
