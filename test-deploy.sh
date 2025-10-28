#!/usr/bin/env bash

# Exit immediately if a command exits with a non-zero status.
set -euo pipefail

# --- Options ---
DRY_RUN=0
VERBOSE=0
WITH_SERVICE=0
PARALLEL=1
SMOKE_TEST=0
MCP_CHECK=0
MCP_PORT=4000
NSM_PORT=8080
SERVICE_FILE=""
AUTO_INSTALL_GO=1
MAX_RETRIES=3
RETRY_DELAY=2

usage() {
    cat <<EOF
Usage: $0 [options]

Options:
  --dry-run              Print commands without executing
  --verbose              Print verbose logs
  --with-service FILE    Install and start systemd service using FILE
  --parallel N           Deploy to N hosts in parallel (default: 1)
  --smoke               Run post-deploy health checks
  --mcp-check           Run MCP initialize check after deploy
  --mcp-port PORT       MCP port for health checks (default: 4000)
  --nsm-port PORT       NSM port for health checks (default: 8080)
  -h, --help            Show this help
EOF
}

while [[ ${#} -gt 0 ]]; do
    case "$1" in
        --dry-run) DRY_RUN=1; shift ;;
        --verbose) VERBOSE=1; shift ;;
        --with-service) 
            WITH_SERVICE=1
            SERVICE_FILE="$2"
            shift 2 ;;
        --parallel)
            PARALLEL="$2"
            shift 2 ;;
        --smoke) SMOKE_TEST=1; shift ;;
            --mcp-check) MCP_CHECK=1; shift ;;
            --mcp-port) MCP_PORT="$2"; shift 2 ;;
            --nsm-port) NSM_PORT="$2"; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        --) shift; break ;;
        *) echo "Unknown option: $1"; usage; exit 2 ;;
    esac
done

# --- Configuration ---
if [ -f "deploy.env" ]; then
    # shellcheck source=/dev/null
    source "deploy.env"
else
    echo "Deployment configuration file deploy.env not found."
    echo "Please create it based on the example in the README."
    exit 1
fi

SOURCE_DIR="./bin"
TARGET_BINARY="nsm"
REMOTE_NSM_DIR="/home/nsm/.nsm"
REMOTE_RESOURCES_DIR="/home/nsm/.nsm/web"
REMOTE_SERVICE_NAME="nsm.service"
REMOTE_SYSTEMD_DIR="/etc/systemd/system"

# Helper to run commands with dry-run support
run_cmd() {
    if [ "$DRY_RUN" -eq 1 ]; then
        echo "DRY-RUN: $*"
        return 0
    fi
    if [ "$VERBOSE" -eq 1 ]; then
        echo "+ $*"
    fi
    eval "$*"
}

# --- Logging (auto) ---
# Create a logs directory and redirect all stdout/stderr for the remainder
# of this script to a timestamped logfile (while still streaming to stdout).
LOG_DIR="logs"
mkdir -p "$LOG_DIR"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
LOGFILE="$LOG_DIR/deploy-$TIMESTAMP.log"
echo "Logging to $LOGFILE"
# Redirect stdout/stderr to tee so output is both visible and saved
exec > >(tee -a "$LOGFILE") 2>&1


# SCP with retries
scp_cmd() {
    local src="$1" dest="$2" retries=0
    while [ $retries -lt $MAX_RETRIES ]; do
        if [ "$DRY_RUN" -eq 1 ]; then
            echo "DRY-RUN: scp -o StrictHostKeyChecking=no -i \"$SSH_KEY\" \"$src\" \"$dest\""
            return 0
        fi
        if [ "$VERBOSE" -eq 1 ]; then
            echo "+ scp -o StrictHostKeyChecking=no -i \"$SSH_KEY\" \"$src\" \"$dest\""
        fi
        if scp -o StrictHostKeyChecking=no -i "$SSH_KEY" "$src" "$dest"; then
            return 0
        fi
        retries=$((retries + 1))
        [ $retries -lt $MAX_RETRIES ] && sleep "$RETRY_DELAY"
    done
    return 1
}

# SSH with retries
ssh_cmd() {
    local host="$1" cmd="$2" retries=0
    while [ $retries -lt $MAX_RETRIES ]; do
        if [ "$DRY_RUN" -eq 1 ]; then
            echo "DRY-RUN: ssh -o StrictHostKeyChecking=no -i \"$SSH_KEY\" \"$SSH_USER@$host\" \"$cmd\""
            return 0
        fi
        if [ "$VERBOSE" -eq 1 ]; then
            echo "+ ssh -o StrictHostKeyChecking=no -i \"$SSH_KEY\" \"$SSH_USER@$host\" \"$cmd\""
        fi
        if ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "$SSH_USER@$host" "$cmd"; then
            return 0
        fi
        retries=$((retries + 1))
        [ $retries -lt $MAX_RETRIES ] && sleep "$RETRY_DELAY"
    done
    return 1
}

# Health check with retries
check_health() {
    local host="$1" port="$2" endpoint="$3" retries=0
    while [ $retries -lt $MAX_RETRIES ]; do
        if [ "$DRY_RUN" -eq 1 ]; then
            echo "DRY-RUN: curl -sS http://$host:$port$endpoint"
            return 0
        fi
        if [ "$VERBOSE" -eq 1 ]; then
            echo "+ curl -sS http://$host:$port$endpoint"
        fi
        if curl -sS "http://$host:$port$endpoint" >/dev/null 2>&1; then
            return 0
        fi
        retries=$((retries + 1))
        [ $retries -lt $MAX_RETRIES ] && sleep "$RETRY_DELAY"
    done
    return 1
}

# Deploy to a single host
deploy_host() {
    local host="$1"
    echo "-------------------------------------"
    echo "Deploying to $host..."
    echo "-------------------------------------"

    # Create .nsm directory structure
    echo "Creating .nsm directory structure..."
    ssh_cmd "$host" "mkdir -p $REMOTE_NSM_DIR $REMOTE_RESOURCES_DIR" || return 1
    
    # Copy binary
    echo "Copying binary $TARGET_BINARY to $REMOTE_NSM_DIR ..."
    scp_cmd "$SOURCE_DIR/$TARGET_BINARY" "$SSH_USER@$host:$REMOTE_NSM_DIR/$TARGET_BINARY" || return 1
    ssh_cmd "$host" "chmod +x $REMOTE_NSM_DIR/$TARGET_BINARY" || return 1

    # Copy web templates and resources
    echo "Copying web templates and resources..."
    ssh_cmd "$host" "mkdir -p $REMOTE_RESOURCES_DIR" || return 1
    for template in internal/web/{home-view,host-view,layout}.html; do
        scp_cmd "$template" "$SSH_USER@$host:$REMOTE_RESOURCES_DIR/" || return 1
    done
    scp_cmd "internal/web/static/htmx.min.js" "$SSH_USER@$host:$REMOTE_RESOURCES_DIR/" || return 1

    # Copy key and hosts files
    echo "Copying key and hosts files..."
    if [ -f "nsm_key.pem" ]; then
        scp_cmd "nsm_key.pem" "$SSH_USER@$host:$REMOTE_NSM_DIR/" || return 1
        ssh_cmd "$host" "chmod 600 $REMOTE_NSM_DIR/nsm_key.pem" || return 1
    fi
    if [ -f "test-hosts.json" ]; then
        scp_cmd "test-hosts.json" "$SSH_USER@$host:$REMOTE_NSM_DIR/" || return 1
    fi

    # Handle systemd service if requested
    if [ "$WITH_SERVICE" -eq 1 ] && [ -n "$SERVICE_FILE" ]; then
        echo "Installing systemd service from $SERVICE_FILE..."
        scp_cmd "$SERVICE_FILE" "$SSH_USER@$host:/tmp/$REMOTE_SERVICE_NAME" || return 1
        
        # Stop service if running
        ssh_cmd "$host" "sudo systemctl stop $REMOTE_SERVICE_NAME || true" || return 1
        
        # Install and start service
        ssh_cmd "$host" "sudo mv /tmp/$REMOTE_SERVICE_NAME $REMOTE_SYSTEMD_DIR/$REMOTE_SERVICE_NAME && sudo systemctl daemon-reload && sudo systemctl enable $REMOTE_SERVICE_NAME && sudo systemctl start $REMOTE_SERVICE_NAME" || return 1

        # Check service status
        if [ "$SMOKE_TEST" -eq 1 ]; then
            echo "Checking service status..."
            ssh_cmd "$host" "sudo systemctl is-active $REMOTE_SERVICE_NAME" || return 1
            echo "Checking service logs..."
            ssh_cmd "$host" "sudo journalctl -u $REMOTE_SERVICE_NAME -n 10 --no-pager" || return 1
        fi
    fi

    # Run health checks if requested
    if [ "$SMOKE_TEST" -eq 1 ]; then
        echo "Running NSM health check..."
        check_health "$host" "$NSM_PORT" "/ping" || return 1
    fi

    if [ "$MCP_CHECK" -eq 1 ]; then
        echo "Running MCP initialize check..."
        local data='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
        if [ "$DRY_RUN" -eq 1 ]; then
            echo "DRY-RUN: curl -sS -X POST http://$host:$MCP_PORT/rpc -H Content-Type: application/json -d $data"
        else
            curl -sS -X POST "http://$host:$MCP_PORT/rpc" \
                -H "Content-Type: application/json" \
                -d "$data" >/dev/null || return 1
        fi
    fi

    echo "✅ Deployment to $host successful."
    return 0
}

# Build the binary
echo "Building nsm binary..."
# If Go is not installed, attempt one-time installation (best-effort).
if ! command -v go >/dev/null 2>&1; then
    echo "Go toolchain not found."
    if [ "$DRY_RUN" -eq 1 ]; then
        echo "DRY-RUN: would attempt to install go (AUTO_INSTALL_GO=$AUTO_INSTALL_GO)"
    else
        if [ "$AUTO_INSTALL_GO" -eq 1 ]; then
            echo "Attempting to install Go automatically..."
            if command -v apt-get >/dev/null 2>&1; then
                run_cmd "sudo apt-get update && sudo apt-get install -y golang-go"
            elif command -v yum >/dev/null 2>&1; then
                run_cmd "sudo yum install -y golang"
            elif command -v apk >/dev/null 2>&1; then
                run_cmd "sudo apk add --no-cache go"
            else
                echo "No supported package manager found. Please install Go manually and re-run."
                exit 1
            fi
        else
            echo "Set AUTO_INSTALL_GO=1 to allow the script to try installing Go automatically."
            exit 1
        fi
    fi
fi
if [ -f "Makefile" ]; then
    run_cmd "make build"
else
    mkdir -p "$SOURCE_DIR"
    run_cmd "go build -o \"$SOURCE_DIR/$TARGET_BINARY\" ./cmd/nsm"
fi

if [ ! -f "$SOURCE_DIR/$TARGET_BINARY" ]; then
    echo "Build failed or binary not found at $SOURCE_DIR/$TARGET_BINARY"
    exit 1
fi
echo "Build successful: $SOURCE_DIR/$TARGET_BINARY created."

# Validate SSH key
if [ ! -f "$SSH_KEY" ]; then
    echo "WARNING: SSH key $SSH_KEY not found. If running for real, ensure SSH_KEY path is correct."
    if [ "$DRY_RUN" -eq 0 ]; then
        exit 1
    fi
fi

if [ "$WITH_SERVICE" -eq 1 ] && [ ! -f "$SERVICE_FILE" ]; then
    echo "ERROR: Service file $SERVICE_FILE not found."
    exit 1
fi

# Deploy to hosts with parallelism
pids=()
failed=0

for host in "${HOSTS[@]}"; do
    # Wait if were at max parallel jobs
    while [ ${#pids[@]} -ge "$PARALLEL" ]; do
        for i in "${!pids[@]}"; do
            if ! kill -0 "${pids[$i]}" 2>/dev/null; then
                wait "${pids[$i]}" || failed=1
                unset "pids[$i]"
            fi
        done
        pids=("${pids[@]}")  # Re-index array
        [ ${#pids[@]} -ge "$PARALLEL" ] && sleep 1
    done

    # Start new deploy in background
    if [ "$DRY_RUN" -eq 1 ]; then
        deploy_host "$host"
    else
        deploy_host "$host" &
        pids+=($!)
    fi
done

# Wait for remaining jobs
for pid in "${pids[@]}"; do
    wait "$pid" || failed=1
done

if [ $failed -eq 0 ]; then
    echo "🎉 All deployments complete."
    exit 0
else
    echo "❌ Some deployments failed."
    exit 1
fi
