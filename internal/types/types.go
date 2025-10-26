package types

import (
	"encoding/json"
	"time"
)

// Host represents the state of a single Anthias host on the network.
type Host struct {
	Hostname       string `json:"hostname"`
	IPAddress      string `json:"ip_address"`
	AnthiasVersion string `json:"anthias_version"`
	AnthiasStatus  string `json:"anthias_status"`
	DashboardURL   string `json:"dashboard_url"`
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

// SignedTransaction is what is actually broadcast to Tendermint
type SignedTransaction struct {
	// The transaction data
	Tx []byte `json:"tx"` // JSON-marshalled Transaction struct
	// The public key of the node that signed this tx
	PublicKey []byte `json:"public_key"`
	// The signature of the Tx field
	Signature []byte `json:"signature"`
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
