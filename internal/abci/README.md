# internal/abci

ABCI (Application Blockchain Interface) application for nexSign mini. This package implements the consensus layer that validates and executes transactions.

## Purpose

The ABCI application bridges the Tendermint consensus engine with the `nsm` business logic. It implements transaction validation (`CheckTx`) and execution (`DeliverTx`), and maintains the in-memory ledger state of known hosts.

## Key Components

### ABCIApplication

The main application struct that implements the ABCI interface:

```go
type ABCIApplication struct {
    state         map[string]types.Host  // In-memory state indexed by public key
    nodePrivKey   ed25519.PrivateKey     // Node's private key
    localPubKey   string                 // Hex-encoded local public key
    ActionHandler func(action string, payload []byte) error  // Optional handler for actions
}
```

### Transaction Types

The application handles three transaction types (defined in `internal/types`):

1. **TxAddHost** - Register a new host on the network
2. **TxUpdateStatus** - Update the status of an existing host
3. **TxRestartHost** - Request a restart action for a target host

### Response Codes

- `CodeTypeOK` (0) - Transaction is valid/executed successfully
- `CodeTypeEncodingError` (1) - Failed to decode transaction data
- `CodeTypeAuthError` (2) - Invalid signature or unauthorized signer
- `CodeTypeInvalidTx` (3) - Invalid transaction logic

## Usage

### Creating an ABCI Application

```go
import (
    "nexsign.mini/nsm/internal/abci"
    "nexsign.mini/nsm/internal/identity"
)

// Load node identity
privKey, _ := identity.LoadOrGenerateKeyPair("nsm_key.pem")

// Initialize with empty state or load from file
initialState := make(map[string]types.Host)

// Create ABCI app
app := abci.NewABCIApplication(initialState, privKey)

// Optional: Set an action handler for RestartHost transactions
app.ActionHandler = func(action string, payload []byte) error {
    // Handle the action (e.g., restart service)
    return nil
}
```

### Transaction Validation (CheckTx)

`CheckTx` validates transactions before they enter the mempool:

1. Decode and unmarshal the `SignedTransaction`
2. Verify the signature using ed25519
3. For non-AddHost transactions, verify the signer is in the state
4. Return OK or an error code

### Transaction Execution (DeliverTx)

`DeliverTx` executes valid transactions and updates state:

1. Re-validate signature (defense in depth)
2. Based on transaction type:
   - **AddHost**: Add the host to state (indexed by public key)
   - **UpdateStatus**: Update the host's status and last-seen time
   - **RestartHost**: If targeting the local node, invoke the ActionHandler
3. Return OK or an error code

## Security Model

### Signature Verification

All transactions must be signed by the host they represent (or the initiator for actions). The signature is verified using ed25519:

```go
ed25519.Verify(signedTx.PublicKey, signedTx.Tx, signedTx.Signature)
```

### Authorization

- **AddHost**: Any new public key can register itself (self-registration model)
- **UpdateStatus**: Only the host owner can update their own status
- **RestartHost**: Any registered host can request a restart of any target (authorization happens at ActionHandler level)

### Action Handler

The `ActionHandler` is an optional callback that allows safe, testable action execution:

- In tests, inject a mock handler to observe behavior without side effects
- In production, inject a handler that calls the agent or executes system commands
- If `nil`, restart actions are logged but not executed (safe default)

## State Management

State is an in-memory map indexed by hex-encoded public key:

```go
state := map[string]types.Host{
    "abc123...": {
        Hostname: "node1",
        IPAddress: "192.168.1.10",
        PublicKey: "abc123...",
        // ...
    },
}
```

To persist state across restarts, serialize to JSON and load on startup (see `cmd/nsm/main.go` for example).

## Testing

The package includes unit tests (`app_test.go`) that verify:

- AddHost happy path (CheckTx + DeliverTx)
- Invalid signature rejection
- UpdateStatus state mutation
- RestartHost ActionHandler invocation

Run tests:

```bash
go test ./internal/abci/...
```

## Integration with Tendermint

This ABCI application is designed to be connected to Tendermint Core. The typical flow:

1. Start Tendermint with this ABCI app
2. Transactions are broadcast to Tendermint
3. Tendermint calls CheckTx to validate
4. Consensus is reached on valid transactions
5. Tendermint calls DeliverTx to execute
6. State is updated and committed

## Future Enhancements

- Persist state to disk (currently in-memory only)
- Add consensus-level authorization policies
- Implement Query handlers for state inspection
- Add BeginBlock/EndBlock handlers for periodic tasks
