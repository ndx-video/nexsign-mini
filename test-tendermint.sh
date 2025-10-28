#!/bin/bash
# test-tendermint.sh - Quick test script for Tendermint integration
#
# This script helps test the socket-based ABCI integration between nsm and Tendermint.
# It can run in either single-node or multi-node mode.
#
# Usage:
#   ./test-tendermint.sh single       # Test single node
#   ./test-tendermint.sh multi 3      # Test 3-node cluster
#   ./test-tendermint.sh clean        # Clean up all test data

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Tendermint version to check for
TM_VERSION="0.35"

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Tendermint is installed
check_tendermint() {
    if ! command -v tendermint &> /dev/null; then
        log_error "Tendermint is not installed!"
        echo ""
        echo "Install with:"
        echo "  wget https://github.com/tendermint/tendermint/releases/download/v0.35.9/tendermint_0.35.9_linux_amd64.tar.gz"
        echo "  tar -xzf tendermint_0.35.9_linux_amd64.tar.gz"
        echo "  sudo mv tendermint /usr/local/bin/"
        exit 1
    fi
    
    local version=$(tendermint version 2>&1 | head -1)
    if [[ ! "$version" =~ $TM_VERSION ]]; then
        log_warn "Tendermint version mismatch. Expected $TM_VERSION, got: $version"
        echo "Continue anyway? (y/n)"
        read -r response
        if [[ ! "$response" =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
    
    log_info "Tendermint version: $version"
}

# Clean up test data
clean_test_data() {
    log_info "Cleaning up test data..."
    
    # Remove Tendermint directories
    for i in 1 2 3; do
        rm -rf ~/.tendermint$i
    done
    rm -rf ~/.tendermint
    
    # Remove socket files
    rm -f nsm*.sock
    
    # Remove log files
    rm -f nsm*.log tendermint*.log
    
    log_info "Cleanup complete"
}

# Initialize single node
init_single_node() {
    log_info "Initializing single-node Tendermint..."
    
    tendermint init --home ~/.tendermint
    
    log_info "Tendermint initialized at ~/.tendermint"
    log_info "Node ID: $(tendermint show-node-id --home ~/.tendermint)"
}

# Initialize multi-node cluster
init_multi_node() {
    local num_nodes=$1
    
    log_info "Initializing $num_nodes-node Tendermint cluster..."
    
    for i in $(seq 1 $num_nodes); do
        local home="$HOME/.tendermint$i"
        log_info "Initializing node $i at $home..."
        tendermint init --home "$home"
        
        local node_id=$(tendermint show-node-id --home "$home")
        log_info "Node $i ID: $node_id"
    done
    
    # Copy genesis from node 1 to all other nodes
    log_info "Copying genesis.json from node 1 to all nodes..."
    for i in $(seq 2 $num_nodes); do
        cp ~/.tendermint1/config/genesis.json ~/.tendermint$i/config/
    done
    
    log_info "Multi-node initialization complete"
}

# Run single node test
run_single_node() {
    log_info "Starting single-node test..."
    log_info ""
    log_info "Step 1: Initialize Tendermint"
    init_single_node
    
    log_info ""
    log_info "Step 2: Start nsm in this terminal"
    log_info "Command: go run cmd/nsm/main.go"
    log_info ""
    log_info "Step 3: In another terminal, start Tendermint"
    log_info "Command: tendermint node --proxy_app=unix://nsm.sock"
    log_info ""
    log_info "Step 4: Verify connection"
    log_info "Check nsm logs for ABCI method calls (CheckTx, BeginBlock, etc.)"
    log_info "Check Tendermint logs for 'Starting ABCI with Tendermint'"
    log_info ""
    log_info "Step 5: Test transaction"
    log_info "Open http://localhost:8080 and trigger a host action"
    log_info ""
    log_info "Step 6: Verify state consistency"
    log_info "curl http://localhost:26657/abci_info"
    log_info "curl http://localhost:26657/status"
    log_info ""
}

# Run multi-node test
run_multi_node() {
    local num_nodes=$1
    
    log_info "Starting $num_nodes-node test..."
    log_info ""
    log_info "Step 1: Initialize nodes"
    init_multi_node $num_nodes
    
    log_info ""
    log_info "Step 2: Start all nodes (each in separate terminal)"
    log_info ""
    
    for i in $(seq 1 $num_nodes); do
        local port=$((8080 + i - 1))
        local socket="nsm$i.sock"
        local tm_home="$HOME/.tendermint$i"
        local p2p_port=$((26656 + (i-1)*10))
        local rpc_port=$((26657 + (i-1)*10))
        
        log_info "Node $i commands:"
        log_info "  Terminal ${i}A (nsm):"
        log_info "    PORT=$port ABCI_SOCKET_PATH=$socket go run cmd/nsm/main.go"
        log_info ""
        log_info "  Terminal ${i}B (Tendermint):"
        
        if [ $i -eq 1 ]; then
            log_info "    tendermint node --home $tm_home --proxy_app=unix://$socket \\"
            log_info "      --p2p.laddr tcp://0.0.0.0:$p2p_port \\"
            log_info "      --rpc.laddr tcp://0.0.0.0:$rpc_port"
        else
            local node1_id=$(tendermint show-node-id --home ~/.tendermint1)
            log_info "    tendermint node --home $tm_home --proxy_app=unix://$socket \\"
            log_info "      --p2p.laddr tcp://0.0.0.0:$p2p_port \\"
            log_info "      --rpc.laddr tcp://0.0.0.0:$rpc_port \\"
            log_info "      --p2p.persistent_peers=\"$node1_id@localhost:26656\""
        fi
        log_info ""
    done
    
    log_info "Step 3: Verify connections"
    for i in $(seq 1 $num_nodes); do
        local rpc_port=$((26657 + (i-1)*10))
        log_info "  Node $i: curl http://localhost:$rpc_port/status"
    done
    log_info ""
    
    log_info "Step 4: Test consensus"
    log_info "  1. Broadcast transaction from node 1: http://localhost:8080"
    log_info "  2. Verify all nodes see the transaction in their logs"
    log_info "  3. Check state consistency across all nodes"
    log_info ""
}

# Print usage help
usage() {
    echo "Usage: $0 <command> [options]"
    echo ""
    echo "Commands:"
    echo "  single              Test single-node setup"
    echo "  multi <N>           Test N-node cluster (default: 3)"
    echo "  clean               Clean up all test data"
    echo "  check               Check if Tendermint is installed"
    echo ""
    echo "Examples:"
    echo "  $0 single           # Test single node"
    echo "  $0 multi 3          # Test 3-node cluster"
    echo "  $0 clean            # Clean up test data"
    echo ""
}

# Main script
main() {
    local command=${1:-help}
    
    case $command in
        single)
            check_tendermint
            run_single_node
            ;;
        multi)
            local num_nodes=${2:-3}
            check_tendermint
            run_multi_node $num_nodes
            ;;
        clean)
            clean_test_data
            ;;
        check)
            check_tendermint
            ;;
        help|--help|-h)
            usage
            ;;
        *)
            log_error "Unknown command: $command"
            echo ""
            usage
            exit 1
            ;;
    esac
}

# Run main with all arguments
main "$@"
