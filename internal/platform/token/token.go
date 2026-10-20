// Package token generates secrets such as session tokens.
package token

import (
	"crypto/rand"
	"encoding/base64"
)

// size is the number of random bytes in a token.
const size = 32

// Generator creates random tokens with crypto/rand.
type Generator struct{}

// NewToken returns 256 random bits as unpadded base64url.
func (Generator) NewToken() (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
