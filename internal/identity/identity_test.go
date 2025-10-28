// Package identity tests validate key generation, loading, and signing
// behavior for the Identity abstraction. These tests ensure persistent key
// files can be created, re-loaded, signed with, and that file permissions
// match security expectations.
package identity

import (
	"os"
	"testing"
)

func TestIdentityLifecycle(t *testing.T) {
	// Create temporary key file
	tmpFile, err := os.CreateTemp("", "test_key_*.pem")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Test creating new identity
	identity1, err := LoadOrCreateIdentity(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create identity: %v", err)
	}

	// Verify we can load the same identity
	identity2, err := LoadOrCreateIdentity(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load identity: %v", err)
	}

	// Verify both identities have the same public key
	if identity1.PublicKeyHex() != identity2.PublicKeyHex() {
		t.Errorf("Loaded identity differs from original. Got %s, want %s",
			identity2.PublicKeyHex(), identity1.PublicKeyHex())
	}
}

func TestSignAndVerify(t *testing.T) {
	// Create a new identity
	identity, err := LoadOrCreateIdentity("test_key.pem")
	if err != nil {
		t.Fatalf("Failed to create identity: %v", err)
	}
	defer os.Remove("test_key.pem")

	// Test message signing and verification
	message := []byte("Hello, nexSign mini!")

	// Sign the message
	signature := identity.Sign(message)

	// Verify with the same identity
	if !identity.Verify(message, signature) {
		t.Error("Failed to verify signature with own public key")
	}

	// Create another identity for negative testing
	otherIdentity, err := LoadOrCreateIdentity("other_key.pem")
	if err != nil {
		t.Fatalf("Failed to create other identity: %v", err)
	}
	defer os.Remove("other_key.pem")

	// Try to verify with wrong public key (should fail)
	if otherIdentity.Verify(message, signature) {
		t.Error("Incorrectly verified signature with wrong public key")
	}
}

func TestPermissions(t *testing.T) {
	keyPath := "secure_test_key.pem"

	// Create a new identity (which creates the key file)
	_, err := LoadOrCreateIdentity(keyPath)
	if err != nil {
		t.Fatalf("Failed to create identity: %v", err)
	}
	defer os.Remove(keyPath)

	// Check file permissions
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("Failed to stat key file: %v", err)
	}

	// On Unix systems, check for 0600 permissions
	if info.Mode().Perm() != 0600 {
		t.Errorf("Key file has wrong permissions. Got %v, want %v",
			info.Mode().Perm(), 0600)
	}
}
