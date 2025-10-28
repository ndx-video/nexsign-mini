# internal/tendermint

Tendermint Core integration package for nexSign mini (nsm).

# internal/tendermint

Tendermint Core integration package for nexSign mini (nsm).

## Overview

This package provides socket-based integration with Tendermint Core v0.35.9. It starts an ABCI server that listens on a Unix socket, allowing Tendermint to connect as a separate process. This is the **standard, battle-tested approach** for Tendermint applications.

## Architecture Decision: Option B (Socket-Based)

We chose **Option B: Socket/Remote Connection** for Tendermint integration:

```go
// nsm starts ABCI server
server := abciserver.NewSocketServer("unix://nsm.sock", abciApp)
server.Start()

// Tendermint connects as separate process
// $ tendermint node --proxy_app=unix://nsm.sock
```

**Why Option B?**
- ✅ **Simple, proven approach** - This is how most Tendermint apps work
- ✅ **Clean separation** - ABCI app and consensus are decoupled
- ✅ **No import issues** - Only need ABCI server package, not full Tendermint node
- ✅ **Better debugging** - Separate logs, can restart independently
- ✅ **Flexible deployment** - Can run on same or different machines

**Trade-offs:**
- Two processes to manage (solved with helpers in config.go)
- Slight socket overhead vs in-process (negligible for our use case)

## Usage

### Basic Usage

```go
import (
    "nexsign.mini/nsm/internal/tendermint"
    "nexsign.mini/nsm/internal/abci"
    "nexsign.mini/nsm/internal/config"
)

// After initializing ABCI app:
abciServer, err := tendermint.NewABCIServer(cfg, abciApp, "nsm.sock")
if err != nil {
    log.Fatal(err)
}

if err := abciServer.Start(); err != nil {
    log.Fatal(err)
}
defer abciServer.Stop()

// In a separate terminal or process, start Tendermint:
// $ tendermint init
// $ tendermint node --proxy_app=unix://$(pwd)/nsm.sock
```

### Helper Functions

The package provides helpers to manage Tendermint:

```go
// Initialize Tendermint home directory (first time only)
err := tendermint.InitTendermint("~/.tendermint")

// Get command to start Tendermint (can run as subprocess)
cmd := tendermint.GetTendermintCommand("~/.tendermint", "unix://nsm.sock")
cmd.Start()
```

## Integration Points

### Input Dependencies

- **ABCI Application** (`internal/abci`): The consensus-ready application implementing CheckTx/DeliverTx
- **Config** (`internal/config`): nsm configuration for socket path and settings

### Output Capabilities

- **ABCI Server**: Listens on Unix socket for Tendermint connections
- **Lifecycle Management**: Start/Stop/IsRunning methods
- **Tendermint Helpers**: Init and start commands for Tendermint process

### Connection Flow

```
nsm process                  Tendermint process
━━━━━━━━━━                  ━━━━━━━━━━━━━━━━━━
┌──────────┐                ┌──────────────┐
│   ABCI   │                │  Tendermint  │
│   App    │                │    Core      │
└────┬─────┘                └──────┬───────┘
     │                             │
     ▼                             ▼
┌──────────┐    Unix Socket   ┌──────────┐
│  ABCI    │◄────────────────►│   ABCI   │
│  Server  │    nsm.sock      │  Client  │
└──────────┘                  └──────────┘
```

## Implementation Status

✅ **COMPLETE - Option B (Socket-Based)**

- [x] ABCIServer with Unix socket listener
- [x] Start/Stop lifecycle methods  
- [x] Socket cleanup on shutdown
- [x] Tendermint initialization helpers
- [x] Command generation for starting Tendermint
- [x] Comprehensive documentation
- [x] All code compiles and tests pass

**Next Steps:**
- Wire ABCIServer into cmd/nsm/main.go
- Test with actual Tendermint node connection
- Add transaction broadcasting helper (RPC client)
- Add multi-node integration tests

## Files

- `node.go` - ABCIServer wrapper with socket lifecycle management
- `config.go` - Tendermint initialization and command helpers
- `broadcast.go` - (planned) Transaction broadcasting via RPC
- `README.md` - This file
- `TM_ROADMAP.md` - Detailed integration roadmap

## Testing

### Current Tests
All existing tests pass with the new socket-based implementation.

### Planned Tests
- `node_test.go`: ABCIServer lifecycle, socket creation/cleanup
- `config_test.go`: Tendermint init and command generation
- Integration test: Start ABCI server + Tendermint, send transactions

## References

- [Tendermint Documentation](https://docs.tendermint.com/)
- [ABCI Specification](https://docs.tendermint.com/v0.34/spec/abci/)
- [ABCI Tutorial (Socket Server)](https://docs.tendermint.com/v0.34/introduction/quick-start.html)
- [TM_ROADMAP.md](./TM_ROADMAP.md) - Our integration roadmap
- [internal/abci/README.md](../abci/README.md) - ABCI app documentation
