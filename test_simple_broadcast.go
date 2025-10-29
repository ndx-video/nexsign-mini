//go:build manual
// +build manual

package main

import (
	"encoding/json"
	"fmt"
	"time"

	"nexsign.mini/nsm/internal/identity"
	"nexsign.mini/nsm/internal/tendermint"
	"nexsign.mini/nsm/internal/types"
)

func main() {
	fmt.Println("Step 1: Loading identity...")
	id, err := identity.LoadOrCreateIdentity("nsm_key.pem")
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	fmt.Printf("Step 2: ID loaded: %s\n", id.PublicKeyHex())

	fmt.Println("Step 3: Creating transaction...")
	payload := types.UpdateStatusPayload{
		Status:   "Test",
		LastSeen: time.Now(),
	}
	payloadBytes, _ := json.Marshal(payload)
	tx := types.Transaction{
		Type:      types.TxUpdateStatus,
		Timestamp: time.Now(),
		Payload:   payloadBytes,
	}

	fmt.Println("Step 4: Signing...")
	signedTx, err := tx.Sign(id)
	if err != nil {
		fmt.Printf("ERROR signing: %v\n", err)
		return
	}

	fmt.Println("Step 5: Creating broadcast client...")
	client := tendermint.NewBroadcastClient("http://localhost:26657")

	fmt.Println("Step 6: Broadcasting...")
	txHash, err := client.BroadcastSignedTransaction(signedTx)
	if err != nil {
		fmt.Printf("ERROR broadcasting: %v\n", err)
		return
	}

	fmt.Printf("SUCCESS! TX Hash: %s\n", txHash)
}
