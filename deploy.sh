#!/bin/bash
# Deploy NSM to remote hosts
# Usage: ./deploy.sh [host_ip] or ./deploy.sh all

set -e

KEY="$HOME/.ssh/nsm-vbox.key"
USER="nsm"
REMOTE_DIR="/home/nsm/nsm"

# List of NSM hosts
HOSTS=(
    "192.168.10.147"
    "192.168.10.174"
    "192.168.10.135"
    "192.168.10.211"
)

deploy_to_host() {
    local HOST=$1
    echo "==> Deploying to $HOST..."
    
    # Stop any running NSM process
    ssh -i "$KEY" "$USER@$HOST" "pkill -f 'nsm$' || true" 2>/dev/null
    
    # Create remote directory
    ssh -i "$KEY" "$USER@$HOST" "mkdir -p $REMOTE_DIR/internal/web"
    
    # Copy binary
    scp -i "$KEY" ./nsm "$USER@$HOST:$REMOTE_DIR/"
    
    # Copy templates
    scp -i "$KEY" internal/web/*.html "$USER@$HOST:$REMOTE_DIR/internal/web/"
    
    # Copy static files
    ssh -i "$KEY" "$USER@$HOST" "mkdir -p $REMOTE_DIR/internal/web/static"
    scp -i "$KEY" internal/web/static/* "$USER@$HOST:$REMOTE_DIR/internal/web/static/" 2>/dev/null || true
    
    # Make binary executable
    ssh -i "$KEY" "$USER@$HOST" "chmod +x $REMOTE_DIR/nsm"
    
    # Start NSM in background with nohup
    ssh -i "$KEY" "$USER@$HOST" "cd $REMOTE_DIR && nohup ./nsm > nsm.log 2>&1 &"
    
    sleep 2
    
    # Check if it's running
    if ssh -i "$KEY" "$USER@$HOST" "pgrep -f 'nsm$' > /dev/null"; then
        echo "✓ NSM deployed and running on $HOST"
    else
        echo "✗ Failed to start NSM on $HOST"
        return 1
    fi
}

# Build binary first
echo "Building NSM binary..."
go build -o nsm .

if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

echo "✓ Build successful"
echo ""

# Deploy based on argument
if [ "$1" == "all" ] || [ -z "$1" ]; then
    for HOST in "${HOSTS[@]}"; do
        deploy_to_host "$HOST"
        echo ""
    done
else
    deploy_to_host "$1"
fi

echo "Deployment complete!"
