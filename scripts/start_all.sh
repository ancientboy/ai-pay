#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"
FRONTEND_DIR="${ROOT_DIR}/frontend"
RUN_DIR="${ROOT_DIR}/.run"

mkdir -p "${RUN_DIR}"

echo "[start] root=${ROOT_DIR}"

echo "[start] ensure backend env"
if [[ ! -f "${BACKEND_DIR}/.env" ]]; then
  cp "${BACKEND_DIR}/.env.example" "${BACKEND_DIR}/.env"
fi

echo "[start] ensure frontend env"
if [[ ! -f "${FRONTEND_DIR}/.env.local" ]]; then
  cp "${FRONTEND_DIR}/.env.example" "${FRONTEND_DIR}/.env.local"
fi

echo "[start] docker compose up (mysql+redis)"
(cd "${BACKEND_DIR}" && docker compose up -d)

if lsof -nP -iTCP:18080 -sTCP:LISTEN >/dev/null 2>&1; then
  echo "[start] backend port 18080 already in use, skip start"
else
  echo "[start] backend start on :18080 (persistent mode)"
  nohup env \
    PORT=18080 \
    MYSQL_DSN="root:root@tcp(127.0.0.1:3307)/ai_pay?parseTime=true" \
    REDIS_ADDR="127.0.0.1:6379" \
    REDIS_PASSWORD="" \
    bash -lc "cd '${BACKEND_DIR}' && go run ./cmd/server" \
    >"${RUN_DIR}/backend.log" 2>&1 &
  echo $! > "${RUN_DIR}/backend.pid"
fi

if lsof -nP -iTCP:3000 -sTCP:LISTEN >/dev/null 2>&1; then
  echo "[start] frontend port 3000 already in use, skip start"
else
  echo "[start] frontend start on :3000"
  nohup bash -lc "cd '${FRONTEND_DIR}' && npm run dev" \
    >"${RUN_DIR}/frontend.log" 2>&1 &
  echo $! > "${RUN_DIR}/frontend.pid"
fi

echo "[start] done"
echo "[start] check with: ${ROOT_DIR}/scripts/status_all.sh"
