#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$ROOT_DIR/scripts/lib/mysql-local.sh"

bash "$ROOT_DIR/scripts/mysql-local.sh" init
bash "$ROOT_DIR/scripts/migrate-mysql.sh" reset

LAGHAIM_TEST_MYSQL_DSN="$(laghaim_mysql_dsn)" \
  go test ./internal/server/authselect -run 'TestMySQLGatewayAndZoneIntegration' -count=1
