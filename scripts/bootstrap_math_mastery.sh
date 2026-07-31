#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
password="$(security find-generic-password -s weknora-math-admin-password -a math-master -w)"
cache_root="${WEKNORA_GO_CACHE_ROOT:-/Volumes/extfastdata01/.cache/weknora-go}"

mkdir -p "${cache_root}/mod" "${cache_root}/build" "${cache_root}/tmp"
export WEKNORA_MATH_ADMIN_PASSWORD="${password}"
export GOMODCACHE="${cache_root}/mod"
export GOCACHE="${cache_root}/build"
export GOTMPDIR="${cache_root}/tmp"

cd "${repo_dir}"
exec go run ./cmd/math-mastery-bootstrap "$@"
