#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo "[status] ports"
lsof -nP -iTCP:3000 -sTCP:LISTEN || true
lsof -nP -iTCP:18080 -sTCP:LISTEN || true
lsof -nP -iTCP:3307 -sTCP:LISTEN || true
lsof -nP -iTCP:6379 -sTCP:LISTEN || true

echo
echo "[status] health checks"
curl -sS http://127.0.0.1:18080/health || true
echo
curl -sS http://127.0.0.1:3000/api/backend/health || true
echo

echo "[status] docker ps"
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | rg "ai_pay_|NAMES" || true
