#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$ROOT_DIR/scripts/lib/mysql-local.sh"

usage() {
  cat <<'EOF'
Usage: bash scripts/migrate-mysql.sh <up|down|reset>
EOF
}

cmd="${1:-up}"

ensure_mysql_started

case "$cmd" in
  up)
    mysql_app "$LAGHAIM_DB_NAME" < "$ROOT_DIR/migrations/000001_p0_core.up.sql"
    echo "applied migrations up"
    ;;
  down)
    mysql_app "$LAGHAIM_DB_NAME" < "$ROOT_DIR/migrations/000001_p0_core.down.sql"
    echo "applied migrations down"
    ;;
  reset)
    mysql_app "$LAGHAIM_DB_NAME" < "$ROOT_DIR/migrations/000001_p0_core.down.sql" || true
    mysql_app "$LAGHAIM_DB_NAME" < "$ROOT_DIR/migrations/000001_p0_core.up.sql"
    echo "reset migrations"
    ;;
  *)
    usage
    exit 1
    ;;
esac
