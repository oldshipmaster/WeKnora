#!/usr/bin/env bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export DASHSCOPE_API_KEY
DASHSCOPE_API_KEY="$(security find-generic-password -a oldshipmaster -s weknora-dashscope-api-key -w)"

export SYSTEM_AES_KEY
SYSTEM_AES_KEY="$(security find-generic-password -a oldshipmaster -s weknora-system-aes-key -w)"

cd "$project_dir"
exec docker compose --profile neo4j up -d "$@"
