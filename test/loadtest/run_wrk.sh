set -e

ROOT="/Users/qwilton/projects/kp/proxy-kp"
BACKENDS="/Users/qwilton/projects/kp/demo-project/backends"

cleanup() {
  echo ""; echo "Cleaning up..."
  kill $PROXY_PID 2>/dev/null || true
  kill $B1_PID $B2_PID $B3_PID 2>/dev/null || true
  wait 2>/dev/null
  echo "Done."
}
trap cleanup EXIT INT TERM

# build backends if needed
for i in 1 2 3; do
  if [ ! -x "$ROOT/bin/backend$i" ]; then
    go build -o "$ROOT/bin/backend$i" "$BACKENDS/backend${i}.go"
  fi
done

# start backends
echo "Starting backends..."
"$ROOT/bin/backend1" & B1_PID=$!
"$ROOT/bin/backend2" & B2_PID=$!
"$ROOT/bin/backend3" & B3_PID=$!
sleep 2

run_test() {
  local ALGO=$1
  local LABEL=$2

  # create config
  sed "s/algorithm: \".*\"/algorithm: \"$ALGO\"/" "$ROOT/config.loadtest.yaml" > /tmp/loadtest.$ALGO.yaml

  echo ""; echo "══════════════════════════════════════════════════════════════"
  echo "  $LABEL"
  echo "══════════════════════════════════════════════════════════════"

  "$ROOT/bin/proxy" -config "/tmp/loadtest.$ALGO.yaml" &
  PROXY_PID=$!
  sleep 2

  for CONN in 1 5 10 25 50 100; do
    THREADS=$CONN
    [ $THREADS -gt 8 ] && THREADS=8
    echo ""; echo "─── Concurrency: $CONN (${THREADS} threads) ───"
    wrk -t$THREADS -c$CONN -d15s --latency http://localhost:8080/ 2>&1
  done

  kill $PROXY_PID 2>/dev/null || true
  wait $PROXY_PID 2>/dev/null || true
  sleep 2
}

run_test "srr" "Smooth Round Robin (SRR)"
run_test "leastconn" "Weighted Least Connections"

echo ""; echo "══════════════════════════════════════════════════════════════"
echo "  All tests complete!"
