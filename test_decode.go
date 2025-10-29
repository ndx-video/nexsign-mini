//go:build manual
// +build manual

// Simple decode test
package main

import (
	"encoding/json"
	"fmt"

	"nexsign.mini/nsm/internal/types"
)

func main() {
	// This is what we're sending (base64-encoded JSON)
	jsonStr := `{"tx":"eyJ0eXBlIjoidXBkYXRlX3N0YXR1cyIsInRpbWVzdGFtcCI6IjIwMjUtMTAtMjhUMjE6NDM6MzQuMTIxMjczOTIzKzExOjAwIiwicGF5bG9hZCI6eyJzdGF0dXMiOiJUZXN0aW5nIEJyb2FkY2FzdCIsImxhc3Rfc2VlbiI6IjIwMjUtMTAtMjhUMjE6NDM6MzQuMTIxMTk5MTMzKzExOjAwIn19","public_key":"daBCUPPi264Qe7GbFSEgs8NnarhxNv+8cts+Pss1V9Q=","signature":"xLLy7+mHQSZ8QSrK6fIoqScVhqW7hDuVY2OJiISDiJC0LB/GlnTwlSqFVJ0dMnzr4l5OcHzqOYdnx2b5SlWkCQ=="}`

	fmt.Println("Attempting to unmarshal...")
	var stx types.SignedTransaction
	if err := json.Unmarshal([]byte(jsonStr), &stx); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	fmt.Printf("SUCCESS!\n")
	fmt.Printf("  PublicKey len: %d\n", len(stx.PublicKey))
	fmt.Printf("  Signature len: %d\n", len(stx.Signature))
	fmt.Printf("  Tx len: %d\n", len(stx.Tx))
}
