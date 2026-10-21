// Package crypto encrypts secrets kept in the database (channel secrets,
// later connection credentials) with AES-256-GCM under the master key
// (DENAR_MASTER_KEY).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// KeySize is the length of the master key in bytes.
const KeySize = 32

// version is the first byte of a sealed value; a later format gets another.
const version byte = 1

// ErrOpen: the value is not one this key sealed, or it was changed.
var ErrOpen = errors.New("crypto: cannot open the sealed value")

// ParseKey decodes a base64 master key (standard or URL alphabet, with or
// without padding) and checks its length.
func ParseKey(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if key, err := enc.DecodeString(s); err == nil {
			if len(key) != KeySize {
				return nil, fmt.Errorf("the master key must be %d bytes, got %d", KeySize, len(key))
			}
			return key, nil
		}
	}
	return nil, errors.New("the master key must be base64")
}

// Box seals and opens values with one key.
type Box struct{ aead cipher.AEAD }

// NewBox returns a box for a 32-byte key.
func NewBox(key []byte) (*Box, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("crypto: key must be %d bytes", KeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal encrypts plaintext bound to aad (such as the id of the record it
// belongs to): version ‖ nonce ‖ ciphertext.
func (b *Box) Seal(plaintext, aad []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append([]byte{version}, nonce...)
	return b.aead.Seal(out, nonce, plaintext, aad), nil
}

// Open decrypts a value Seal made with the same key and aad; anything
// else gives ErrOpen.
func (b *Box) Open(sealed, aad []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < 1+n+b.aead.Overhead() || sealed[0] != version {
		return nil, ErrOpen
	}
	plain, err := b.aead.Open(nil, sealed[1:1+n], sealed[1+n:], aad)
	if err != nil {
		return nil, ErrOpen
	}
	return plain, nil
}
