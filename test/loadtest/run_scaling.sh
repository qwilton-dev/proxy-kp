#!/bin/bash
set -e

BIN="/Users/qwilton/projects/kp/proxy-kp/bin"
ROOT="/Users/qwilton/projects/kp/proxy-kp"
BASE_CONFIG="$ROOT/config.loadtest.base.yaml"

cleanup() {
  echo ""; echo "Cleaning up..."
  kill $PROXY_PID 2>/dev/null || true
  for pid in "${BACKEND_PIDS[@]}"; do kill $pid 2>/dev/null || true; done
  wait 2>/dev/null
  echo "Done."
}
trap cleanup EXIT INT TERM

# kill anything on test ports
lsof -ti :8080 2>/dev/null | xargs kill -9 2>/dev/null || true
for p in $(seq 8004 8011); do lsof -ti :$p 2>/dev/null | xargs kill -9 2>/dev/null || true; done
sleep 1

run_test() {
  local N=$1
  local LABEL=$2

  BACKEND_PIDS=()

  # generate config
  CONFIG="/tmp/loadtest.$N.yaml"
  cp "$BASE_CONFIG" "$CONFIG"

  echo "backends:" >> "$CONFIG"
  for i in $(seq 1 $N); do
    PORT=$((8003 + i))
    echo "  - url: \"http://localhost:$PORT\"" >> "$CONFIG"
    echo "    weight: 1" >> "$CONFIG"
  done

  # start backends
  echo "Starting $N backends..."
  for i in $(seq 1 $N); do
    PORT=$((8003 + i))
    "$BIN/backend4" -port "$PORT" &>/dev/null &
    BACKEND_PIDS+=($!)
  done
  sleep 1

  # start proxy
  "$BIN/proxy" -config "$CONFIG" &>/tmp/proxy.log &
  PROXY_PID=$!
  sleep 1

  echo ""
  echo "╔══════════════════════════════════════════════════════════════╗"
  echo "║  $LABEL"
  echo "╚══════════════════════════════════════════════════════════════╝"
  echo ""

  for CONN in 1 5 10 25 50 100; do
    THREADS=$CONN
    [ $THREADS -gt 8 ] && THREADS=8
    echo "─── Connections: $CONN ───"
    wrk -t$THREADS -c$CONN -d15s --latency http://localhost:8080/api/fast 2>&1 | \
      grep -E "(Requests/sec|Latency| 50%| 75%| 90%| 99%|Thread Stats)"
    echo ""
  done

  kill $PROXY_PID 2>/dev/null || true
  wait $PROXY_PID 2>/dev/null || true
  for pid in "${BACKEND_PIDS[@]}"; do kill $pid 2>/dev/null || true; done
  sleep 1
}

run_test 1 "1 backend"
run_test 3 "3 backends"
run_test 8 "8 backends"

echo ""
echo "══════════════════════════════════════════════════════════════"
echo "  All tests complete!"
echo "══════════════════════════════════════════════════════════════"
