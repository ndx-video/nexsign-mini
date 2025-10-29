# Multi-Node Deployment Guide

## Overview

This guide walks through deploying `nexSign mini` to multiple nodes with Tendermint consensus, creating a real distributed ledger where each node broadcasts its identity and status.

## Prerequisites

1. **Target VMs/Hosts**: Linux systems with:
   - SSH access
   - Go 1.21+ installed (or `AUTO_INSTALL_GO=1` in deploy script)
   - Network connectivity between all nodes
   - Ports 8080 (nsm), 26656 (Tendermint P2P), 26657 (Tendermint RPC) open

2. **Local Development Machine**:
   - Go 1.21+
   - SSH key for remote access
   - `deploy.env` configured with target hosts

## Configuration

### 1. Create `deploy.env`

```bash
# Target VMs (use IPs for test environments)
HOSTS=("192.168.10.147" "192.168.10.174" "192.168.10.135" "192.168.10.211")

# SSH credentials
SSH_USER="nsm"
SSH_KEY="/path/to/your/ssh-key.pem"
```

### 2. Ensure Binaries Exist

```bash
# Build nsm
go build -o bin/nsm cmd/nsm/main.go

# Install Tendermint (if not already in bin/)
go install github.com/tendermint/tendermint/cmd/tendermint@v0.35.9
cp ~/go/bin/tendermint bin/tendermint
```

## Quick Deployment

Use the automated script:

```bash
./quick-deploy-test.sh
```

This script:
1. Builds the nsm binary
2. Cleans all remote hosts (stops processes, removes old data)
3. **Generates unique ED25519 identity keys for each node**
4. Deploys keys to each node
5. Runs `test-deploy.sh --with-tendermint` to:
   - Copy nsm and Tendermint binaries
   - Copy web templates
   - Initialize Tendermint on each node
   - Configure peer connections
   - Start both services
6. Checks node status and provides dashboard URLs

## What Happens on Each Node

### 1. Identity Generation

Each node gets a unique `nsm_key.pem` file containing an ED25519 keypair. The public key hex becomes the node's permanent identifier in the ledger.

### 2. Node Startup Sequence

```
nsm starts → Loads identity → Starts ABCI server on unix://nsm.sock
                ↓
Tendermint starts → Connects to ABCI → Begins consensus
                ↓
Anthias poller starts → Detects host info → Broadcasts TxAddHost (commit)
                ↓
                Every 30s: Broadcasts TxUpdateStatus (sync)
```

### 3. Data Flow

```
Node A                    Consensus Layer                Node B
  |                             |                           |
  |-- TxAddHost(A) ------------>|                           |
  |                             |--------TxAddHost(A)------>|
  |                             |                           |
  |                             |<---TxAddHost(B)-----------|
  |<---TxAddHost(B)-------------|                           |
  |                             |                           |
  |-- TxUpdateStatus(A)-------->|                           |
  |                             |-----TxUpdateStatus(A)---->|
```

All nodes see all add_host and update_status transactions, building a complete view of the network.

## Verification

### Check Node Processes

```bash
# On any node
ps aux | grep -E "nsm|tendermint"
```

### Check Consensus

```bash
# Query Tendermint RPC
curl http://192.168.10.147:26657/status

# Check block height is increasing
watch -n 2 'curl -s http://192.168.10.147:26657/status | jq .result.sync_info.latest_block_height'
```

### Check Host Registry

```bash
# Query nsm API
curl http://192.168.10.147:8080/api/hosts | jq

# Should show all nodes with their unique public keys
```

### Check Dashboard

Open any node's dashboard:
- http://192.168.10.147:8080/
- http://192.168.10.174:8080/
- etc.

All dashboards should show the **same list of hosts** (replicated state).

## Expected Output

When fully operational, each dashboard will display:

```
Discovered Hosts
┌────────────┬────────────────┬─────────┬────────┬─────────┐
│ Hostname   │ IP Address     │ Version │ Status │ Actions │
├────────────┼────────────────┼─────────┼────────┼─────────┤
│ node-147   │ 192.168.10.147 │ unknown │ online │   API   │
│ node-174   │ 192.168.10.174 │ unknown │ online │   API   │
│ node-135   │ 192.168.10.135 │ unknown │ online │   API   │
│ node-211   │ 192.168.10.211 │ unknown │ online │   API   │
└────────────┴────────────────┴─────────┴────────┴─────────┘
```

With color-coded health indicators:
- 🟢 Green = online (last seen < 30s)
- 🟡 Yellow = degraded (last seen 30-90s)
- 🔴 Red = offline (last seen > 90s)

## Architecture

### Each Node Runs

1. **nsm service**
   - ABCI server listening on `unix:///home/nsm/.nsm/nsm.sock`
   - Web dashboard on `:8080`
   - Anthias poller (broadcasts status every 30s)

2. **Tendermint Core**
   - Connects to ABCI via socket
   - P2P gossip on `:26656`
   - RPC on `:26657`
   - Consensus algorithm (BFT)

### Data Directory Structure

```
/home/nsm/
  .nsm/
    nsm_key.pem          # Node's unique ED25519 keypair (0600)
    nsm                  # Binary
    nsm.log              # Service logs
    nsm.sock             # ABCI Unix socket
  .tendermint/
    config/
      config.toml        # Tendermint config (ports, peers)
      genesis.json       # Shared genesis (must be identical on all nodes)
    data/
      *.db               # Blockchain data
    tendermint.log       # Consensus logs
```

## Troubleshooting

### Nodes Not Discovering Each Other

**Check:** Tendermint peer connectivity

```bash
# On each node
curl -s http://localhost:26657/net_info | jq .result.n_peers
# Should show (N-1) peers for N-node network
```

**Fix:** Verify `persistent_peers` in Tendermint config includes all other nodes.

### Hosts Not Appearing in Dashboard

**Check:** Anthias poller and ABCI logs

```bash
ssh nsm@192.168.10.147 tail -f /home/nsm/.nsm/nsm.log
```

**Look for:**
- "✓ add_host committed" (initial registration)
- "INFO: Added host to state" (ABCI processing)
- "INFO: Updated status for host" (periodic updates)

**Fix:** Ensure:
1. Node registered itself (`add_host` transaction committed)
2. Tendermint is syncing blocks
3. ABCI connection is healthy

### Permission Denied on nsm_key.pem

**Check:** File permissions

```bash
ssh nsm@192.168.10.147 ls -l /home/nsm/.nsm/nsm_key.pem
# Should show: -rw------- (600)
```

**Fix:**

```bash
ssh nsm@192.168.10.147 chmod 600 /home/nsm/.nsm/nsm_key.pem
```

### Tendermint Not Starting

**Check:** Socket availability

```bash
ssh nsm@192.168.10.147 ls -l /home/nsm/.nsm/nsm.sock
# Should exist while nsm is running
```

**Check:** Tendermint logs

```bash
ssh nsm@192.168.10.147 tail -f /home/nsm/.tendermint/tendermint.log
```

**Common issues:**
- ABCI socket not available (start nsm first)
- Port conflicts (another Tendermint instance running)
- Genesis mismatch (genesis.json must be identical across nodes)

## Manual Deployment Steps

If you prefer manual deployment or need to debug:

### 1. Generate Unique Keys Locally

```bash
# Build key generator
go build -tags manual -o /tmp/generate_key generate_key.go

# Generate unique key for each node
for host in 192.168.10.{147,174,135,211}; do
    /tmp/generate_key /tmp/nsm_key_${host}.pem
done
```

### 2. Deploy Keys

```bash
for host in 192.168.10.{147,174,135,211}; do
    ssh nsm@$host "mkdir -p /home/nsm/.nsm"
    scp /tmp/nsm_key_${host}.pem nsm@$host:/home/nsm/.nsm/nsm_key.pem
    ssh nsm@$host "chmod 600 /home/nsm/.nsm/nsm_key.pem"
done
```

### 3. Deploy Binaries and Tendermint

```bash
./test-deploy.sh --with-tendermint --verbose
```

### 4. Verify

```bash
# Check all nodes show same host count
for host in 192.168.10.{147,174,135,211}; do
    echo "=== $host ==="
    curl -s http://$host:8080/api/hosts | jq '. | length'
done
```

## Viewing Logs

### Real-time Monitoring

```bash
# NSM logs on all nodes (tmux/screen recommended)
for host in 192.168.10.{147,174,135,211}; do
    ssh nsm@$host tail -f /home/nsm/.nsm/nsm.log &
done

# Tendermint logs
for host in 192.168.10.{147,174,135,211}; do
    ssh nsm@$host tail -f /home/nsm/.tendermint/tendermint.log &
done
```

### Aggregated Logs

Use your preferred log aggregation tool (rsyslog, Grafana Loki, etc.) to centralize logs from all nodes.

## Stopping the Network

```bash
# Stop all services
for host in 192.168.10.{147,174,135,211}; do
    ssh nsm@$host "pkill nsm; pkill tendermint"
done
```

## Clean Slate (Reset Everything)

```bash
# WARNING: This deletes all blockchain data and keys
for host in 192.168.10.{147,174,135,211}; do
    ssh nsm@$host "pkill -9 nsm; pkill -9 tendermint; rm -rf /home/nsm/.nsm /home/nsm/.tendermint"
done
```

Then re-run `./quick-deploy-test.sh` to start fresh.

---

## Next Steps

1. **Add Real Anthias Integration**: Update `internal/anthias/client.go` to query actual Anthias API
2. **Persistent State**: Add database backend so ledger survives restarts
3. **Web Actions**: Wire dashboard buttons to broadcast `TxRestartHost` and other actions
4. **Monitoring**: Set up alerts for node health degradation
5. **Production Hardening**: TLS, auth, rate limiting, etc.

---

## Reference

- **Tendermint RPC Docs**: https://docs.tendermint.com/v0.35/rpc/
- **ABCI Spec**: https://docs.tendermint.com/v0.35/spec/abci/
- **ED25519 Keys**: Used for both nsm identity and Tendermint validator keys
