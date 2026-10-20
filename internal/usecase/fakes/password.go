package fakes

import "strings"

const hashPrefix = "fake-hash:"

// PasswordHasher marks passwords with a prefix instead of hashing them, so
// tests stay fast and hashes readable. Hash returns Err when set.
type PasswordHasher struct {
	Err error
}

func NewPasswordHasher() *PasswordHasher { return &PasswordHasher{} }

func (h *PasswordHasher) Hash(password string) (string, error) {
	if h.Err != nil {
		return "", h.Err
	}
	return hashPrefix + password, nil
}

func (h *PasswordHasher) Verify(password, encodedHash string) bool {
	stored, ok := strings.CutPrefix(encodedHash, hashPrefix)
	return ok && stored == password
}
