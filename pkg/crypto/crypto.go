package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var (
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	ErrInvalidKeyLength   = errors.New("encryption key must be 32 bytes for AES-256")
)

// Encrypt encrypts plaintext using AES-256-GCM
// The key must be 32 bytes for AES-256
// Returns base64-encoded ciphertext (nonce + ciphertext)
func Encrypt(plaintext, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("%w: got %d bytes, want 32", ErrInvalidKeyLength, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and append nonce to ciphertext
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Return base64-encoded ciphertext
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext using AES-256-GCM
// The key must be 32 bytes for AES-256
func Decrypt(ciphertextB64 string, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("%w: got %d bytes, want 32", ErrInvalidKeyLength, len(key))
	}

	// Decode base64
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode base64", ErrInvalidCiphertext)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("%w: ciphertext too short", ErrInvalidCiphertext)
	}

	// Split nonce and actual ciphertext
	nonce, cipher := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, cipher, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decrypt", ErrInvalidCiphertext)
	}

	return plaintext, nil
}

// EncryptString is a convenience function for string input
func EncryptString(plaintext string, key []byte) (string, error) {
	return Encrypt([]byte(plaintext), key)
}

// DecryptString is a convenience function for string output
func DecryptString(ciphertextB64 string, key []byte) (string, error) {
	plaintext, err := Decrypt(ciphertextB64, key)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// GenerateKey generates a random 32-byte key for AES-256
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}
