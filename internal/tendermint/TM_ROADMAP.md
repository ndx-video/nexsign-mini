# Tendermint Integration - Socket-Based Approach (Option B)

## Decision: Switched from Option A to Option B

**Original Plan:** In-process integration with `proxy.NewLocalClientCreator` (Option A)

**Issue:** Tendermint v0.35.9 package import complexity blocked implementation

**Solution:** Socket-based integration (Option B) - the standard Tendermint approach

### Why Option B is Better

1. **Unblocked immediately** - Only need `abci/server` package, no complex node imports
2. **Battle-tested** - This is how most Tendermint apps work (Cosmos SDK, etc.)
3. **Better separation** - ABCI app and consensus are properly decoupled
4. **Easier debugging** - Separate processes with separate logs
5. **More documentation** - Abundant examples and tutorials available

### Architecture

```
nsm Process                 Tendermint Process
━━━━━━━━━━━                ━━━━━━━━━━━━━━━━━━
ABCIServer                  Tendermint Core
    ↓                              ↓
unix://nsm.sock ←──────────────→ ABCI Client
```

## Proposed Implementation Steps

### **Step 1: Create Tendermint wrapper package**
**Location:** `internal/tendermint/node.go`

**Tasks:**
- Create `Node` struct wrapping Tendermint Core node
- Implement `NewNode(cfg *config.Config, abciApp *abci.ABCIApplication) (*Node, error)`
- Generate Tendermint config directory structure (`.tendermint/config/`)
- Generate `genesis.json` with single validator (this node's public key)
- Generate `config.toml` with proper P2P settings
- Use peer addresses from discovery service for `persistent_peers`

**Key decisions:**
- Run Tendermint in-process (same binary), not as separate process
- Use `node.NewDefault()` from Tendermint SDK
- Connect via local ABCI socket or in-process connection

---

### **Step 2: Wire Tendermint into main.go startup**
**Location:** `cmd/nsm/main.go`

**Tasks:**
- After initializing ABCI app (step 3), before web server (step 7):
  ```
  3. ABCI app ✅
  4. Web port ✅
  5. mDNS discovery ✅
  → NEW 5.5: Start Tendermint node with ABCI app
  6. Anthias client ✅
  7. Web server ✅
  ```
- Pass discovered peer addresses to Tendermint config
- Start Tendermint in background goroutine with proper error handling
- Implement graceful shutdown on SIGTERM/SIGINT

**Key decisions:**
- Tendermint starts **after** discovery so we can seed `persistent_peers`
- Store Tendermint data in configurable directory (default: `.tendermint/`)

---

### **Step 3: Implement transaction broadcasting**
**Location:** `internal/tendermint/broadcast.go`

**Tasks:**
- Create `BroadcastTx(signedTx *types.SignedTransaction) error` function
- Use Tendermint's local RPC client (avoid network overhead)
- Marshal transaction to JSON
- Use `BroadcastTxSync` or `BroadcastTxCommit` depending on guarantees needed
- Handle response codes and errors

**Key decisions:**
- Use `BroadcastTxSync` for dashboard actions (fast response)
- Use `BroadcastTxCommit` for Anthias polling (wait for consensus)

---

### **Step 4: Wire broadcast into existing code**

**A. Dashboard actions** (`internal/web/server.go`)
- Update POST handlers for host actions
- Instead of directly calling `abciApp.DeliverTx()`, call `tendermint.BroadcastTx()`
- This ensures transactions go through full consensus

**B. Anthias polling loop** (new code in `cmd/nsm/main.go` or `internal/anthias/poller.go`)
- Implement periodic status check (every 30s?)
- Fetch status from Anthias API
- Create `TxUpdateStatus` transaction
- Sign and broadcast via Tendermint

---

### **Step 5: Update configuration**
**Location:** `internal/config/config.go`, `deploy/config.json.sample`

**New fields:**
```json
{
  "tendermint_home": ".tendermint",
  "tendermint_rpc_port": 26657,
  "tendermint_p2p_port": 26656,
  "anthias_poll_interval_sec": 30
}
```

---

### **Step 6: Testing approach**

**Unit tests:**
- `internal/tendermint/node_test.go`: Config generation, genesis validation
- Mock Tendermint client for broadcast tests

**Integration test:**
- Start 3 nsm instances on different ports
- Ensure they discover each other via mDNS
- Broadcast transaction from node A
- Verify nodes B and C receive it via ABCI app state

**Manual testing:**
- Single-node consensus (easier debugging)
- Then 3-node local cluster
- Verify dashboard action creates transaction that shows up in all nodes

---

## Recommended Order

1. **Day 1:** Implement `internal/tendermint/node.go` (config + genesis generation)
2. **Day 2:** Wire into `main.go`, test single-node startup
3. **Day 3:** Implement `broadcast.go`, test transaction submission
4. **Day 4:** Wire dashboard actions to use broadcast
5. **Day 5:** Implement Anthias polling loop, test multi-node cluster

---

## Open Questions

1. **Single validator vs multi-validator genesis?**
   - Proposal: Start with single-validator (this node), add dynamic validator set later

2. **Persistent storage?**
   - Proposal: Use Tendermint's default LevelDB storage in `.tendermint/data/`

3. **Peer discovery timing?**
   - Proposal: Initial `persistent_peers` from mDNS, but Tendermint's P2P handles reconnection

4. **Transaction replay protection?**
   - Current ABCI app doesn't check nonces/sequence numbers
   - Proposal: Add sequence number to `SignedTransaction` in Phase 3.5

---

## Implementation Status

- [x] **Step 0: Socket-based ABCI server** (COMPLETE)
  - Created `internal/tendermint/node.go` with `ABCIServer` implementation
  - Created `internal/tendermint/config.go` with Tendermint initialization helpers
  - Created comprehensive `README.md` with Option B architecture documentation
  - Implemented Unix socket server using `abciserver.NewSocketServer()`
  - Added Start/Stop lifecycle management with proper cleanup
  - All code compiles successfully
  - All tests passing (15 tests)
  
**Files:**
- `node.go` - 127 lines, fully functional ABCIServer
- `config.go` - 69 lines, InitTendermint() and GetTendermintCommand() helpers
- `README.md` - Comprehensive socket-based integration guide

**Next Steps:**
  
- [x] **Step 1: Wire ABCIServer into `main.go`** (COMPLETE)
  - ✅ Added ABCIServer startup after discovery initialization (step 5.5)
  - ✅ Configured socket path from env vars (ABCI_SOCKET_PATH, TENDERMINT_HOME)
  - ✅ Start server with error handling
  - ✅ Updated shutdown handler to stop ABCI server gracefully
  - ✅ Log instructions for starting Tendermint separately
  - ✅ Binary compiles successfully
  
**Integration Code:**
```go
// 5.5. Start the ABCI server for Tendermint connection
tmHome := getEnv("TENDERMINT_HOME", tendermint.TendermintHome())
socketPath := getEnv("ABCI_SOCKET_PATH", "nsm.sock")
tmConfig := &tendermint.Config{
    TendermintHome: tmHome,
    SocketAddress:  "unix://" + socketPath,
}

abciServer, err := tendermint.NewABCIServer(abciApp, tmConfig)
if err != nil {
    log.Fatalf("Error creating ABCI server: %s", err)
}

if err := abciServer.Start(); err != nil {
    log.Fatalf("Error starting ABCI server: %s", err)
}
```
  
- [ ] **Step 2: Test with actual Tendermint node** (NEXT)
  - Run `tendermint init` to create config
  - Start nsm with ABCI server running
  - Start Tendermint with `--proxy_app=unix://nsm.sock`
  - Verify connection and ABCI method calls
  
- [ ] Step 3: Transaction broadcasting (`broadcast.go`)
  - Create RPC client to Tendermint
  - Implement `BroadcastTx()` functions
  - Handle response codes and errors
  
- [ ] Step 4: Wire dashboard actions to broadcast
- [ ] Step 5: Implement Anthias polling with broadcast
- [ ] Step 6: Multi-node integration testing
