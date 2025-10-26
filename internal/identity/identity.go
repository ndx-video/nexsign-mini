package identity

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"os"
)

// LoadOrGenerateKeyPair loads an ed25519 private key from the given path.
// If the file does not exist, it generates a new key and saves it to the path.
func LoadOrGenerateKeyPair(keyPath string) (ed25519.PrivateKey, error) {
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return generateAndSaveKeyPair(keyPath)
	} else if err != nil {
		return nil, err
	}

	return loadKeyPair(keyPath)
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
