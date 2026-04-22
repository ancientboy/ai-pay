#!/usr/bin/env bash
set -euo pipefail

echo "[rollback] start fallback to in-memory mode"

if [[ -f ".env" ]]; then
  cp .env .env.bak.$(date +%Y%m%d%H%M%S)
  # Remove persistence envs so server uses in-memory fallback.
  sed -i '/^MYSQL_DSN=/d' .env || true
  sed -i '/^REDIS_ADDR=/d' .env || true
  sed -i '/^REDIS_PASSWORD=/d' .env || true
fi

echo "[rollback] persistence env removed. restart service with:"
echo "  export \$(grep -v '^#' .env | xargs)"
echo "  go run ./cmd/server"
echo "[rollback] done"
