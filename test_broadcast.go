//go:build manual
// +build manual

// Quick test of Tendermint broadcast functionality
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"nexsign.mini/nsm/internal/identity"
	"nexsign.mini/nsm/internal/tendermint"
	"nexsign.mini/nsm/internal/types"
)

func main() {
	fmt.Println("Testing Tendermint transaction broadcasting...")

	// Load or generate identity
	id, err := identity.LoadOrCreateIdentity("nsm_key.pem")
	if err != nil {
		log.Fatalf("Failed to load identity: %v", err)
	}

	fmt.Printf("Node ID: %s\n", id.PublicKeyHex())

	// Create broadcast client
	client := tendermint.NewBroadcastClient("http://localhost:26657")

	// 1) Ensure signer exists in state by broadcasting an add_host transaction (commit mode)
	host := types.Host{
		Hostname:       "LocalTest",
		IPAddress:      "127.0.0.1",
		AnthiasVersion: "",
		AnthiasStatus:  "",
		DashboardURL:   "http://localhost:8080",
		PublicKey:      id.PublicKeyHex(),
	}
	hostPayload, err := json.Marshal(host)
	if err != nil {
		log.Fatalf("Failed to marshal add_host payload: %v", err)
	}
	addHostTx := types.Transaction{
		Type:      types.TxAddHost,
		Timestamp: time.Now(),
		Payload:   hostPayload,
	}
	addHostSigned, err := addHostTx.Sign(id)
	if err != nil {
		log.Fatalf("Failed to sign add_host tx: %v", err)
	}
	fmt.Println("\nBroadcasting add_host (commit)...")
	if _, err := client.BroadcastSignedTransactionCommit(addHostSigned); err != nil {
		log.Fatalf("Failed to broadcast add_host: %v", err)
	}
	fmt.Println("✓ add_host committed")

	// 2) Now send an update_status transaction (sync mode)
	updPayload := types.UpdateStatusPayload{
		Status:   "Testing Broadcast",
		LastSeen: time.Now(),
	}
	updPayloadBytes, err := json.Marshal(updPayload)
	if err != nil {
		log.Fatalf("Failed to marshal update payload: %v", err)
	}
	updTx := types.Transaction{
		Type:      types.TxUpdateStatus,
		Timestamp: time.Now(),
		Payload:   updPayloadBytes,
	}
	updSigned, err := updTx.Sign(id)
	if err != nil {
		log.Fatalf("Failed to sign update tx: %v", err)
	}
	fmt.Println("Broadcasting update_status (sync)...")
	txHash, err := client.BroadcastSignedTransaction(updSigned)
	if err != nil {
		log.Fatalf("Failed to broadcast update_status: %v", err)
	}
	fmt.Printf("✓ update_status broadcast! tx: %s\n", txHash)

	fmt.Println("\n✅ Broadcast test complete!")
}
