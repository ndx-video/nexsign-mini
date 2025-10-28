// Package identity manages node keypairs and signing utilities. Each nsm
// instance maintains a persistent ed25519 private key which acts as the node's
// canonical identifier (public key hex). This package exposes an Identity
// abstraction for signing and verifying messages and for retrieving the
// canonical public key used across the ledger and networking layers.
package identity

import (
	"crypto/ed25519"
	"encoding/hex"
)

// Identity represents a node's cryptographic identity
type Identity struct {
	privateKey   ed25519.PrivateKey
	publicKey    ed25519.PublicKey
	publicKeyHex string
}

// NewIdentity creates a new Identity from a private key
func NewIdentity(privKey ed25519.PrivateKey) *Identity {
	pubKey := privKey.Public().(ed25519.PublicKey)
	return &Identity{
		privateKey:   privKey,
		publicKey:    pubKey,
		publicKeyHex: hex.EncodeToString(pubKey),
	}
}

// Sign signs the provided message with the identity's private key
func (i *Identity) Sign(message []byte) []byte {
	return ed25519.Sign(i.privateKey, message)
}

// Verify verifies a signature against a message using the identity's public key
func (i *Identity) Verify(message, signature []byte) bool {
	return ed25519.Verify(i.publicKey, message, signature)
}

// PublicKey returns the raw public key
func (i *Identity) PublicKey() ed25519.PublicKey {
	return i.publicKey
}

// PrivateKey returns the raw private key
func (i *Identity) PrivateKey() ed25519.PrivateKey {
	return i.privateKey
}

// PublicKeyHex returns the hex-encoded public key string
// This is the canonical node identifier in the network
func (i *Identity) PublicKeyHex() string {
	return i.publicKeyHex
}
