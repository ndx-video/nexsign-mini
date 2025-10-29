// Package types defines the core domain models and transaction formats used
// across the nexSign mini (nsm) application. It contains the Host data model,
// transaction types and payloads, and helpers for creating and verifying
// signed transactions. These types are used by the ABCI application, the web
// UI, and any networking/discovery components that share or validate state.
package types

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"time"

	"nexsign.mini/nsm/internal/identity"
)

// Host represents the state of a single Anthias host on the network.
type Host struct {
	Hostname       string    `json:"hostname"`
	IPAddress      string    `json:"ip_address"`
	AnthiasVersion string    `json:"anthias_version"`
	AnthiasStatus  string    `json:"anthias_status"`
	DashboardURL   string    `json:"dashboard_url"`
	LastSeen       time.Time `json:"last_seen"`
	// The hex-encoded public key of the node
	PublicKey string `json:"public_key"`
}

type TransactionType string

const (
	TxAddHost      TransactionType = "add_host"
	TxUpdateStatus TransactionType = "update_status"
	TxRestartHost  TransactionType = "restart_host"
)

// Transaction is the data that gets signed and sent
type Transaction struct {
	Type      TransactionType `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"` // Flexible payload (e.g., a Host struct, or a RestartAction)
}

// Sign creates a SignedTransaction using the provided identity
func (tx *Transaction) Sign(id *identity.Identity) (*SignedTransaction, error) {
	// Marshal the transaction
	txBytes, err := json.Marshal(tx)
	if err != nil {
		return nil, err
	}

	// Create signature using identity's private key
	signature := id.Sign(txBytes)

	// Create signed transaction
	return &SignedTransaction{
		Tx:        txBytes,
		PublicKey: []byte(id.PublicKey()),
		Signature: signature,
	}, nil
}

// SignedTransaction is what is actually broadcast to Tendermint
type SignedTransaction struct {
	// The transaction data
	Tx []byte `json:"tx"` // JSON-marshalled Transaction struct
	// The public key of the node that signed this tx
	PublicKey []byte `json:"public_key"`
	// The signature of the Tx field
	Signature []byte `json:"signature"`
}

// Verify checks if the transaction signature is valid
func (stx *SignedTransaction) Verify() bool {
	return ed25519.Verify(ed25519.PublicKey(stx.PublicKey), stx.Tx, stx.Signature)
}

// GetTransaction unmarshals and returns the inner Transaction
func (stx *SignedTransaction) GetTransaction() (*Transaction, error) {
	var tx Transaction
	if err := json.Unmarshal(stx.Tx, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

// GetPublicKeyHex returns the signer's public key as a hex string
func (stx *SignedTransaction) GetPublicKeyHex() string {
	return hex.EncodeToString(stx.PublicKey)
}

// --- Payloads ---

// Example payload for TxUpdateStatus
type UpdateStatusPayload struct {
	Status   string    `json:"status"`
	LastSeen time.Time `json:"last_seen"`
}

// Example payload for TxRestartHost
type RestartHostPayload struct {
	// The public key of the node to be restarted
	TargetPublicKey string `json:"target_public_key"`
}
