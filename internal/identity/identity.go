// Package identity handles loading, generating, and persisting node
// cryptographic identities (ED25519 keypairs). It provides helpers to create
// and load key files, ensure secure permissions, and build an Identity
// object used throughout the application for signing transactions and
// exposing the canonical public key for ledger entries.
package identity

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"os"
)

// LoadOrCreateIdentity loads an existing identity or creates a new one
// from the given key path. This is the main entry point for identity management.
//
// The function will:
// 1. Check if a key file exists at the given path
// 2. If it exists, load and validate the key
// 3. If it doesn't exist, generate a new keypair and save it
// 4. Create an Identity instance from the keypair
//
// The key file is stored in PEM format with PKCS8 encoding and
// must have 0600 permissions for security.
func LoadOrCreateIdentity(keyPath string) (*Identity, error) {
	info, err := os.Stat(keyPath)
	if os.IsNotExist(err) {
		// Key file doesn't exist - generate and save
		privKey, err := generateAndSaveKeyPair(keyPath)
		if err != nil {
			return nil, err
		}
		return NewIdentity(privKey), nil
	}
	if err != nil {
		return nil, err
	}

	// If the file exists but is empty (size 0), treat it as missing and generate
	if info.Size() == 0 {
		privKey, err := generateAndSaveKeyPair(keyPath)
		if err != nil {
			return nil, err
		}
		return NewIdentity(privKey), nil
	}

	privKey, err := loadKeyPair(keyPath)
	if err != nil {
		return nil, err
	}
	return NewIdentity(privKey), nil
}

// Backwards compatible wrapper used by older call sites
func LoadOrGenerateKeyPair(keyPath string) (ed25519.PrivateKey, error) {
	id, err := LoadOrCreateIdentity(keyPath)
	if err != nil {
		return nil, err
	}
	return id.PrivateKey(), nil
}

// GetPublicKeyHex returns the public key as a hex-encoded string.
func GetPublicKeyHex(priv ed25519.PrivateKey) string {
	pub := priv.Public().(ed25519.PublicKey)
	return hex.EncodeToString(pub)
}

func generateAndSaveKeyPair(keyPath string) (ed25519.PrivateKey, error) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}

	x509Encoded, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}

	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509Encoded,
	}

	file, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if err := pem.Encode(file, pemBlock); err != nil {
		return nil, err
	}

	return priv, nil
}

func loadKeyPair(keyPath string) (ed25519.PrivateKey, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	pemBlock, _ := pem.Decode(keyData)
	if pemBlock == nil {
		return nil, errors.New("failed to decode PEM block from key file")
	}

	genericKey, err := x509.ParsePKCS8PrivateKey(pemBlock.Bytes)
	if err != nil {
		return nil, err
	}

	privKey, ok := genericKey.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("key is not an ed25519 private key")
	}

	return privKey, nil
}
