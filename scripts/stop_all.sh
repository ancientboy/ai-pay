#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"
RUN_DIR="${ROOT_DIR}/.run"

echo "[stop] root=${ROOT_DIR}"

if [[ -f "${RUN_DIR}/backend.pid" ]]; then
  pid="$(cat "${RUN_DIR}/backend.pid")"
  if kill -0 "${pid}" >/dev/null 2>&1; then
    echo "[stop] kill backend pid=${pid}"
    kill "${pid}" || true
  fi
  rm -f "${RUN_DIR}/backend.pid"
fi

if [[ -f "${RUN_DIR}/frontend.pid" ]]; then
  pid="$(cat "${RUN_DIR}/frontend.pid")"
  if kill -0 "${pid}" >/dev/null 2>&1; then
    echo "[stop] kill frontend pid=${pid}"
    kill "${pid}" || true
  fi
  rm -f "${RUN_DIR}/frontend.pid"
fi

echo "[stop] docker compose down"
(cd "${BACKEND_DIR}" && docker compose down)

echo "[stop] done"
