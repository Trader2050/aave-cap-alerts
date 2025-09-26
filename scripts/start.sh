#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

CONFIG_PATH="${1:-${ROOT_DIR}/config.yaml}"
if [[ ! -f "${CONFIG_PATH}" ]]; then
  echo "config file not found: ${CONFIG_PATH}" >&2
  exit 1
fi

export GOCACHE="${GOCACHE:-${ROOT_DIR}/.gocache}"

cd "${ROOT_DIR}"
exec go run ./cmd/aave-cap-alerts --config "${CONFIG_PATH}"
