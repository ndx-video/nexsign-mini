// Package types tests exercise the transaction and payload serialization
// utilities defined in the `internal/types` package. These tests ensure that
// transactions marshal/unmarshal correctly and that payload shapes remain
// compatible across the codebase.
// Package types tests exercise the transaction and payload serialization
// utilities defined in the `internal/types` package. These tests ensure that
// transactions marshal/unmarshal correctly and that payload shapes remain
// compatible across the codebase.
package types

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"nexsign.mini/nsm/internal/identity"
)

func TestTransactionSigning(t *testing.T) {
	// Create a test identity
	id, err := identity.LoadOrCreateIdentity("test_key.pem")
	if err != nil {
		t.Fatalf("Failed to create test identity: %v", err)
	}
	defer os.Remove("test_key.pem")

	// Create a test transaction
	tx := &Transaction{
		Type:      TxUpdateStatus,
		Timestamp: time.Now(),
		Payload: json.RawMessage(`{
			"status": "online",
			"last_seen": "2025-10-28T12:00:00Z"
		}`),
	}

	// Sign the transaction
	signedTx, err := tx.Sign(id)
	if err != nil {
		t.Fatalf("Failed to sign transaction: %v", err)
	}

	// Verify the signature
	if !signedTx.Verify() {
		t.Error("Failed to verify transaction signature")
	}

	// Extract and verify the inner transaction
	extractedTx, err := signedTx.GetTransaction()
	if err != nil {
		t.Fatalf("Failed to extract transaction: %v", err)
	}

	if extractedTx.Type != tx.Type {
		t.Errorf("Transaction type mismatch. Got %s, want %s",
			extractedTx.Type, tx.Type)
	}
}

func TestTransactionPayloads(t *testing.T) {
	testCases := []struct {
		name    string
		txType  TransactionType
		payload interface{}
	}{
		{
			name:   "UpdateStatus",
			txType: TxUpdateStatus,
			payload: UpdateStatusPayload{
				Status:   "online",
				LastSeen: time.Now(),
			},
		},
		{
			name:   "RestartHost",
			txType: TxRestartHost,
			payload: RestartHostPayload{
				TargetPublicKey: "deadbeef",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Marshal payload
			payloadBytes, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("Failed to marshal payload: %v", err)
			}

			// Create transaction
			tx := &Transaction{
				Type:      tc.txType,
				Timestamp: time.Now(),
				Payload:   payloadBytes,
			}

			// Marshal and unmarshal full transaction
			txBytes, err := json.Marshal(tx)
			if err != nil {
				t.Fatalf("Failed to marshal transaction: %v", err)
			}

			var unmarshalled Transaction
			if err := json.Unmarshal(txBytes, &unmarshalled); err != nil {
				t.Fatalf("Failed to unmarshal transaction: %v", err)
			}

			if unmarshalled.Type != tc.txType {
				t.Errorf("Transaction type mismatch. Got %s, want %s",
					unmarshalled.Type, tc.txType)
			}
		})
	}
}
