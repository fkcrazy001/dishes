#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONT_DIR="$ROOT_DIR/dishes-front"
BACK_DIR="$ROOT_DIR/dishes-backend"

PORT="${PORT:-3000}"
HOST="${HOST:-0.0.0.0}"
JWT_SECRET="${JWT_SECRET:-dev-secret}"
DB_FILE="${DB_FILE:-./data/db.sqlite}"

if [[ -f "$FRONT_DIR/package-lock.json" ]]; then
  npm --prefix "$FRONT_DIR" ci
else
  npm --prefix "$FRONT_DIR" install
fi
npm --prefix "$FRONT_DIR" run build

if [[ -f "$BACK_DIR/package-lock.json" ]]; then
  npm --prefix "$BACK_DIR" ci
else
  npm --prefix "$BACK_DIR" install
fi
npm --prefix "$BACK_DIR" run build

cd "$BACK_DIR"
PORT="$PORT" HOST="$HOST" JWT_SECRET="$JWT_SECRET" DB_FILE="$DB_FILE" node dist/server.js
