#!/bin/bash
# Manage NSM services on remote hosts
# Usage: ./nsm-remote.sh [start|stop|restart|status|logs] [host_ip|all]

KEY="$HOME/.ssh/nsm-vbox.key"
USER="nsm"
REMOTE_DIR="/home/nsm/nsm"

HOSTS=(
    "192.168.10.147"
    "192.168.10.174"
    "192.168.10.135"
    "192.168.10.211"
)

start_service() {
    local HOST=$1
    echo "Starting NSM on $HOST..."
    ssh -i "$KEY" "$USER@$HOST" "cd $REMOTE_DIR && nohup ./nsm > nsm.log 2>&1 &"
}

stop_service() {
    local HOST=$1
    echo "Stopping NSM on $HOST..."
    ssh -i "$KEY" "$USER@$HOST" "pkill -f 'nsm$' || true"
}

status_service() {
    local HOST=$1
    echo -n "$HOST: "
    if ssh -i "$KEY" "$USER@$HOST" "pgrep -f 'nsm$' > /dev/null"; then
        PID=$(ssh -i "$KEY" "$USER@$HOST" "pgrep -f 'nsm$'")
        echo "✓ Running (PID: $PID)"
    else
        echo "✗ Not running"
    fi
}

show_logs() {
    local HOST=$1
    echo "==> Logs from $HOST:"
    ssh -i "$KEY" "$USER@$HOST" "tail -20 $REMOTE_DIR/nsm.log 2>/dev/null || echo 'No logs found'"
    echo ""
}

ACTION=$1
TARGET=$2

if [ -z "$ACTION" ]; then
    echo "Usage: $0 [start|stop|restart|status|logs] [host_ip|all]"
    exit 1
fi

# Default to all hosts if no target specified
if [ -z "$TARGET" ] || [ "$TARGET" == "all" ]; then
    TARGETS=("${HOSTS[@]}")
else
    TARGETS=("$TARGET")
fi

# Execute action
for HOST in "${TARGETS[@]}"; do
    case "$ACTION" in
        start)
            start_service "$HOST"
            ;;
        stop)
            stop_service "$HOST"
            ;;
        restart)
            stop_service "$HOST"
            sleep 1
            start_service "$HOST"
            ;;
        status)
            status_service "$HOST"
            ;;
        logs)
            show_logs "$HOST"
            ;;
        *)
            echo "Unknown action: $ACTION"
            exit 1
            ;;
    esac
done
