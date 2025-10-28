# Multi-Node Tendermint Deployment Guide

## Overview

The `test-deploy.sh` script now supports full multi-node Tendermint consensus deployment. It will:

1. Build and deploy nsm binary to all nodes
2. Install Tendermint v0.35.9 on each node
3. Initialize Tendermint with unique validator keys
4. Synchronize genesis.json across all nodes
5. Configure peer connections automatically
6. Start both nsm (ABCI server) and Tendermint on each node

## Prerequisites

1. **Update `deploy.env`** with your VM IPs:
   ```bash
   HOSTS=("192.168.10.147" "192.168.10.174" "192.168.10.135" "192.168.10.211")
   SSH_USER="nsm"
   SSH_KEY="/path/to/your/ssh/key"
   ```

2. **Ensure VMs are accessible**:
   ```bash
   for host in "${HOSTS[@]}"; do ssh -i $SSH_KEY $SSH_USER@$host echo "OK"; done
   ```

3. **Build nsm binary** (or let script do it):
   ```bash
   go build -o bin/nsm cmd/nsm/main.go
   ```

## Deployment Commands

### Basic Deployment (No Tendermint)
Deploy nsm only without consensus:
```bash
./test-deploy.sh --verbose
```

### Full Multi-Node Tendermint Deployment
Deploy nsm + Tendermint with automatic configuration:
```bash
./test-deploy.sh --verbose --with-tendermint
```

### Deployment with Systemd Service
Install systemd service and start automatically:
```bash
./test-deploy.sh --verbose --with-tendermint --with-service nsm.service
```

### Dry Run (Test Without Executing)
See what would be done without actually doing it:
```bash
./test-deploy.sh --dry-run --with-tendermint
```

### Parallel Deployment
Deploy to multiple nodes simultaneously (default is sequential):
```bash
./test-deploy.sh --verbose --with-tendermint --parallel 4
```

### With Health Checks
Run post-deployment health checks:
```bash
./test-deploy.sh --verbose --with-tendermint --smoke
```

## What Gets Deployed

### On Each Node

**NSM (nexSign mini):**
- Binary: `/home/nsm/.nsm/nsm`
- Key: `/home/nsm/.nsm/nsm_key.pem`
- Hosts file: `/home/nsm/.nsm/test-hosts.json`
- Web templates: `/home/nsm/.nsm/web/*.html`
- ABCI socket: `unix:///home/nsm/.nsm/nsm.sock`
- Web UI: `http://<host>:8080`

**Tendermint:**
- Binary: `/usr/local/bin/tendermint`
- Data: `/home/nsm/.tendermint/`
- Config: `/home/nsm/.tendermint/config/config.toml`
- Genesis: `/home/nsm/.tendermint/config/genesis.json`
- Validator key: `/home/nsm/.tendermint/config/priv_validator_key.json`
- Node key: `/home/nsm/.tendermint/config/node_key.json`
- P2P ports: 26656, 26666, 26676, 26686 (staggered by 10)
- RPC ports: 26657, 26667, 26677, 26687 (staggered by 10)
- Logs: `/home/nsm/.tendermint/tendermint.log`

## Port Assignments

| Node | Host IP | NSM Port | TM P2P Port | TM RPC Port |
|------|---------|----------|-------------|-------------|
| 1 | 192.168.10.147 | 8080 | 26656 | 26657 |
| 2 | 192.168.10.174 | 8080 | 26666 | 26667 |
| 3 | 192.168.10.135 | 8080 | 26676 | 26677 |
| 4 | 192.168.10.211 | 8080 | 26686 | 26687 |

## Verification

### Check NSM Status
```bash
# From your local machine
for host in "${HOSTS[@]}"; do
    echo "=== $host ==="
    curl -s http://$host:8080/ping
done
```

### Check Tendermint Status
```bash
# Node 1
curl -s http://192.168.10.147:26657/status | jq '.result.node_info'

# Node 2
curl -s http://192.168.10.174:26667/status | jq '.result.node_info'

# Node 3
curl -s http://192.168.10.135:26677/status | jq '.result.node_info'

# Node 4
curl -s http://192.168.10.211:26687/status | jq '.result.node_info'
```

### Check Peer Connections
```bash
curl -s http://192.168.10.147:26657/net_info | jq '.result.n_peers'
# Should show 3 peers (other nodes)
```

### View Tendermint Logs
```bash
ssh nsm@192.168.10.147 tail -f /home/nsm/.tendermint/tendermint.log
```

### View NSM Logs
```bash
ssh nsm@192.168.10.147 tail -f /home/nsm/.nsm/nsm.log
```

## Testing Consensus

### 1. Access Dashboard
Open in browser: `http://192.168.10.147:8080`

### 2. Trigger an Action
Use the web UI to perform a host action (e.g., update status)

### 3. Verify Consensus
Check that the transaction appears in logs on all nodes:
```bash
for host in "${HOSTS[@]}"; do
    echo "=== Checking $host ==="
    ssh nsm@$host "grep -i 'DeliverTx' /home/nsm/.tendermint/tendermint.log | tail -5"
done
```

### 4. Check Block Height
All nodes should be at the same block height:
```bash
for host in 192.168.10.{147,174,135,211}; do
    echo -n "$host: "
    curl -s http://$host:26657/status | jq -r '.result.sync_info.latest_block_height'
done
```

## Troubleshooting

### NSM Not Starting
```bash
ssh nsm@192.168.10.147 /home/nsm/.nsm/nsm
# Check for errors in output
```

### Tendermint Not Connecting
1. Check if Tendermint is running:
   ```bash
   ssh nsm@192.168.10.147 ps aux | grep tendermint
   ```

2. Check logs for connection errors:
   ```bash
   ssh nsm@192.168.10.147 tail -100 /home/nsm/.tendermint/tendermint.log | grep -i error
   ```

3. Verify peer configuration:
   ```bash
   ssh nsm@192.168.10.147 cat /home/nsm/.tendermint/config/config.toml | grep persistent_peers
   ```

### Socket Connection Issues
1. Check if ABCI server is running:
   ```bash
   ssh nsm@192.168.10.147 ls -la /home/nsm/.nsm/nsm.sock
   ```

2. Restart nsm first, then Tendermint:
   ```bash
   ssh nsm@192.168.10.147 "pkill nsm; /home/nsm/.nsm/nsm &"
   sleep 2
   ssh nsm@192.168.10.147 "pkill tendermint; cd /home/nsm && nohup tendermint node --home /home/nsm/.tendermint --proxy_app=unix:///home/nsm/.nsm/nsm.sock > /home/nsm/.tendermint/tendermint.log 2>&1 &"
   ```

### Genesis Mismatch
All nodes must have identical genesis.json. If you see genesis hash errors:
```bash
# Copy genesis from node 1 to all others
for host in 192.168.10.{174,135,211}; do
    scp nsm@192.168.10.147:/home/nsm/.tendermint/config/genesis.json /tmp/genesis.json
    scp /tmp/genesis.json nsm@$host:/home/nsm/.tendermint/config/genesis.json
done

# Restart Tendermint on all nodes
for host in 192.168.10.{147,174,135,211}; do
    ssh nsm@$host "pkill tendermint; cd /home/nsm && nohup tendermint node --home /home/nsm/.tendermint --proxy_app=unix:///home/nsm/.nsm/nsm.sock > /home/nsm/.tendermint/tendermint.log 2>&1 &"
done
```

## Clean Slate Redeployment

To completely reset and redeploy:

```bash
# 1. Stop all services
for host in "${HOSTS[@]}"; do
    ssh nsm@$host "pkill nsm; pkill tendermint"
done

# 2. Remove all data
for host in "${HOSTS[@]}"; do
    ssh nsm@$host "rm -rf /home/nsm/.nsm /home/nsm/.tendermint"
done

# 3. Redeploy
./test-deploy.sh --verbose --with-tendermint
```

## Advanced: Manual Tendermint Start

If you need to start Tendermint manually with custom parameters:

```bash
ssh nsm@192.168.10.147

# Start NSM first
cd /home/nsm/.nsm
./nsm &

# Wait for ABCI socket
sleep 2
ls -la nsm.sock  # Should exist

# Start Tendermint
tendermint node \
  --home /home/nsm/.tendermint \
  --proxy_app=unix:///home/nsm/.nsm/nsm.sock \
  --p2p.laddr tcp://0.0.0.0:26656 \
  --rpc.laddr tcp://0.0.0.0:26657 \
  > /home/nsm/.tendermint/tendermint.log 2>&1 &
```

## Next Steps

Once deployment is successful:

1. **Test transaction broadcasting** - Implement `internal/tendermint/broadcast.go`
2. **Wire dashboard actions** - Update web handlers to use consensus
3. **Add Anthias polling** - Implement automated status updates
4. **Monitor performance** - Track block times and transaction throughput
5. **Test failure scenarios** - Stop nodes and verify consensus continues

## References

- [Tendermint Documentation](https://docs.tendermint.com/v0.35/)
- [ABCI Specification](https://docs.tendermint.com/v0.35/spec/abci/)
- [Internal Tendermint README](internal/tendermint/README.md)
- [Tendermint Quick Start](internal/tendermint/QUICKSTART.md)
