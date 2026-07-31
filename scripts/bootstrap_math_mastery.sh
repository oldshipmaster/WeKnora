#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
password="$(security find-generic-password -s weknora-math-admin-password -a math-master -w)"
go_mod_cache="${WEKNORA_GOMODCACHE:-/Volumes/extfastdata01/.cache/weknora-go-mod}"
go_build_cache="${WEKNORA_GOCACHE:-/Volumes/extfastdata01/.cache/weknora-go-build}"
go_tmp_dir="${WEKNORA_GOTMPDIR:-/Volumes/extfastdata01/.cache/weknora-go-tmp}"

mkdir -p "${go_mod_cache}" "${go_build_cache}" "${go_tmp_dir}"
export WEKNORA_MATH_ADMIN_PASSWORD="${password}"
export GOMODCACHE="${go_mod_cache}"
export GOCACHE="${go_build_cache}"
export GOTMPDIR="${go_tmp_dir}"

cd "${repo_dir}"
exec go run ./cmd/math-mastery-bootstrap "$@"
