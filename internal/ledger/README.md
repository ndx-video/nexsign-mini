# internal/ledger

Ledger state management (currently minimal/stub).

## Purpose

This package is intended to provide persistence and state management for the distributed ledger. It serves as an abstraction layer between the in-memory ABCI state and durable storage.

## Current Status

**Note**: This package is currently a stub with minimal functionality. The `state.go` file exists but contains no active implementation.

## Planned Functionality

The ledger package will be responsible for:

1. **State Persistence**: Save ABCI state to disk
2. **State Loading**: Load state from disk on startup
3. **State Queries**: Efficient querying of historical state
4. **Snapshots**: Create point-in-time snapshots for backup/restore
5. **State Sync**: Synchronize state with other nodes

## Intended Architecture

### StateStore Interface

```go
type StateStore interface {
    // Save the current state
    Save(state map[string]types.Host) error
    
    // Load the state from storage
    Load() (map[string]types.Host, error)
    
    // Query a specific host by public key
    Get(publicKey string) (*types.Host, error)
    
    // List all hosts
    List() ([]types.Host, error)
    
    // Create a snapshot
    Snapshot(path string) error
    
    // Restore from snapshot
    Restore(path string) error
}
```

### Storage Backends

Planned backends:
- **JSON File**: Simple file-based storage (good for small deployments)
- **BadgerDB**: Embedded key-value store (better performance)
- **PostgreSQL**: For production deployments requiring SQL queries

## Current Workaround

Until this package is fully implemented, state management is handled by:

1. **Initial State**: Loaded from `test-hosts.json` in `cmd/nsm/main.go`
2. **Runtime State**: Maintained in-memory by the ABCI app
3. **Persistence**: None (state is lost on restart)

### Loading Initial State

```go
// In cmd/nsm/main.go
func initStateFromFile(filePath string) map[string]types.Host {
    hosts := make(map[string]types.Host)
    file, err := os.ReadFile(filePath)
    if err != nil {
        return hosts
    }
    json.Unmarshal(file, &hosts)
    return hosts
}
```

## Future Implementation Plan

### Phase 1: JSON File Backend

Simple file-based storage with periodic saves:

```go
type JSONStore struct {
    path string
}

func (s *JSONStore) Save(state map[string]types.Host) error {
    data, _ := json.Marshal(state)
    return os.WriteFile(s.path, data, 0644)
}
```

### Phase 2: Periodic Snapshots

ABCI app calls `Save()` periodically (e.g., every 100 blocks or 5 minutes):

```go
// In ABCI EndBlock
if app.height % 100 == 0 {
    ledger.Save(app.state)
}
```

### Phase 3: Efficient Storage

Switch to BadgerDB or similar for better performance:

```go
type BadgerStore struct {
    db *badger.DB
}
```

### Phase 4: Query API

Add efficient querying:

```go
// Query by status
hosts, _ := store.QueryByStatus("Online")

// Query by IP range
hosts, _ := store.QueryByIPRange("192.168.1.0/24")
```

## Integration Points

### ABCI Application

The ABCI app will use the ledger for:
- Loading initial state on startup
- Persisting state after transaction execution
- Responding to Query requests

### Web API

The web server will use the ledger for:
- Displaying the current host list
- Providing API endpoints for state queries

### Testing

Tests will use an in-memory store:

```go
type MemoryStore struct {
    state map[string]types.Host
}
```

## Configuration

Future configuration options:

```json
{
  "ledger_backend": "json",
  "ledger_path": "/var/lib/nsm/state.json",
  "snapshot_interval": 100,
  "snapshot_path": "/var/lib/nsm/snapshots"
}
```

## Why This Package Exists

Even though it's currently minimal, the package exists to:

1. **Reserve the namespace**: Establish where state management logic will live
2. **Document the plan**: Capture design decisions for future implementation
3. **Prepare the structure**: Make it easy to add implementation without refactoring

## Development Notes

When implementing this package:

1. Start with a simple JSON file backend
2. Add comprehensive tests for concurrent access
3. Ensure state writes are atomic (write to temp file, then rename)
4. Add proper error handling and recovery
5. Consider using checksums to detect corruption
6. Implement proper locking for concurrent access

## Related Packages

- **`internal/abci`**: Uses the ledger for state persistence
- **`internal/types`**: Defines the `Host` type stored in the ledger
- **`internal/web`**: Queries the ledger for the UI

## Contributing

If you're implementing ledger functionality:

1. Start with the `StateStore` interface
2. Implement `JSONStore` as the first backend
3. Add unit tests covering all operations
4. Update the ABCI app to use the ledger
5. Update this README with usage examples
