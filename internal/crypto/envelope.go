// Package crypto provides envelope encryption for secrets stored at rest in the
// metadata database. Per-org Arc API keys are encrypted with AES-256-GCM using
// a master key supplied via the MEMTRACE_MASTER_KEY environment variable.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
)

const (
	// MasterKeyEnvVar is the environment variable Memtrace reads to obtain the
	// 32-byte master key (base64-encoded) used for envelope encryption.
	MasterKeyEnvVar = "MEMTRACE_MASTER_KEY"

	keySize   = 32 // AES-256
	nonceSize = 12 // GCM standard
)

// Cipher encrypts and decrypts byte slices using AES-256-GCM.
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher constructs a Cipher from a raw 32-byte key.
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("master key must be %d bytes, got %d", keySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM AEAD: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// NewCipherFromEnv reads the master key from MEMTRACE_MASTER_KEY (base64-encoded
// 32 bytes) and returns a Cipher. Returns a clear error if the variable is missing
// or malformed so the server can refuse to start.
func NewCipherFromEnv() (*Cipher, error) {
	raw := os.Getenv(MasterKeyEnvVar)
	if raw == "" {
		return nil, fmt.Errorf("%s is not set; generate one with `memtrace keygen master`", MasterKeyEnvVar)
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%s is not valid base64: %w", MasterKeyEnvVar, err)
	}
	return NewCipher(key)
}

// Encrypt seals plaintext with a fresh random 12-byte nonce. The nonce is
// returned alongside the ciphertext (which includes the GCM tag).
func (c *Cipher) Encrypt(plaintext []byte) (ciphertext, nonce []byte, err error) {
	nonce = make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext = c.aead.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// Decrypt opens ciphertext using the supplied nonce. Returns an error if the
// ciphertext or nonce has been tampered with or the master key is wrong.
func (c *Cipher) Decrypt(ciphertext, nonce []byte) ([]byte, error) {
	if len(nonce) != nonceSize {
		return nil, errors.New("nonce has wrong length")
	}
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return plaintext, nil
}

// GenerateMasterKey returns a fresh, base64-encoded 32-byte master key. Used by
// `memtrace keygen master` to print a key for operators to set in their env.
func GenerateMasterKey() (string, error) {
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
