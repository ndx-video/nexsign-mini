#!/bin/bash
# Development deployment script for nsm to test VirtualBox hosts
# This script builds nsm locally, copies it to remote hosts, and restarts the service

set -e

# Configuration
HOSTS=("192.168.10.147" "192.168.10.174" "192.168.10.135" "192.168.10.211")
HOST_NAMES=("nsm01" "nsm02" "nsm03" "nsm04")
REMOTE_USER="nsm"  # Change this to match your VirtualBox user
BUILD_DIR="$(pwd)"
BINARY_NAME="nsm"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== NSM Development Deployment ===${NC}"
echo ""

# Build the binary
echo -e "${YELLOW}Building nsm binary...${NC}"
go build -o "${BINARY_NAME}" main.go
if [ $? -ne 0 ]; then
    echo -e "${RED}Build failed!${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Build successful${NC}"
echo ""

# Deploy to each host
for i in "${!HOSTS[@]}"; do
    HOST="${HOSTS[$i]}"
    NAME="${HOST_NAMES[$i]}"
    
    echo -e "${YELLOW}Deploying to ${NAME} (${HOST})...${NC}"
    
    # Test SSH connection
    if ! ssh -o ConnectTimeout=5 "${REMOTE_USER}@${HOST}" "echo 'Connection OK'" &>/dev/null; then
        echo -e "${RED}✗ Cannot connect to ${HOST} - skipping${NC}"
        echo ""
        continue
    fi
    
    # Stop the service if running
    echo "  Stopping nsm service..."
    ssh "${REMOTE_USER}@${HOST}" "sudo systemctl stop nsm.service 2>/dev/null || pkill -f ./nsm || true"
    
    # Copy the binary
    echo "  Copying binary..."
    scp -q "${BINARY_NAME}" "${REMOTE_USER}@${HOST}:/tmp/${BINARY_NAME}"
    
    # Move binary to /usr/local/bin and set permissions
    echo "  Installing binary..."
    ssh "${REMOTE_USER}@${HOST}" "sudo mv /tmp/${BINARY_NAME} /usr/local/bin/${BINARY_NAME} && sudo chmod +x /usr/local/bin/${BINARY_NAME}"
    
    # Start the service
    echo "  Starting nsm service..."
    ssh "${REMOTE_USER}@${HOST}" "sudo systemctl start nsm.service 2>/dev/null || (cd /opt/nsm && nohup /usr/local/bin/nsm > /var/log/nsm.log 2>&1 &)"
    
    # Verify it's running
    sleep 2
    if ssh "${REMOTE_USER}@${HOST}" "curl -s http://localhost:8080/api/health" &>/dev/null; then
        echo -e "${GREEN}✓ ${NAME} deployed successfully${NC}"
    else
        echo -e "${RED}✗ ${NAME} deployed but health check failed${NC}"
    fi
    
    echo ""
done

echo -e "${GREEN}=== Deployment Complete ===${NC}"
echo ""
echo "Test the dashboard at: http://localhost:8080"
echo "Check individual hosts:"
for i in "${!HOSTS[@]}"; do
    echo "  - ${HOST_NAMES[$i]}: http://${HOSTS[$i]}:8080"
done
