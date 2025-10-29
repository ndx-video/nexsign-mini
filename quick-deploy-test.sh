#!/usr/bin/env bash
# Quick deployment script for testing nsm with real nodes

set -euo pipefail

echo "=== nexSign mini Multi-Node Test Deployment ==="
echo ""

# Load deployment config
if [ ! -f "deploy.env" ]; then
    echo "❌ deploy.env not found"
    exit 1
fi
source deploy.env

# Step 1: Build nsm binary
echo "Step 1: Building nsm binary..."
go build -o bin/nsm cmd/nsm/main.go
echo "✓ Binary built"
echo ""

# Step 2: Ensure Tendermint binary exists
if [ ! -f "bin/tendermint" ]; then
    echo "❌ Tendermint binary not found in bin/"
    echo "Run: go install github.com/tendermint/tendermint/cmd/tendermint@v0.35.9"
    echo "Then: cp ~/go/bin/tendermint bin/tendermint"
    exit 1
fi
echo "✓ Tendermint binary found"
echo ""

# Step 3: Clean remote hosts
echo "Step 2: Cleaning remote hosts..."
for host in "${HOSTS[@]}"; do
    echo "  Cleaning $host..."
    ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "$SSH_USER@$host" \
        "pkill -9 nsm 2>/dev/null || true; pkill -9 tendermint 2>/dev/null || true; rm -rf /home/nsm/.nsm /home/nsm/.tendermint" 2>&1 | head -2
done
echo "✓ All hosts cleaned"
echo ""

# Step 3.5: Generate unique identity keys for each node
echo "Step 2.5: Generating unique identity keys for each node..."
mkdir -p /tmp/nsm-keys

# Build key generator if needed
if [ ! -f "/tmp/generate_key" ]; then
    echo "  Building key generator..."
    go build -tags manual -o /tmp/generate_key generate_key.go
fi

for i in "${!HOSTS[@]}"; do
    host="${HOSTS[$i]}"
    keyfile="/tmp/nsm-keys/nsm_key_${host}.pem"
    
    if [ ! -f "$keyfile" ]; then
        echo "  Generating key for $host..."
        /tmp/generate_key "$keyfile"
    fi
    
    # Copy key to remote host
    echo "  Deploying key to $host..."
    ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "$SSH_USER@$host" "mkdir -p /home/nsm/.nsm"
    scp -o StrictHostKeyChecking=no -i "$SSH_KEY" "$keyfile" "$SSH_USER@$host:/home/nsm/.nsm/nsm_key.pem"
    ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "$SSH_USER@$host" "chmod 600 /home/nsm/.nsm/nsm_key.pem"
done
echo "✓ Unique keys generated and deployed"
echo ""

# Step 4: Deploy with Tendermint
echo "Step 3: Deploying nsm + Tendermint to all nodes..."
# Temporarily move local key so deploy script doesn't copy it
if [ -f "nsm_key.pem" ]; then
    mv nsm_key.pem nsm_key.pem.backup
    MOVED_KEY=1
else
    MOVED_KEY=0
fi

./test-deploy.sh --with-tendermint --verbose

# Restore local key
if [ $MOVED_KEY -eq 1 ]; then
    mv nsm_key.pem.backup nsm_key.pem
fi

if [ $? -ne 0 ]; then
    echo "❌ Deployment failed"
    exit 1
fi

echo ""
echo "✓ Deployment complete!"
echo ""

# Step 5: Wait for services to start
echo "Step 4: Waiting for services to initialize..."
sleep 5

# Step 6: Check node status
echo "Step 5: Checking node status..."
echo ""
for host in "${HOSTS[@]}"; do
    echo "=== $host ==="
    # Check if nsm is running
    if ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "$SSH_USER@$host" "pgrep -f nsm" >/dev/null 2>&1; then
        echo "  ✓ nsm process running"
    else
        echo "  ❌ nsm process NOT running"
    fi
    
    # Check if Tendermint is running
    if ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "$SSH_USER@$host" "pgrep -f tendermint" >/dev/null 2>&1; then
        echo "  ✓ Tendermint process running"
    else
        echo "  ❌ Tendermint process NOT running"
    fi
    
    # Try to get consensus status
    rpc_result=$(ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "$SSH_USER@$host" \
        "curl -s http://localhost:26657/status 2>/dev/null | grep -o '\"latest_block_height\":\"[^\"]*\"' | cut -d'\"' -f4" 2>/dev/null || echo "N/A")
    echo "  Tendermint block height: $rpc_result"
    
    # Try to get nsm API
    hosts_count=$(ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "$SSH_USER@$host" \
        "curl -s http://localhost:8080/api/hosts 2>/dev/null | grep -o '\"public_key\"' | wc -l" 2>/dev/null || echo "0")
    echo "  Hosts in ledger: $hosts_count"
    echo ""
done

# Step 7: Show how to access dashboards
echo "=== Dashboards ==="
for host in "${HOSTS[@]}"; do
    echo "  http://$host:8080/"
done
echo ""

echo "=== Tendermint RPC ==="
for host in "${HOSTS[@]}"; do
    echo "  http://$host:26657/status"
done
echo ""

echo "=== View Logs ==="
echo "NSM logs:"
for host in "${HOSTS[@]}"; do
    echo "  ssh -i $SSH_KEY $SSH_USER@$host tail -f /home/nsm/.nsm/nsm.log"
done
echo ""
echo "Tendermint logs:"
for host in "${HOSTS[@]}"; do
    echo "  ssh -i $SSH_KEY $SSH_USER@$host tail -f /home/nsm/.tendermint/tendermint.log"
done
echo ""

echo "=== Test Complete ==="
echo "All nodes should now be:"
echo "  1. Running nsm with ABCI server"
echo "  2. Running Tendermint consensus"
echo "  3. Broadcasting their host info to the ledger"
echo "  4. Showing all discovered hosts in the dashboard"
echo ""
echo "Check any dashboard to see the full network!"
