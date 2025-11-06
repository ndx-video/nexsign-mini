#!/bin/bash
# Deploy NSM to remote hosts
# Usage: ./deploy-to-hosts.sh [host_ip...]
# If no IPs provided, deploys to all known hosts

set -e

# Configuration
SSH_KEY="$HOME/.ssh/nsm-vbox.key"
SSH_USER="nsm"
REMOTE_DIR="/home/nsm"
SERVICE_NAME="nsm"

# Default hosts from hosts.json (excluding frodo which is the local dev machine)
DEFAULT_HOSTS=(
    "192.168.10.147"
    "192.168.10.174"
    "192.168.10.135"
    "192.168.10.211"
)

# Use provided hosts or default to all
HOSTS=("${@:-${DEFAULT_HOSTS[@]}}")

echo "=== NSM Deployment Script ==="
echo ""

# Build the binary
echo "📦 Building NSM binary..."
GOOS=linux GOARCH=amd64 go build -o nsm .
if [ $? -ne 0 ]; then
    echo "❌ Build failed!"
    exit 1
fi
echo "✅ Build successful"
echo ""

# Deploy to each host
for HOST in "${HOSTS[@]}"; do
    echo "🚀 Deploying to $HOST..."
    
    # Check if host is reachable
    if ! ping -c 1 -W 2 "$HOST" &> /dev/null; then
        echo "   ⚠️  Host unreachable, skipping..."
        echo ""
        continue
    fi
    
    # Stop existing service if running
    echo "   Stopping existing NSM service..."
    ssh -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
        "pkill -f 'nsm$' 2>/dev/null || true" 2>&1 | sed 's/^/   /'
    
    # Copy binary
    echo "   Copying binary and templates..."
    scp -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no \
        nsm "$SSH_USER@$HOST:$REMOTE_DIR/" 2>&1 | sed 's/^/   /'
    
    # Copy templates directory
    ssh -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
        "mkdir -p $REMOTE_DIR/internal/web" 2>&1 | sed 's/^/   /'
    scp -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no \
        internal/web/*.html "$SSH_USER@$HOST:$REMOTE_DIR/internal/web/" 2>&1 | sed 's/^/   /'
    
    # Copy static files
    ssh -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
        "mkdir -p $REMOTE_DIR/internal/web/static" 2>&1 | sed 's/^/   /'
    scp -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no \
        internal/web/static/* "$SSH_USER@$HOST:$REMOTE_DIR/internal/web/static/" 2>&1 | sed 's/^/   /'
    
    # Set executable permissions
    echo "   Setting permissions..."
    ssh -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
        "chmod +x $REMOTE_DIR/nsm" 2>&1 | sed 's/^/   /'
    
    # Start the service in background
    echo "   Starting NSM service..."
    ssh -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
        "cd $REMOTE_DIR && nohup ./nsm > nsm.log 2>&1 &" 2>&1 | sed 's/^/   /'
    
    # Wait a moment and check if it's running
    sleep 2
    echo "   Checking service status..."
    RUNNING=$(ssh -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
        "pgrep -f 'nsm$' > /dev/null && echo 'running' || echo 'not running'")
    
    if [ "$RUNNING" = "running" ]; then
        echo "   ✅ NSM is running on $HOST"
        # Show the last few lines of the log
        echo "   📋 Recent log output:"
        ssh -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
            "tail -5 $REMOTE_DIR/nsm.log" 2>&1 | sed 's/^/      /'
    else
        echo "   ❌ NSM failed to start on $HOST"
        echo "   📋 Log output:"
        ssh -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
            "cat $REMOTE_DIR/nsm.log 2>/dev/null || echo 'No log file found'" 2>&1 | sed 's/^/      /'
    fi
    
    echo ""
done

echo "=== Deployment Summary ==="
echo ""
for HOST in "${HOSTS[@]}"; do
    if ping -c 1 -W 2 "$HOST" &> /dev/null; then
        RUNNING=$(ssh -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
            "pgrep -f 'nsm$' > /dev/null && echo '✅ Running' || echo '❌ Not running'" 2>/dev/null)
        echo "  $HOST: $RUNNING - http://$HOST:8080"
    else
        echo "  $HOST: ⚠️  Unreachable"
    fi
done
echo ""
echo "🎉 Deployment complete!"
