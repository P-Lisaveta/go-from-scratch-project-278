#!/bin/sh
set -eu

echo "[run.sh] Starting service"

echo "[run.sh] Running DB migrations"
goose -dir ./db/migrations postgres "${DATABASE_URL}" up

echo "[run.sh] Starting Go app"
PORT=8081 /app/bin/app &
api_pid=$!

stop() {
  kill -TERM "$api_pid" "$caddy_pid" 2>/dev/null || true
  wait "$api_pid" "$caddy_pid" 2>/dev/null || true
  exit 0
}

echo "[run.sh] Starting Caddy"
caddy run --config /etc/caddy/Caddyfile --adapter caddyfile &
caddy_pid=$!
trap stop INT TERM

wait "$caddy_pid"
caddy_status=$?
kill -TERM "$api_pid" 2>/dev/null || true
wait "$api_pid" 2>/dev/null || true
exit "$caddy_status"
