#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$ROOT_DIR/scripts/lib/mysql-local.sh"

usage() {
  cat <<'EOF'
Usage: bash scripts/mysql-local.sh <command>

Commands:
  start     Start local MySQL/MariaDB if needed
  status    Show connection status and detected socket/server paths
  stop      Stop local MySQL/MariaDB when mysql.server is available
  init      Ensure server is running, create DB/user, and print app DSN
  dsn       Print the app DSN for LAGHAIM_DATABASE_DSN
EOF
}

cmd="${1:-status}"
case "$cmd" in
  start)
    ensure_mysql_started
    echo "mysql is ready"
    ;;
  status)
    echo "MYSQL_SOCKET=${MYSQL_SOCKET:-<tcp>}"
    echo "MYSQL_SERVER_BIN=${MYSQL_SERVER_BIN:-<none>}"
    if mysql_root_ping; then
      echo "status=ready"
    else
      echo "status=down"
      exit 1
    fi
    ;;
  stop)
    if [[ -n "$MYSQL_SERVER_BIN" ]]; then
      "$MYSQL_SERVER_BIN" stop
      echo "mysql stopped"
    else
      echo "mysql.server not found; cannot stop automatically" >&2
      exit 1
    fi
    ;;
  init)
    ensure_mysql_started
    mysql_root <<SQL
CREATE DATABASE IF NOT EXISTS ${LAGHAIM_DB_NAME} CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS '${LAGHAIM_APP_USER}'@'localhost' IDENTIFIED BY '${LAGHAIM_APP_PASSWORD}';
GRANT ALL PRIVILEGES ON ${LAGHAIM_DB_NAME}.* TO '${LAGHAIM_APP_USER}'@'localhost';
FLUSH PRIVILEGES;
SQL
    echo "initialized database ${LAGHAIM_DB_NAME}"
    echo "LAGHAIM_DATABASE_DSN=$(laghaim_mysql_dsn)"
    ;;
  dsn)
    laghaim_mysql_dsn
    ;;
  *)
    usage
    exit 1
    ;;
esac
