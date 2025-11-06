#!/bin/bash
# Manage NSM services on remote hosts
# Usage: ./manage-remote.sh [start|stop|status|logs|restart] [host_ip...]

set -e

ACTION="${1:-status}"
shift 2>/dev/null || true

# Configuration
SSH_KEY="$HOME/.ssh/nsm-vbox.key"
SSH_USER="nsm"
REMOTE_DIR="/home/nsm"

# Default hosts
DEFAULT_HOSTS=(
    "192.168.10.147"
    "192.168.10.174"
    "192.168.10.135"
    "192.168.10.211"
)

HOSTS=("${@:-${DEFAULT_HOSTS[@]}}")

case "$ACTION" in
    start)
        echo "=== Starting NSM on remote hosts ==="
        for HOST in "${HOSTS[@]}"; do
            echo "Starting on $HOST..."
            ssh -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
                "cd $REMOTE_DIR && nohup ./nsm > nsm.log 2>&1 &" 2>&1 | sed 's/^/  /'
            sleep 1
        done
        echo ""
        $0 status "${HOSTS[@]}"
        ;;
    
    stop)
        echo "=== Stopping NSM on remote hosts ==="
        for HOST in "${HOSTS[@]}"; do
            echo "Stopping on $HOST..."
            ssh -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
                "pkill -f 'nsm$' 2>/dev/null && echo '  ✅ Stopped' || echo '  ℹ️  Not running'"
        done
        echo ""
        ;;
    
    restart)
        echo "=== Restarting NSM on remote hosts ==="
        $0 stop "${HOSTS[@]}"
        sleep 2
        $0 start "${HOSTS[@]}"
        ;;
    
    logs)
        echo "=== NSM Logs from remote hosts ==="
        for HOST in "${HOSTS[@]}"; do
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "📋 Logs from $HOST:"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            ssh -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
                "tail -20 $REMOTE_DIR/nsm.log 2>/dev/null || echo 'No log file found'"
        done
        echo ""
        ;;
    
    status|*)
        echo "=== NSM Status on remote hosts ==="
        echo ""
        for HOST in "${HOSTS[@]}"; do
            if ! ping -c 1 -W 2 "$HOST" &> /dev/null; then
                echo "  $HOST: ⚠️  Unreachable"
                continue
            fi
            
            PID=$(ssh -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
                "pgrep -f 'nsm$' 2>/dev/null" || echo "")
            
            if [ -n "$PID" ]; then
                UPTIME=$(ssh -i "$SSH_KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
                    "ps -p $PID -o etime= 2>/dev/null | tr -d ' '" || echo "unknown")
                echo "  $HOST: ✅ Running (PID: $PID, uptime: $UPTIME) - http://$HOST:8080"
            else
                echo "  $HOST: ❌ Not running"
            fi
        done
        echo ""
        ;;
esac
