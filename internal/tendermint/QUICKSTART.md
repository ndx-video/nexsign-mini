# Tendermint Integration - Quick Start Guide

This guide shows how to run nsm with Tendermint consensus (socket-based, Option B).

## Prerequisites

1. **Install Tendermint v0.35.x**:
   ```bash
   # Option 1: Download binary
   wget https://github.com/tendermint/tendermint/releases/download/v0.35.9/tendermint_0.35.9_linux_amd64.tar.gz
   tar -xzf tendermint_0.35.9_linux_amd64.tar.gz
   sudo mv tendermint /usr/local/bin/
   
   # Option 2: Build from source
   git clone https://github.com/tendermint/tendermint.git
   cd tendermint
   git checkout v0.35.9
   make install
   ```

2. **Verify installation**:
   ```bash
   tendermint version
   # Should show: 0.35.9
   ```

## Single Node Setup (Testing)

### Step 1: Initialize Tendermint

```bash
# Create Tendermint config and genesis
tendermint init --home ~/.tendermint

# This creates:
# ~/.tendermint/config/config.toml
# ~/.tendermint/config/genesis.json
# ~/.tendermint/config/node_key.json
# ~/.tendermint/config/priv_validator_key.json
```

### Step 2: Start nsm with ABCI Server

```bash
# In terminal 1
cd /path/to/nexsign-mini
go run cmd/nsm/main.go

# You should see:
# Consensus: ABCI application initialized.
# Consensus: ABCI server listening on unix://nsm.sock
# Consensus: To connect Tendermint, run:
#   tendermint init --home /home/user/.tendermint
#   tendermint node --home /home/user/.tendermint --proxy_app=unix://nsm.sock
```

### Step 3: Start Tendermint

```bash
# In terminal 2
tendermint node --proxy_app=unix://nsm.sock

# You should see:
# I[...] Starting ABCI with Tendermint
# I[...] Executed block...
# I[...] Committed state...
```

### Step 4: Test Transaction Broadcasting

Once both are running, you can:

1. **Via Dashboard**: Open http://localhost:8080 and trigger a host action
2. **Via API**: POST to the transaction endpoints
3. **Via curl**:
   ```bash
   # Broadcast via Tendermint RPC (once broadcast.go is implemented)
   curl http://localhost:26657/broadcast_tx_sync?tx="..."
   ```

## Multi-Node Setup (Consensus Testing)

### Node 1 Setup

```bash
# Terminal 1A: Start nsm on port 8080
PORT=8080 ABCI_SOCKET_PATH=nsm1.sock go run cmd/nsm/main.go

# Terminal 1B: Init and start Tendermint for node 1
tendermint init --home ~/.tendermint1
tendermint node --home ~/.tendermint1 --proxy_app=unix://nsm1.sock \
  --p2p.laddr tcp://0.0.0.0:26656 \
  --rpc.laddr tcp://0.0.0.0:26657
```

### Node 2 Setup

```bash
# Terminal 2A: Start nsm on port 8081
PORT=8081 ABCI_SOCKET_PATH=nsm2.sock go run cmd/nsm/main.go

# Terminal 2B: Init and start Tendermint for node 2
tendermint init --home ~/.tendermint2
tendermint node --home ~/.tendermint2 --proxy_app=unix://nsm2.sock \
  --p2p.laddr tcp://0.0.0.0:26666 \
  --rpc.laddr tcp://0.0.0.0:26667 \
  --p2p.persistent_peers="<node1_id>@localhost:26656"
```

**Note**: Get `<node1_id>` from node 1's startup logs or from `~/.tendermint1/config/node_key.json`.

### Node 3 Setup

```bash
# Terminal 3A: Start nsm on port 8082
PORT=8082 ABCI_SOCKET_PATH=nsm3.sock go run cmd/nsm/main.go

# Terminal 3B: Init and start Tendermint for node 3
tendermint init --home ~/.tendermint3
tendermint node --home ~/.tendermint3 --proxy_app=unix://nsm3.sock \
  --p2p.laddr tcp://0.0.0.0:26676 \
  --rpc.laddr tcp://0.0.0.0:26677 \
  --p2p.persistent_peers="<node1_id>@localhost:26656,<node2_id>@localhost:26666"
```

## Verification

### Check ABCI Connection

In the Tendermint logs, you should see:
```
I[...] Starting ABCI with Tendermint
I[...] service start msg="Starting ABCIClient service"
```

In the nsm logs, you should see ABCI method calls when transactions are processed.

### Check Consensus

1. Broadcast a transaction from any node
2. Check that all three nodes execute the same transaction via `DeliverTx`
3. Verify state consistency across all nodes via dashboard or API

### Monitor Tendermint

```bash
# Check node status
curl http://localhost:26657/status

# Check validators
curl http://localhost:26657/validators

# Check latest block
curl http://localhost:26657/block
```

## Troubleshooting

### Socket already in use

```bash
# Remove stale socket file
rm nsm.sock

# Or use a different socket path
ABCI_SOCKET_PATH=nsm-new.sock go run cmd/nsm/main.go
```

### Connection refused

- Ensure nsm started BEFORE Tendermint
- Check that socket path matches in both commands
- Verify socket file exists: `ls -la nsm.sock`

### Genesis mismatch (multi-node)

All nodes must have the SAME genesis.json. Copy from node 1:
```bash
cp ~/.tendermint1/config/genesis.json ~/.tendermint2/config/
cp ~/.tendermint1/config/genesis.json ~/.tendermint3/config/
```

### Peer connection issues

- Check firewall rules allow P2P ports (26656, 26666, 26676)
- Verify `persistent_peers` IDs match node keys
- Check P2P listen addresses don't conflict

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ABCI_SOCKET_PATH` | `nsm.sock` | Unix socket path for ABCI |
| `TENDERMINT_HOME` | `~/.tendermint` | Tendermint data directory |
| `PORT` | `8080` | nsm web server port |
| `KEY_FILE` | `nsm_key.pem` | nsm node identity key |

### Tendermint Config

Edit `~/.tendermint/config/config.toml` to adjust:
- `consensus.timeout_*` - Block time and consensus timeouts
- `p2p.persistent_peers` - Hardcoded peer list
- `p2p.max_num_inbound_peers` - Peer limits
- `rpc.laddr` - RPC server address

## Next Steps

Once basic connection works:

1. **Implement transaction broadcasting** (`internal/tendermint/broadcast.go`)
2. **Wire dashboard actions** to use `BroadcastTx()` instead of direct `DeliverTx()`
3. **Add Anthias polling** with transaction broadcast on status changes
4. **Test multi-node consensus** with real host state updates

## References

- [Tendermint Docs - Getting Started](https://docs.tendermint.com/v0.35/introduction/quick-start.html)
- [ABCI Specification](https://docs.tendermint.com/v0.35/spec/abci/)
- [Tendermint Configuration](https://docs.tendermint.com/v0.35/nodes/configuration.html)
