#!/bin/bash
# Start nsm and Tendermint locally for testing

set -e

cd "$(dirname "$0")"

echo "Cleaning up..."
killall -9 nsm tendermint 2>/dev/null || true
rm -f nsm.sock
sleep 1

echo "Starting nsm..."
PORT=8080 KEY_FILE=nsm_key.pem HOST_DATA_FILE=test-hosts.json TEMPLATE_DIR=internal/web \
  ./bin/nsm > logs/nsm.log 2>&1 &
NSM_PID=$!
echo "  nsm started (PID: $NSM_PID)"
sleep 2

if [ ! -S nsm.sock ]; then
  echo "ERROR: nsm.sock not created"
  exit 1
fi

echo "Starting Tendermint..."
./bin/tendermint node --proxy-app=unix://$(pwd)/nsm.sock > logs/tendermint.log 2>&1 &
TMINT_PID=$!
echo "  Tendermint started (PID: $TMINT_PID)"
sleep 5

echo "Checking status..."
if curl -s http://localhost:26657/status > /dev/null 2>&1; then
  BLOCK_HEIGHT=$(curl -s http://localhost:26657/status 2>/dev/null | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*')
  echo "✓ Tendermint responding (block height: $BLOCK_HEIGHT)"
else
  echo "ERROR: Tendermint not responding"
  exit 1
fi

echo ""
echo "=== Both services running ==="
echo "  nsm: http://localhost:8080"
echo "  Tendermint RPC: http://localhost:26657"
echo ""
echo "To stop: killall nsm tendermint"
