#!/bin/bash
set -e

ROOT="/Users/qwilton/projects/kp/proxy-kp"
BIN="$ROOT/bin"

cleanup() {
  echo ""; echo "Cleaning up..."
  kill $PROXY_PID 2>/dev/null || true
  kill $B4_PID 2>/dev/null || true
  wait 2>/dev/null
  echo "Done."
}
trap cleanup EXIT INT TERM

echo "Starting test backend..."
"$BIN/backend4" & B4_PID=$!
sleep 1

run_scenario() {
  local LABEL=$1
  local CONFIG=$2
  local METHOD_FLAG=$3
  local URL=$4
  local CONN=$5

  "$BIN/proxy" -config "$CONFIG" & PROXY_PID=$!
  sleep 1

  echo ""
  echo "──────────────────────────────────────────────────────────────"
  echo "  $LABEL"
  echo "──────────────────────────────────────────────────────────────"

  wrk -t$CONN -c$CONN -d20s --latency $METHOD_FLAG "$URL" 2>&1

  kill $PROXY_PID 2>/dev/null || true
  wait $PROXY_PID 2>/dev/null || true
  sleep 1
}

CACHE="$ROOT/config.loadtest.backend.yaml"
NOCACHE="$ROOT/config.loadtest.nocache.yaml"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║           Load Test — сценарии бэкенда                       ║"
echo "╚══════════════════════════════════════════════════════════════╝"

# 1. GET cached — cache HIT
run_scenario "1. GET /api/cached  (cache ON)" \
  "$CACHE" "" "http://localhost:8080/api/cached" 8

# 2. GET cached — cache MISS (cache disabled)
run_scenario "2. GET /api/cached  (cache OFF)" \
  "$NOCACHE" "" "http://localhost:8080/api/cached" 8

# 3. GET fast uncached
run_scenario "3. GET /api/fast    (cache ON)" \
  "$CACHE" "" "http://localhost:8080/api/fast" 8

# 4. GET fast uncached, no cache
run_scenario "4. GET /api/fast    (cache OFF)" \
  "$NOCACHE" "" "http://localhost:8080/api/fast" 8

# 5. POST slow
run_scenario "5. POST /api/order  (cache OFF)" \
  "$NOCACHE" "-X POST" "http://localhost:8080/api/order" 8

# 6. POST slow with cache ON (no effect, POST not cached)
run_scenario "6. POST /api/order  (cache ON)ы" \
  "$CACHE" "-X POST" "http://localhost:8080/api/order" 8

echo ""
echo "══════════════════════════════════════════════════════════════"
echo "  Все тесты завершены!"
echo "══════════════════════════════════════════════════════════════"
