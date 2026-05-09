#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$ROOT_DIR/scripts/lib/mysql-local.sh"

bash "$ROOT_DIR/scripts/mysql-local.sh" init
export LAGHAIM_STORAGE_BACKEND=mysql
export LAGHAIM_DATABASE_DSN="$(laghaim_mysql_dsn)"

exec go run ./cmd/dev-cluster -config configs/dev.yaml
