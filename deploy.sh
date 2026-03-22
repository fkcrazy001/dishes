#!/usr/bin/env bash
set -euxo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONT_DIR="$ROOT_DIR/dishes-front"
GO_DIR="$ROOT_DIR/dishes-go"
GO_WEB_DIST_DIR="$GO_DIR/internal/httpapi/web/dist"
GO_BIN_DIR="$GO_DIR/bin"
GO_BIN="$GO_BIN_DIR/dishes"

PORT="${PORT:-3000}"
HOST="${HOST:-0.0.0.0}"
JWT_SECRET="${JWT_SECRET:-dev-secret}"
DB_FILE="${DB_FILE:-./data/db.sqlite}"
UPLOAD_DIR="${UPLOAD_DIR:-./data/uploads}"

if [[ -f "$FRONT_DIR/package-lock.json" ]]; then
  npm --prefix "$FRONT_DIR" ci
else
  npm --prefix "$FRONT_DIR" install
fi
npm --prefix "$FRONT_DIR" run build

rm -rf "$GO_WEB_DIST_DIR"
mkdir -p "$GO_WEB_DIST_DIR"
cp -a "$FRONT_DIR/dist/." "$GO_WEB_DIST_DIR/"

mkdir -p "$GO_BIN_DIR"

cd "$GO_DIR"
export CGO_ENABLED="${CGO_ENABLED:-0}"
if [[ -n "${GOOS:-}" ]]; then export GOOS="$GOOS"; fi
if [[ -n "${GOARCH:-}" ]]; then export GOARCH="$GOARCH"; fi
if [[ -n "${GOARM:-}" ]]; then export GOARM="$GOARM"; fi
go build -o "$GO_BIN" ./cmd/dishes-go

cd "$ROOT_DIR"
PORT="$PORT" HOST="$HOST" JWT_SECRET="$JWT_SECRET" DB_FILE="$DB_FILE" UPLOAD_DIR="$UPLOAD_DIR" "$GO_BIN"
