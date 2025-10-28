# internal/identity

Node identity management using ED25519 cryptographic keys.

## Purpose

This package handles the creation, loading, and management of a node's cryptographic identity. Each nexSign mini node has a unique ED25519 key pair that serves as its permanent identity on the network.

## Key Components

### Identity

The main abstraction for a node's cryptographic identity:

```go
type Identity struct {
    privateKey ed25519.PrivateKey
    publicKey  ed25519.PublicKey
}
```

Methods:
- `Sign(message []byte) []byte` - Sign arbitrary data
- `Verify(message, signature []byte) bool` - Verify signatures
- `PublicKey() ed25519.PublicKey` - Get public key
- `PublicKeyHex() string` - Get hex-encoded public key
- `PrivateKey() ed25519.PrivateKey` - Get private key

## Usage

### Creating or Loading an Identity

```go
import "nexsign.mini/nsm/internal/identity"

// Load existing key or generate a new one
id, err := identity.LoadOrCreateIdentity("nsm_key.pem")
if err != nil {
    log.Fatal(err)
}

// Get the public key (hex string, used as node identifier)
pubKeyHex := id.PublicKeyHex()
fmt.Printf("Node ID: %s\n", pubKeyHex)
```

### Signing Data

```go
data := []byte("hello world")
signature := id.Sign(data)
```

### Verifying Signatures

```go
valid := id.Verify(data, signature)
if !valid {
    log.Println("Invalid signature!")
}
```

### Backward Compatibility

For code that expects raw `ed25519.PrivateKey`:

```go
privKey, err := identity.LoadOrGenerateKeyPair("nsm_key.pem")
```

## Key File Format

Identity keys are stored in PEM format (PKCS8):

```
-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg...
-----END PRIVATE KEY-----
```

### File Permissions

**Critical**: The key file MUST have `0600` permissions (owner read/write only):

```bash
chmod 600 nsm_key.pem
```

The `LoadOrCreateIdentity` function automatically sets these permissions when creating a new key.

## Security Model

### Identity as Trust Anchor

The ED25519 key pair is the node's permanent identity:

- **Public Key**: Used as the node's unique identifier (hex-encoded)
- **Private Key**: Used to sign transactions and prove identity
- **Signatures**: Prove that a transaction originated from a specific node

### Non-Repudiation

All state-changing operations (AddHost, UpdateStatus, RestartHost) must be signed with the node's private key. This ensures:

1. **Authenticity**: The transaction came from the claimed node
2. **Integrity**: The transaction hasn't been tampered with
3. **Non-repudiation**: The node cannot deny creating the transaction

### Key Rotation

Currently, key rotation is not supported. The key file is the node's permanent identity. If compromised:

1. Generate a new key file
2. Re-register the node with the new public key
3. Old transactions signed with the old key remain valid

## Implementation Details

### Key Generation

New keys are generated using `ed25519.GenerateKey()` from the Go standard library:

```go
pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
```

### Key Persistence

Private keys are encoded to PKCS8 format and PEM-wrapped before writing to disk:

```go
pkcs8Key, _ := x509.MarshalPKCS8PrivateKey(privKey)
pem.Encode(file, &pem.Block{
    Type:  "PRIVATE KEY",
    Bytes: pkcs8Key,
})
```

### Key Loading

Keys are loaded by:
1. Reading the PEM file
2. Decoding the PEM block
3. Parsing the PKCS8-encoded key
4. Extracting the ED25519 private key

If the file is missing or empty, a new key is generated automatically.

## Testing

The package includes unit tests covering:

- Identity creation and lifecycle
- Signing and verification
- File permissions
- Empty key file handling
- Concurrent access patterns

Run tests:

```bash
go test ./internal/identity/...
```

## Integration with Other Packages

### ABCI (Consensus)

The ABCI app uses the node's identity to verify transaction signatures:

```go
app := abci.NewABCIApplication(state, id.PrivateKey())
```

### Types (Transactions)

Transactions are signed using the Identity:

```go
tx := &types.Transaction{...}
signedTx, err := tx.Sign(id)
```

### Discovery (Network)

The public key hex string is used to identify nodes in the peer store and Tendermint configuration.

## Configuration

The key file path is configurable:

```json
{
  "key_file": "/var/lib/nsm/nsm_key.pem"
}
```

Or via environment variable:

```bash
export KEY_FILE=/secure/nsm_key.pem
```

Default: `nsm_key.pem` in the current directory

## Best Practices

### Production Deployment

1. **Store key securely**: Use a dedicated directory with restricted access
   ```bash
   sudo mkdir -p /var/lib/nsm
   sudo chmod 700 /var/lib/nsm
   ```

2. **Set proper ownership**:
   ```bash
   sudo chown nsm:nsm /var/lib/nsm/nsm_key.pem
   sudo chmod 600 /var/lib/nsm/nsm_key.pem
   ```

3. **Backup the key**: Store an encrypted backup in a secure location

4. **Never share**: The private key should never leave the node

### Development

For development, the default `nsm_key.pem` in the current directory is fine. The key will be automatically generated on first run.

### Testing

In tests, use temporary files:

```go
tmpFile := filepath.Join(t.TempDir(), "test_key.pem")
id, err := identity.LoadOrCreateIdentity(tmpFile)
```

## Troubleshooting

### Permission Denied

If you see:
```
Failed to load or generate key pair: permission denied
```

Check file and directory permissions:
```bash
ls -la nsm_key.pem
# Should show: -rw------- 1 nsm nsm ... nsm_key.pem
```

### Invalid Key Format

If the key file is corrupted or has invalid format:
```
Failed to decode PEM block from key file
```

Delete the key file and let the service regenerate it:
```bash
rm nsm_key.pem
# Restart the service - a new key will be generated
```

### Empty Key File

The code automatically handles empty key files by generating a new key. This can happen if:
- File creation was interrupted
- File system issues
- Incomplete copy operation

## Future Enhancements

- Support for hardware security modules (HSM)
- Key rotation with migration path
- Multi-signature support for high-security operations
- Integration with system keystores (e.g., PKCS#11)
