//go:build manual
// +build manual

// Debug version of broadcast test
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"nexsign.mini/nsm/internal/identity"
	"nexsign.mini/nsm/internal/types"
)

func main() {
	fmt.Println("=== DEBUG: Transaction Creation ===\n")

	// Load identity
	id, err := identity.LoadOrCreateIdentity("nsm_key.pem")
	if err != nil {
		log.Fatalf("Failed to load identity: %v", err)
	}

	fmt.Printf("PublicKey (raw): %v\n", id.PublicKey())
	fmt.Printf("PublicKey (hex): %s\n", id.PublicKeyHex())
	fmt.Printf("PublicKey (len): %d\n\n", len(id.PublicKey()))

	// Create transaction
	payload := types.UpdateStatusPayload{
		Status:   "Testing",
		LastSeen: time.Now(),
	}

	payloadBytes, _ := json.Marshal(payload)

	tx := types.Transaction{
		Type:      types.TxUpdateStatus,
		Timestamp: time.Now(),
		Payload:   payloadBytes,
	}

	fmt.Printf("Transaction: %+v\n\n", tx)

	// Sign it
	signedTx, err := tx.Sign(id)
	if err != nil {
		log.Fatalf("Failed to sign: %v", err)
	}

	fmt.Printf("SignedTransaction.PublicKey (raw): %v\n", signedTx.PublicKey)
	fmt.Printf("SignedTransaction.PublicKey (hex): %s\n", hex.EncodeToString(signedTx.PublicKey))
	fmt.Printf("SignedTransaction.PublicKey (len): %d\n", len(signedTx.PublicKey))
	fmt.Printf("SignedTransaction.Signature (len): %d\n", len(signedTx.Signature))
	fmt.Printf("SignedTransaction.Tx (len): %d\n\n", len(signedTx.Tx))

	// Marshal to JSON (what gets sent over HTTP)
	signedTxJSON, err := json.Marshal(signedTx)
	if err != nil {
		log.Fatalf("Failed to marshal: %v", err)
	}

	fmt.Printf("JSON (will be hex-encoded for Tendermint):\n%s\n\n", string(signedTxJSON))

	// Hex encode (what Tendermint RPC expects)
	txHex := fmt.Sprintf("%X", signedTxJSON)
	fmt.Printf("Hex-encoded (sent to Tendermint):\n%s\n\n", txHex)

	// What Tendermint will decode
	fmt.Printf("Tendermint will decode hex back to JSON, then ABCI app will unmarshal JSON\n")
}
