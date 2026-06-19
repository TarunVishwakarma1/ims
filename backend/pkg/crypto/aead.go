package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// Encryptor performs AES-256-GCM authenticated encryption.
// Nonce is randomly generated per encryption (12 bytes) and prepended to ciphertext.
// Output format: nonce || ciphertext || tag — all bytes, no encoding.
type Encryptor struct {
	aead cipher.AEAD
}

// New constructs an Encryptor from a 32-byte key.
// If key is empty, returns a no-op encryptor (data passes through unchanged).
func New(key []byte) (*Encryptor, error) {
	if len(key) == 0 {
		return &Encryptor{aead: nil}, nil
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Encryptor{aead: aead}, nil
}

// Enabled reports whether encryption is active.
func (e *Encryptor) Enabled() bool {
	return e != nil && e.aead != nil
}

// Encrypt seals plaintext with a fresh random nonce.
// Returns plaintext unchanged when encryption is disabled.
func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
	if !e.Enabled() {
		return plaintext, nil
	}
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	// Seal appends the ciphertext+tag to nonce, giving nonce||ct||tag.
	return e.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt opens ciphertext that begins with the nonce.
// Returns input unchanged when encryption is disabled (allows reading
// historical plaintext rows during a rollout).
func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if !e.Enabled() {
		return ciphertext, nil
	}
	if len(ciphertext) < e.aead.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce := ciphertext[:e.aead.NonceSize()]
	ct := ciphertext[e.aead.NonceSize():]
	return e.aead.Open(nil, nonce, ct, nil)
}
