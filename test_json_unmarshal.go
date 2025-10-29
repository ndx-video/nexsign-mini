//go:build manual
// +build manual

package main

import (
	"encoding/json"
	"fmt"
	"log"

	"nexsign.mini/nsm/internal/types"
)

func main() {
	// This is the JSON format that Tendermint sends to ABCI
	jsonStr := `{"tx":"eyJ0eXBlIjoidXBkYXRlX3N0YXR1cyIsInRpbWVzdGFtcCI6IjIwMjUtMTAtMjhUMjE6MjI6MTAuNDA4MjMxMjUrMTE6MDAiLCJwYXlsb2FkIjp7InN0YXR1cyI6IlRlc3RpbmciLCJsYXN0X3NlZW4iOiIyMDI1LTEwLTI4VDIxOjIyOjEwLjQwODE1NjM1KzExOjAwIn19","public_key":"daBCUPPi264Qe7GbFSEgs8NnarhxNv+8cts+Pss1V9Q=","signature":"PqgbqyQSe9kxVwPQS0bcuU/+CjysBr7ET+T7ra3DLxB+Jpxu2FlzZsj9bsgWKEON5ROp+55NUO9UgELp8Yx5DQ=="}`

	var signedTx types.SignedTransaction
	if err := json.Unmarshal([]byte(jsonStr), &signedTx); err != nil {
		log.Fatalf("Failed to unmarshal: %v", err)
	}

	fmt.Printf("✓ Successfully unmarshaled!\n")
	fmt.Printf("PublicKey length: %d\n", len(signedTx.PublicKey))
	fmt.Printf("Signature length: %d\n", len(signedTx.Signature))
	fmt.Printf("Tx length: %d\n", len(signedTx.Tx))

	// Now try to unmarshal the inner transaction
	var tx types.Transaction
	if err := json.Unmarshal(signedTx.Tx, &tx); err != nil {
		log.Fatalf("Failed to unmarshal inner tx: %v", err)
	}

	fmt.Printf("✓ Inner transaction unmarshaled!\n")
	fmt.Printf("Type: %s\n", tx.Type)
}
