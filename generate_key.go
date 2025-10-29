//go:build manual
// +build manual

// Generate ED25519 keypair for nsm node identity
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <output-file>\n", os.Args[0])
		os.Exit(1)
	}

	outfile := os.Args[1]

	// Generate ED25519 keypair
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate key: %v\n", err)
		os.Exit(1)
	}

	// Marshal to PKCS8 format
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal key: %v\n", err)
		os.Exit(1)
	}

	// Create output file with restrictive permissions
	file, err := os.OpenFile(outfile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Write PEM-encoded private key
	err = pem.Encode(file, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write PEM: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Key generated: %s\n", outfile)
}
