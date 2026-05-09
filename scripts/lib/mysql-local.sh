#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

MYSQL_ROOT_USER="${MYSQL_ROOT_USER:-root}"
MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-root}"
LAGHAIM_DB_NAME="${LAGHAIM_DB_NAME:-laghaim}"
LAGHAIM_APP_USER="${LAGHAIM_APP_USER:-laghaim}"
LAGHAIM_APP_PASSWORD="${LAGHAIM_APP_PASSWORD:-laghaim}"
MYSQL_HOST="${MYSQL_HOST:-127.0.0.1}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_SOCKET="${MYSQL_SOCKET:-}"
MYSQL_SERVER_BIN="${MYSQL_SERVER_BIN:-}"

if [[ -z "$MYSQL_SOCKET" ]]; then
  for candidate in \
    "/Applications/XAMPP/xamppfiles/var/mysql/mysql.sock" \
    "/tmp/mysql.sock" \
    "/opt/homebrew/var/mysql/mysql.sock" \
    "/usr/local/var/mysql/mysql.sock"
  do
    if [[ -S "$candidate" ]]; then
      MYSQL_SOCKET="$candidate"
      break
    fi
  done
fi

if [[ -z "$MYSQL_SERVER_BIN" ]]; then
  for candidate in \
    "/Applications/XAMPP/xamppfiles/bin/mysql.server" \
    "/opt/homebrew/bin/mysql.server" \
    "/usr/local/bin/mysql.server"
  do
    if [[ -x "$candidate" ]]; then
      MYSQL_SERVER_BIN="$candidate"
      break
    fi
  done
fi

mysql_root() {
  if [[ -n "$MYSQL_SOCKET" ]]; then
    mysql --socket="$MYSQL_SOCKET" -u"$MYSQL_ROOT_USER" -p"$MYSQL_ROOT_PASSWORD" "$@"
    return
  fi
  mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_ROOT_USER" -p"$MYSQL_ROOT_PASSWORD" "$@"
}

mysql_app() {
  if [[ -n "$MYSQL_SOCKET" ]]; then
    mysql --socket="$MYSQL_SOCKET" -u"$LAGHAIM_APP_USER" -p"$LAGHAIM_APP_PASSWORD" "$@"
    return
  fi
  mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$LAGHAIM_APP_USER" -p"$LAGHAIM_APP_PASSWORD" "$@"
}

mysql_root_ping() {
  if [[ -n "$MYSQL_SOCKET" ]]; then
    mysqladmin --socket="$MYSQL_SOCKET" -u"$MYSQL_ROOT_USER" -p"$MYSQL_ROOT_PASSWORD" ping >/dev/null 2>&1
    return
  fi
  mysqladmin -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_ROOT_USER" -p"$MYSQL_ROOT_PASSWORD" ping >/dev/null 2>&1
}

laghaim_mysql_dsn() {
  if [[ -n "$MYSQL_SOCKET" ]]; then
    printf '%s:%s@unix(%s)/%s?parseTime=true&multiStatements=true\n' \
      "$LAGHAIM_APP_USER" "$LAGHAIM_APP_PASSWORD" "$MYSQL_SOCKET" "$LAGHAIM_DB_NAME"
    return
  fi
  printf '%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true\n' \
    "$LAGHAIM_APP_USER" "$LAGHAIM_APP_PASSWORD" "$MYSQL_HOST" "$MYSQL_PORT" "$LAGHAIM_DB_NAME"
}

ensure_mysql_started() {
  if mysql_root_ping; then
    return
  fi

  if [[ -n "$MYSQL_SERVER_BIN" ]]; then
    "$MYSQL_SERVER_BIN" start
  elif command -v brew >/dev/null 2>&1; then
    if brew list mariadb >/dev/null 2>&1; then
      brew services start mariadb
    elif brew list mysql >/dev/null 2>&1; then
      brew services start mysql
    else
      echo "no supported local mysql launcher found" >&2
      return 1
    fi
  else
    echo "no supported local mysql launcher found" >&2
    return 1
  fi

  for _ in $(seq 1 20); do
    if mysql_root_ping; then
      return
    fi
    sleep 1
  done

  echo "mysql did not become ready in time" >&2
  return 1
}
