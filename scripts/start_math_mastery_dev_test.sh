#!/usr/bin/env bash
set -euo pipefail

test_tmp_parent="/Volumes/extfastdata01/WeKnora-runtime/test-tmp"
mkdir -p "${test_tmp_parent}"
test_root="$(mktemp -d "${test_tmp_parent}/start-math-mastery.XXXXXX")"

cleanup() {
  case "${test_root}" in
    "${test_tmp_parent}"/start-math-mastery.*) rm -rf -- "${test_root}" ;;
    *) printf 'refusing to remove unexpected test path: %s\n' "${test_root}" >&2 ;;
  esac
}
trap cleanup EXIT

mkdir -p "${test_root}/scripts"
ln -s /tmp "${test_root}/escape-link"
cp "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/start_math_mastery_dev.sh" \
  "${test_root}/scripts/start_math_mastery_dev.sh"

cat >"${test_root}/scripts/dev.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'mode=%s\n' "${1:-}"
printf 'GOPATH=%s\n' "${GOPATH:-}"
printf 'GOMODCACHE=%s\n' "${GOMODCACHE:-}"
printf 'GOCACHE=%s\n' "${GOCACHE:-}"
printf 'GOTMPDIR=%s\n' "${GOTMPDIR:-}"
printf 'TMPDIR=%s\n' "${TMPDIR:-}"
printf 'NPM_CONFIG_CACHE=%s\n' "${NPM_CONFIG_CACHE:-}"
printf 'XDG_CACHE_HOME=%s\n' "${XDG_CACHE_HOME:-}"
EOF
chmod +x "${test_root}/scripts/dev.sh"

output="$(
  env \
    -u GOPATH \
    -u GOMODCACHE \
    -u GOCACHE \
    -u GOTMPDIR \
    -u NPM_CONFIG_CACHE \
    -u XDG_CACHE_HOME \
    TMPDIR="/tmp/system-default-must-not-survive" \
    bash "${test_root}/scripts/start_math_mastery_dev.sh" frontend
)"

expected="$(cat <<'EOF'
mode=frontend
GOPATH=/Volumes/extfastdata01/.cache/weknora-go-path
GOMODCACHE=/Volumes/extfastdata01/.cache/weknora-go-mod
GOCACHE=/Volumes/extfastdata01/.cache/weknora-go-build
GOTMPDIR=/Volumes/extfastdata01/.cache/weknora-go-tmp
TMPDIR=/Volumes/extfastdata01/WeKnora-runtime/tmp
NPM_CONFIG_CACHE=/Volumes/extfastdata01/.cache/weknora-npm
XDG_CACHE_HOME=/Volumes/extfastdata01/.cache/weknora-xdg
EOF
)"

if [[ "${output}" != "${expected}" ]]; then
  printf 'unexpected runtime environment\nexpected:\n%s\nactual:\n%s\n' "${expected}" "${output}" >&2
  exit 1
fi

if env \
  WEKNORA_EXTERNAL_ROOT="/Users/xingsui" \
  WEKNORA_RUNTIME_ROOT="${test_root}/runtime" \
  WEKNORA_CACHE_ROOT="${test_root}/cache" \
  bash "${test_root}/scripts/start_math_mastery_dev.sh" frontend >/dev/null 2>&1; then
  printf 'start script accepted a data root on the system disk\n' >&2
  exit 1
fi

if env \
  WEKNORA_TMPDIR="/tmp" \
  bash "${test_root}/scripts/start_math_mastery_dev.sh" frontend >/dev/null 2>&1; then
  printf 'start script accepted a writable path outside the external data root\n' >&2
  exit 1
fi

if env \
  WEKNORA_TMPDIR="/Volumes/extfastdata01/.." \
  bash "${test_root}/scripts/start_math_mastery_dev.sh" frontend >/dev/null 2>&1; then
  printf 'start script accepted a writable path that traverses outside the external data root\n' >&2
  exit 1
fi

if env \
  WEKNORA_TMPDIR="${test_root}/escape-link" \
  bash "${test_root}/scripts/start_math_mastery_dev.sh" frontend >/dev/null 2>&1; then
  printf 'start script accepted a writable symlink that resolves outside the external data root\n' >&2
  exit 1
fi

printf 'start_math_mastery_dev external runtime test: PASS\n'
