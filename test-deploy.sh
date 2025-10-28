#!/usr/bin/env bash

# This is a deployment script for the nexSign mini (nsm) service.
# It handles building the binary, copying all required files to remote hosts,
# and optionally setting up systemd services.
#
# Key features:
# - Parallel deployment to multiple hosts
# - Automatic retry on network failures
# - systemd service installation
# - Health checks after deployment
# - Dry-run mode for testing
# - Verbose logging
# - MCP (Model Context Protocol) validation

# Exit immediately if a command exits with a non-zero status.
# -e: exit on error
# -u: treat unset variables as errors
# -o pipefail: if any command in a pipe fails, the pipe's exit status is failure
set -euo pipefail

# --- Configuration Options ---
# DRY_RUN: If 1, print commands without executing them
DRY_RUN=0
# VERBOSE: If 1, print detailed execution logs
VERBOSE=0
# WITH_SERVICE: If 1, install and configure systemd service
WITH_SERVICE=0
# PARALLEL: Number of hosts to deploy to simultaneously
PARALLEL=1
# SMOKE_TEST: If 1, run basic health checks after deployment
SMOKE_TEST=0
# MCP_CHECK: If 1, verify MCP server initialization
MCP_CHECK=0
# MCP_PORT: Port for Model Context Protocol server
MCP_PORT=4000
# NSM_PORT: Port for nexSign mini service
NSM_PORT=8080
# SERVICE_FILE: Path to systemd service template
SERVICE_FILE=""
# AUTO_INSTALL_GO: If 1, attempt to install Go if not found
AUTO_INSTALL_GO=1
# MAX_RETRIES: Maximum retry attempts for network operations
MAX_RETRIES=3
# RETRY_DELAY: Seconds to wait between retries
RETRY_DELAY=2
# WITH_TENDERMINT: If 1, deploy and configure Tendermint consensus
WITH_TENDERMINT=0
# TENDERMINT_VERSION: Version of Tendermint to install
TENDERMINT_VERSION="0.35.9"
# TENDERMINT_HOME: Remote Tendermint data directory
TENDERMINT_HOME="/home/nsm/.tendermint"

usage() {
    cat <<EOF
Usage: $0 [options]

Options:
  --dry-run              Print commands without executing
  --verbose              Print verbose logs
  --with-service FILE    Install and start systemd service using FILE
  --with-tendermint      Deploy and configure Tendermint consensus
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
        --with-tendermint) WITH_TENDERMINT=1; shift ;;
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

# --- Path Configuration ---
# SOURCE_DIR: Local directory containing the built binary
SOURCE_DIR="./bin"
# TARGET_BINARY: Name of the nsm executable
TARGET_BINARY="nsm"
# REMOTE_NSM_DIR: Remote installation directory for nsm
REMOTE_NSM_DIR="/home/nsm/.nsm"
# REMOTE_RESOURCES_DIR: Remote directory for web templates and static assets
REMOTE_RESOURCES_DIR="/home/nsm/.nsm/web"
# REMOTE_SERVICE_NAME: Name of the systemd service file
REMOTE_SERVICE_NAME="nsm.service"
# REMOTE_SYSTEMD_DIR: Standard systemd service directory
REMOTE_SYSTEMD_DIR="/etc/systemd/system"

# Helper function to execute shell commands with dry-run and verbose support
# This function wraps command execution to support:
# - Dry run mode: only print commands without executing
# - Verbose mode: print commands before executing
# - Actual command execution via eval
#
# Usage: run_cmd "command string"
# Example: run_cmd "make build"
run_cmd() {
    if [ "$DRY_RUN" -eq 1 ]; then
        # In dry-run mode, just print what would be executed
        echo "DRY-RUN: $*"
        return 0
    fi
    if [ "$VERBOSE" -eq 1 ]; then
        # In verbose mode, print the command before execution
        echo "+ $*"
    fi
    # Execute the command using eval to handle complex commands with pipes/redirects
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


# Helper function to copy files to remote hosts with automatic retry
# Implements a robust file copy mechanism that:
# - Uses rsync with retry logic for network resilience
# - Supports compression and partial transfers
# - Supports dry-run and verbose modes
# - Disables strict host key checking for automated deployments
# - Uses specified SSH key for authentication
#
# Parameters:
#   $1: Source file path (local)
#   $2: Destination (in user@host:/path format)
#
# Returns:
#   0 on success, 1 if all retries failed
#
# Example: scp_cmd "local/file.txt" "user@host:/remote/path/file.txt"
scp_cmd() {
    local src="$1" dest="$2" retries=0
    while [ $retries -lt $MAX_RETRIES ]; do
        if [ "$DRY_RUN" -eq 1 ]; then
            echo "DRY-RUN: rsync -az --partial -e \"ssh -o StrictHostKeyChecking=no -i $SSH_KEY\" \"$src\" \"$dest\""
            return 0
        fi
        if [ "$VERBOSE" -eq 1 ]; then
            echo "+ rsync -az --partial -e \"ssh -o StrictHostKeyChecking=no -i $SSH_KEY\" \"$src\" \"$dest\""
        fi
        if rsync -az --partial -e "ssh -o StrictHostKeyChecking=no -i $SSH_KEY" "$src" "$dest"; then
            return 0
        fi
        retries=$((retries + 1))
        [ $retries -lt $MAX_RETRIES ] && sleep "$RETRY_DELAY"
    done
    return 1
}

# Helper function to execute commands on remote hosts with automatic retry
# Implements a robust remote command execution that:
# - Uses SSH with retry logic for network resilience
# - Supports dry-run and verbose modes
# - Disables strict host key checking for automated deployments
# - Uses specified SSH key for authentication
#
# Parameters:
#   $1: Target hostname or IP
#   $2: Command to execute on remote host
#
# Environment variables used:
#   SSH_KEY: Path to SSH private key
#   SSH_USER: Remote username
#
# Returns:
#   0 on success, 1 if all retries failed
#
# Example: ssh_cmd "host.example.com" "systemctl restart nsm"
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

# Helper function to verify service health via HTTP endpoints
# Implements health checking that:
# - Tests HTTP endpoints with retry logic
# - Supports both NSM service and MCP endpoints
# - Handles temporary network/startup delays
# - Supports dry-run and verbose modes
#
# Parameters:
#   $1: Target hostname or IP
#   $2: Port number to check
#   $3: Endpoint path (e.g., "/ping", "/health")
#
# Returns:
#   0 if endpoint responds successfully
#   1 if health check fails after all retries
#
# Example: check_health "host.example.com" "8080" "/ping"
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
        # Silent curl with no output - we only care about the HTTP status
        if curl -sS "http://$host:$port$endpoint" >/dev/null 2>&1; then
            return 0
        fi
        retries=$((retries + 1))
        [ $retries -lt $MAX_RETRIES ] && sleep "$RETRY_DELAY"
    done
    return 1
}

# Helper function to install Tendermint on a remote host
# Copies the local Tendermint binary to the remote host.
#
# Parameters:
#   $1: Target hostname or IP
#
# Returns:
#   0 on success, 1 on failure
install_tendermint() {
    local host="$1"
    echo "Installing Tendermint $TENDERMINT_VERSION on $host..."
    
    # Check if local Tendermint binary exists
    if [ ! -f "$SOURCE_DIR/tendermint" ]; then
        echo "ERROR: Local Tendermint binary not found at $SOURCE_DIR/tendermint"
        echo "Run: go install github.com/tendermint/tendermint/cmd/tendermint@v${TENDERMINT_VERSION} && cp ~/go/bin/tendermint $SOURCE_DIR/"
        return 1
    fi
    
    # Check if Tendermint is already installed and up to date
    if ssh_cmd "$host" "command -v tendermint >/dev/null 2>&1"; then
        local installed_version=$(ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "$SSH_USER@$host" "tendermint version 2>&1 | head -1")
        echo "Tendermint already installed: $installed_version"
        # Could add version comparison here if needed
    fi
    
    # Copy Tendermint binary to remote host
    echo "Copying Tendermint binary..."
    scp_cmd "$SOURCE_DIR/tendermint" "$SSH_USER@$host:/tmp/tendermint" || return 1
    
    # Install to /usr/local/bin with sudo, or to ~/go/bin without
    if ssh_cmd "$host" "sudo mv /tmp/tendermint /usr/local/bin/tendermint && sudo chmod +x /usr/local/bin/tendermint 2>/dev/null"; then
        echo "Tendermint installed to /usr/local/bin/"
    else
        echo "No sudo access, installing to ~/go/bin/"
        ssh_cmd "$host" "mkdir -p ~/go/bin && mv /tmp/tendermint ~/go/bin/tendermint && chmod +x ~/go/bin/tendermint" || return 1
        ssh_cmd "$host" "grep -q 'export PATH=\$PATH:~/go/bin' ~/.bashrc || echo 'export PATH=\$PATH:~/go/bin' >> ~/.bashrc"
    fi
    
    # Verify installation
    local version=$(ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "$SSH_USER@$host" "tendermint version 2>&1 || ~/go/bin/tendermint version 2>&1" | head -1)
    echo "Tendermint installed successfully: $version"
    return 0
}

# Helper function to initialize Tendermint on a remote host
# Creates the Tendermint configuration, genesis, and validator keys.
#
# Parameters:
#   $1: Target hostname or IP
#
# Returns:
#   0 on success, 1 on failure
init_tendermint() {
    local host="$1"
    echo "Initializing Tendermint on $host..."
    
    # Initialize Tendermint (creates config, genesis, keys)
    # Note: Tendermint v0.35.9 requires specifying node type (validator, full, or seed)
    ssh_cmd "$host" "tendermint init validator --home $TENDERMINT_HOME" || return 1
    
    # Get node ID for this host
    local node_id=$(ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "$SSH_USER@$host" "tendermint show-node-id --home $TENDERMINT_HOME")
    echo "Node ID for $host: $node_id"
    
    # Store node_id globally for peer configuration
    echo "$node_id" > "/tmp/node_id_${host}"
    
    return 0
}

# Main deployment function for installing NSM on a single host
# This function handles the complete deployment process including:
# - Creating directory structure
# - Copying binary and web resources
# - Installing identity keys and configuration
# - Setting up systemd service (optional)
# - Running health checks (optional)
#
# The deployment follows this sequence:
# 1. Create directories
# 2. Copy and configure binary
# 3. Copy web UI templates
# 4. Install identity key and hosts file
# 5. Configure systemd (if requested)
# 6. Verify deployment (if smoke tests enabled)
#
# Parameters:
#   $1: Target hostname or IP
#
# Returns:
#   0 on successful deployment
#   1 if any step fails
#
# Example: deploy_host "host.example.com"
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

    # Install node identity and configuration files
    # The nsm_key.pem file is the node's cryptographic identity:
    # - Used for signing transactions in the distributed ledger
    # - Must have strict permissions (600) for security
    # - Each node needs its own unique key
    #
    # The test-hosts.json file contains the initial node discovery list:
    # - Provides bootstrap peer information
    # - Used until automatic peer discovery is active
    echo "Copying key and hosts files..."
    if [ -f "nsm_key.pem" ]; then
        scp_cmd "nsm_key.pem" "$SSH_USER@$host:$REMOTE_NSM_DIR/" || return 1
        # Ensure strict permissions on private key file
        ssh_cmd "$host" "chmod 600 $REMOTE_NSM_DIR/nsm_key.pem" || return 1
    fi
    if [ -f "test-hosts.json" ]; then
        scp_cmd "test-hosts.json" "$SSH_USER@$host:$REMOTE_NSM_DIR/" || return 1
    fi

    # --- Tendermint Installation and Configuration ---
    # Install and configure Tendermint for consensus if enabled
    if [ "$WITH_TENDERMINT" -eq 1 ]; then
        echo "Setting up Tendermint on $host..."
        install_tendermint "$host" || return 1
        init_tendermint "$host" || return 1
    fi

    # --- Systemd Service Management ---
    # This section handles the systemd service installation and activation:
    # 1. Copy service definition to temporary location
    # 2. Stop existing service (if running)
    # 3. Install service file and reload systemd
    # 4. Enable service for automatic start on boot
    # 5. Start the service
    #
    # The service installation requires sudo access and manages:
    # - Service definition installation
    # - Service activation state
    # - System-wide service configuration
    if [ "$WITH_SERVICE" -eq 1 ] && [ -n "$SERVICE_FILE" ]; then
        echo "Installing systemd service from $SERVICE_FILE..."
        # Stage service file in temporary location
        scp_cmd "$SERVICE_FILE" "$SSH_USER@$host:/tmp/$REMOTE_SERVICE_NAME" || return 1
        
        # Gracefully stop existing service if running
        # '|| true' prevents failure if service doesn't exist
        ssh_cmd "$host" "sudo systemctl stop $REMOTE_SERVICE_NAME || true" || return 1
        
        # Install and configure service:
        # 1. Move service file to system directory
        # 2. Reload systemd to recognize new service
        # 3. Enable service for automatic start
        # 4. Start the service immediately
        ssh_cmd "$host" "sudo mv /tmp/$REMOTE_SERVICE_NAME $REMOTE_SYSTEMD_DIR/$REMOTE_SERVICE_NAME && sudo systemctl daemon-reload && sudo systemctl enable $REMOTE_SERVICE_NAME && sudo systemctl start $REMOTE_SERVICE_NAME" || return 1

        # Check service status
        if [ "$SMOKE_TEST" -eq 1 ]; then
            echo "Checking service status..."
            ssh_cmd "$host" "sudo systemctl is-active $REMOTE_SERVICE_NAME" || return 1
            echo "Checking service logs..."
            ssh_cmd "$host" "sudo journalctl -u $REMOTE_SERVICE_NAME -n 10 --no-pager" || return 1
        fi
    fi

    # --- Post-Deployment Health Verification ---
    # This section performs various health checks after deployment:
    # 1. NSM Service Health: Basic HTTP endpoint check
    # 2. MCP Service Check: Verify Model Context Protocol initialization
    #
    # The health checks ensure:
    # - Services are running and responsive
    # - Required ports are accessible
    # - Core functionality is working
    
    # Check NSM service health if smoke tests are enabled
    if [ "$SMOKE_TEST" -eq 1 ]; then
        echo "Running NSM health check..."
        # Verify the /ping endpoint responds
        check_health "$host" "$NSM_PORT" "/ping" || return 1
    fi

    # Verify MCP server initialization if enabled
    if [ "$MCP_CHECK" -eq 1 ]; then
        echo "Running MCP initialize check..."
        # Prepare JSON-RPC initialize request
        local data='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
        if [ "$DRY_RUN" -eq 1 ]; then
            echo "DRY-RUN: curl -sS -X POST http://$host:$MCP_PORT/rpc -H Content-Type: application/json -d $data"
        else
            # Send initialize request to MCP endpoint
            # Redirecting output to /dev/null - we only care about HTTP status
            curl -sS -X POST "http://$host:$MCP_PORT/rpc" \
                -H "Content-Type: application/json" \
                -d "$data" >/dev/null || return 1
        fi
    fi

    echo "✅ Deployment to $host successful."
    return 0
}

# --- Build Process ---
# This section handles the NSM binary build process:
# 1. Ensures Go toolchain is available
# 2. Installs Go if missing (when AUTO_INSTALL_GO=1)
# 3. Builds the binary using make or direct go build
#
# The build process supports multiple Linux distributions:
# - Debian/Ubuntu (apt-get)
# - RHEL/CentOS (yum)
# - Alpine Linux (apk)
#
# Environment variables used:
#   AUTO_INSTALL_GO: Controls automatic Go installation
#   SOURCE_DIR: Output directory for built binary
#   TARGET_BINARY: Name of the output binary

echo "Building nsm binary..."
# Check if Go is installed, attempt installation if missing
if ! command -v go >/dev/null 2>&1; then
    echo "Go toolchain not found."
    if [ "$DRY_RUN" -eq 1 ]; then
        echo "DRY-RUN: would attempt to install go (AUTO_INSTALL_GO=$AUTO_INSTALL_GO)"
    else
        if [ "$AUTO_INSTALL_GO" -eq 1 ]; then
            echo "Attempting to install Go automatically..."
            # Try different package managers based on the distribution
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

# --- Parallel Deployment Management ---
# This section implements parallel deployment to multiple hosts:
# - Manages a pool of background deployment processes
# - Enforces maximum parallel deployments limit
# - Tracks deployment success/failure
# - Provides clean process management and cleanup
#
# The parallel deployment system:
# 1. Maintains an array of active deployment process IDs
# 2. Limits concurrent deployments to PARALLEL count
# 3. Monitors for completed deployments
# 4. Tracks overall success/failure status
#
# Variables:
#   pids[]: Array of active deployment process IDs
#   failed: Tracks if any deployment has failed (0=success, 1=failure)
#   PARALLEL: Maximum number of concurrent deployments

# Initialize parallel deployment tracking
pids=()  # Array to track background process IDs
failed=0 # Flag to track if any deployment fails

# Process each host in the HOSTS array
for host in "${HOSTS[@]}"; do
    # Wait loop: Block if we're at max parallel jobs
    while [ ${#pids[@]} -ge "$PARALLEL" ]; do
        # Check each running deployment
        for i in "${!pids[@]}"; do
            # Test if process is still running (kill -0 only tests process existence)
            if ! kill -0 "${pids[$i]}" 2>/dev/null; then
                # Process completed - check exit status and clean up
                wait "${pids[$i]}" || failed=1
                unset "pids[$i]"
            fi
        done
        # Re-index array after removing completed processes
        pids=("${pids[@]}")  
        # If still at capacity, wait before checking again
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

# --- Final Deployment Cleanup and Status ---
# This section:
# 1. Waits for all remaining background deployments
# 2. Collects all exit statuses
# 3. Provides final deployment summary
# 4. Sets appropriate script exit code
#
# Exit codes:
# - 0: All deployments successful
# - 1: One or more deployments failed

# Wait for any remaining background deployment processes
for pid in "${pids[@]}"; do
    # wait returns the exit status of the process
    wait "$pid" || failed=1
done

# --- Tendermint Post-Deployment Configuration ---
# After all nodes are deployed, configure Tendermint peer connections
# and start Tendermint on each node
if [ "$WITH_TENDERMINT" -eq 1 ] && [ $failed -eq 0 ]; then
    echo "========================================="
    echo "Configuring Tendermint peer connections..."
    echo "========================================="
    
    # Build persistent_peers list from all nodes
    persistent_peers=""
    for i in "${!HOSTS[@]}"; do
        host="${HOSTS[$i]}"
        if [ -f "/tmp/node_id_${host}" ]; then
            node_id=$(cat "/tmp/node_id_${host}")
            if [ -n "$persistent_peers" ]; then
                persistent_peers="${persistent_peers},"
            fi
            persistent_peers="${persistent_peers}${node_id}@${host}:26656"
        fi
    done
    
    echo "Persistent peers: $persistent_peers"
    
    # Copy genesis from first node to all other nodes (must be identical)
    echo "Synchronizing genesis.json across all nodes..."
    first_host="${HOSTS[0]}"
    
    # Download genesis from first node
    scp_cmd "$SSH_USER@$first_host:$TENDERMINT_HOME/config/genesis.json" "/tmp/genesis.json" || failed=1
    
    # Upload to all other nodes
    for host in "${HOSTS[@]:1}"; do
        echo "Copying genesis to $host..."
        scp_cmd "/tmp/genesis.json" "$SSH_USER@$host:$TENDERMINT_HOME/config/genesis.json" || failed=1
    done
    
    # Configure and start Tendermint on each node
    for i in "${!HOSTS[@]}"; do
        host="${HOSTS[$i]}"
        node_num=$((i + 1))
        p2p_port=$((26656 + i * 10))
        rpc_port=$((26657 + i * 10))
        abci_socket="unix:///home/nsm/.nsm/nsm.sock"
        
        echo "Starting Tendermint on $host (node $node_num)..."
        echo "  P2P port: $p2p_port"
        echo "  RPC port: $rpc_port"
        echo "  ABCI socket: $abci_socket"
        
        # Update Tendermint config for unique ports
        ssh_cmd "$host" "sed -i 's/laddr = \"tcp:\\/\\/127.0.0.1:26656\"/laddr = \"tcp:\\/\\/0.0.0.0:$p2p_port\"/' $TENDERMINT_HOME/config/config.toml" || failed=1
        ssh_cmd "$host" "sed -i 's/laddr = \"tcp:\\/\\/127.0.0.1:26657\"/laddr = \"tcp:\\/\\/0.0.0.0:$rpc_port\"/' $TENDERMINT_HOME/config/config.toml" || failed=1
        
        # Set persistent peers (excluding self)
        other_peers=$(echo "$persistent_peers" | sed "s/${node_id}@${host}:${p2p_port}//g" | sed 's/,,/,/g' | sed 's/^,//g' | sed 's/,$//g')
        ssh_cmd "$host" "sed -i 's/persistent_peers = \"\"/persistent_peers = \"$other_peers\"/' $TENDERMINT_HOME/config/config.toml" || failed=1
        
        # Start Tendermint in background (using screen or nohup)
        # Note: Tendermint v0.35.x uses --proxy-app (hyphen) not --proxy_app (underscore)
        echo "Starting Tendermint node..."
        ssh_cmd "$host" "nohup tendermint node --home $TENDERMINT_HOME --proxy-app=$abci_socket --p2p.laddr tcp://0.0.0.0:$p2p_port --rpc.laddr tcp://0.0.0.0:$rpc_port > $TENDERMINT_HOME/tendermint.log 2>&1 &" || failed=1
        
        sleep 2
    done
    
    echo "Tendermint configuration complete."
    echo "To check Tendermint status on a node:"
    echo "  curl http://\${HOST}:26657/status"
    echo ""
    echo "To view Tendermint logs on a node:"
    echo "  ssh $SSH_USER@\${HOST} tail -f $TENDERMINT_HOME/tendermint.log"
fi

# Provide final status and exit appropriately
if [ $failed -eq 0 ]; then
    echo "🎉 All deployments complete."
    exit 0
else
    echo "❌ Some deployments failed."
    exit 1
fi
