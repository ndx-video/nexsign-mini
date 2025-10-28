# internal/types

Core domain models and transaction definitions for nexSign mini.

## Purpose

This package defines the fundamental data structures used throughout the application:

- **Host**: Represents a node in the nexSign network
- **Transaction**: State-changing operations
- **SignedTransaction**: Cryptographically signed transactions
- **Payloads**: Type-specific transaction payloads

## Core Types

### Host

Represents a single Anthias-powered digital signage node:

```go
type Host struct {
    Hostname       string `json:"hostname"`
    IPAddress      string `json:"ip_address"`
    AnthiasVersion string `json:"anthias_version"`
    AnthiasStatus  string `json:"anthias_status"`
    DashboardURL   string `json:"dashboard_url"`
    PublicKey      string `json:"public_key"`  // Hex-encoded ED25519 public key
}
```

**Key Fields**:
- `PublicKey`: Unique identifier for the host (never changes)
- `IPAddress`: Current IP address (may change)
- `AnthiasStatus`: Current status ("Online", "Offline", "Error", etc.)

### Transaction

Represents an operation to be executed via consensus:

```go
type Transaction struct {
    Type      TransactionType `json:"type"`
    Timestamp time.Time       `json:"timestamp"`
    Payload   json.RawMessage `json:"payload"`
}
```

**Transaction Types**:
- `TxAddHost`: Register a new host
- `TxUpdateStatus`: Update host status
- `TxRestartHost`: Request a host restart

### SignedTransaction

A transaction with cryptographic signature:

```go
type SignedTransaction struct {
    Tx        []byte `json:"tx"`         // JSON-marshalled Transaction
    PublicKey []byte `json:"public_key"` // Signer's public key
    Signature []byte `json:"signature"`  // ED25519 signature
}
```

## Usage

### Creating and Signing a Transaction

```go
import (
    "nexsign.mini/nsm/internal/types"
    "nexsign.mini/nsm/internal/identity"
)

// Load identity
id, _ := identity.LoadOrCreateIdentity("nsm_key.pem")

// Create host payload
host := types.Host{
    Hostname:       "signage-1",
    IPAddress:      "192.168.1.10",
    AnthiasVersion: "0.18.2",
    AnthiasStatus:  "Online",
    DashboardURL:   "http://192.168.1.10:80",
    PublicKey:      id.PublicKeyHex(),
}
payload, _ := json.Marshal(host)

// Create transaction
tx := &types.Transaction{
    Type:      types.TxAddHost,
    Timestamp: time.Now(),
    Payload:   payload,
}

// Sign the transaction
signedTx, err := tx.Sign(id)
if err != nil {
    log.Fatal(err)
}

// Now broadcast signedTx to the network
```

### Verifying a Signed Transaction

```go
// Verify signature
if !signedTx.Verify() {
    log.Println("Invalid signature!")
    return
}

// Get the inner transaction
tx, err := signedTx.GetTransaction()
if err != nil {
    log.Println("Failed to decode transaction")
    return
}

// Get signer's public key
signerPubKey := signedTx.GetPublicKeyHex()
```

### Working with Payloads

Each transaction type has a specific payload:

#### AddHost Payload

```go
// The payload is the Host struct itself
var host types.Host
json.Unmarshal(tx.Payload, &host)
```

#### UpdateStatus Payload

```go
type UpdateStatusPayload struct {
    Status   string    `json:"status"`
    LastSeen time.Time `json:"last_seen"`
}

payload := types.UpdateStatusPayload{
    Status:   "Online",
    LastSeen: time.Now(),
}
payloadBytes, _ := json.Marshal(payload)

tx := &types.Transaction{
    Type:      types.TxUpdateStatus,
    Timestamp: time.Now(),
    Payload:   payloadBytes,
}
```

#### RestartHost Payload

```go
type RestartHostPayload struct {
    TargetPublicKey string `json:"target_public_key"`
}

payload := types.RestartHostPayload{
    TargetPublicKey: "abc123...",
}
payloadBytes, _ := json.Marshal(payload)

tx := &types.Transaction{
    Type:      types.TxRestartHost,
    Timestamp: time.Now(),
    Payload:   payloadBytes,
}
```

## Transaction Flow

### 1. Creation

A node creates a transaction to represent an operation:

```go
tx := &types.Transaction{
    Type:      types.TxAddHost,
    Timestamp: time.Now(),
    Payload:   hostJSON,
}
```

### 2. Signing

The transaction is signed with the node's private key:

```go
signedTx, _ := tx.Sign(identity)
```

### 3. Serialization

The signed transaction is marshalled to JSON:

```go
txBytes, _ := json.Marshal(signedTx)
```

### 4. Broadcasting

The serialized transaction is sent to Tendermint/ABCI:

```go
// Via Tendermint RPC
tendermint.BroadcastTxSync(txBytes)
```

### 5. Validation

The ABCI app validates the signature:

```go
// In CheckTx
if !ed25519.Verify(signedTx.PublicKey, signedTx.Tx, signedTx.Signature) {
    return CodeTypeAuthError
}
```

### 6. Execution

The ABCI app executes the transaction:

```go
// In DeliverTx
switch tx.Type {
case TxAddHost:
    app.state[host.PublicKey] = host
}
```

## Security Model

### Signature Requirements

All transactions MUST be signed:
- Ensures authenticity (transaction came from the claimed node)
- Prevents tampering (any modification invalidates the signature)
- Enables non-repudiation (node cannot deny creating the transaction)

### Authorization Model

- **AddHost**: Self-registration (any new public key can join)
- **UpdateStatus**: Must be signed by the host owner
- **RestartHost**: Must be signed by a registered node (authorization at action handler level)

### Public Key as Identity

The hex-encoded public key serves as the permanent node identifier:
- Never changes (tied to the key file)
- Used as the state map key
- Used for authorization checks

## Testing

The package includes unit tests for:
- Transaction signing and verification
- Payload serialization/deserialization
- Invalid signature detection

Run tests:

```bash
go test ./internal/types/...
```

## JSON Serialization

All types use JSON for serialization, making them:
- Human-readable
- Easy to debug
- Compatible with REST APIs
- Simple to store in files

Example serialized `SignedTransaction`:

```json
{
  "tx": "eyJ0eXBlIjoiYWRkX2hvc3QiLCJ0aW1lc3RhbXAiOiIyMDI1LTEwLTI4VDEyOjAwOjAwWiIsInBheWxvYWQiOnsiaG9zdG5hbWUiOiJzaWduYWdlLTEiLCJpcF9hZGRyZXNzIjoiMTkyLjE2OC4xLjEwIn19",
  "public_key": "ZjY0YWViMzI5...",
  "signature": "YzEyZGVmNTY3..."
}
```

## Best Practices

### Creating Transactions

1. **Always set Timestamp**: Use `time.Now()` for auditability
2. **Validate Payloads**: Ensure required fields are present before creating the transaction
3. **Use Correct Type**: Match the transaction type to the payload structure

### Handling Transactions

1. **Verify Before Trust**: Always verify signatures before processing
2. **Check Signer Authority**: Ensure the signer has permission for the operation
3. **Validate Payload**: Unmarshal and validate the payload structure

### Error Handling

1. **Check Sign Errors**: `tx.Sign()` can fail if identity is invalid
2. **Handle Marshal Errors**: JSON marshalling can fail with circular references or invalid types
3. **Validate Unmarshal**: Check errors when extracting payloads

## Extending Types

### Adding a New Transaction Type

1. Add the new type constant:

```go
const (
    TxAddHost      TransactionType = "add_host"
    TxUpdateStatus TransactionType = "update_status"
    TxRestartHost  TransactionType = "restart_host"
    TxNewAction    TransactionType = "new_action"  // New type
)
```

2. Define the payload structure:

```go
type NewActionPayload struct {
    Field1 string `json:"field1"`
    Field2 int    `json:"field2"`
}
```

3. Update the ABCI app to handle the new type in `DeliverTx`

4. Update this README with usage examples

### Adding Fields to Host

To add a new field to the Host struct:

1. Add the field with a JSON tag:

```go
type Host struct {
    // ...existing fields...
    NewField string `json:"new_field"`
}
```

2. Update serialization code that creates Host instances
3. Consider backward compatibility with existing stored state
4. Update tests and documentation

## Related Packages

- **`internal/identity`**: Provides signing capabilities
- **`internal/abci`**: Validates and executes transactions
- **`internal/web`**: Displays hosts in the UI
- **`internal/ledger`**: Stores host state (future)
