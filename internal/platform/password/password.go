// Package password hashes the passwords of local accounts.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Cost of new hashes: 64 MiB, 3 passes, 1 thread.
const (
	memoryKiB = 64 * 1024
	passes    = 3
	threads   = 1
	saltLen   = 16
	keyLen    = 32
)

// Limits on the cost a stored hash may ask for, so a damaged or hostile
// value cannot make Verify panic or exhaust memory.
const (
	maxMemoryKiB = 4 * memoryKiB
	maxPasses    = 16
	maxThreads   = 16
)

// maxConcurrent is how many keys may be derived at once. Each derivation
// holds its memory cost, so a burst of logins must not run them all
// together; the rest wait their turn.
const maxConcurrent = 4

var (
	b64   = base64.RawStdEncoding
	slots = make(chan struct{}, maxConcurrent)
	// deriveKey is argon2id; tests replace it.
	deriveKey = argon2.IDKey
)

// derive runs deriveKey in one of the maxConcurrent slots.
func derive(password string, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
	slots <- struct{}{}
	defer func() { <-slots }()
	return deriveKey([]byte(password), salt, time, memory, threads, keyLen)
}

// Hasher hashes passwords with argon2id and stores them in the PHC string
// format: $argon2id$v=19$m=65536,t=3,p=1$<salt>$<key>.
type Hasher struct{}

// Hash returns the encoded hash of password under a fresh random salt.
func (Hasher) Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := derive(password, salt, passes, memoryKiB, threads, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoryKiB, passes, threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify reports whether password matches encodedHash, using the cost the
// hash was made with. A malformed hash gives false.
func (Hasher) Verify(password, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v="+strconv.Itoa(argon2.Version) {
		return false
	}
	memory, time, par, ok := parseCost(parts[3])
	if !ok {
		return false
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return false
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false
	}
	got := derive(password, salt, time, memory, par, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// parseCost reads "m=<KiB>,t=<passes>,p=<threads>" and checks the limits.
func parseCost(s string) (memory, time uint32, par uint8, ok bool) {
	fields := strings.Split(s, ",")
	if len(fields) != 3 {
		return 0, 0, 0, false
	}
	var values [3]uint32
	for i, name := range []string{"m=", "t=", "p="} {
		digits, found := strings.CutPrefix(fields[i], name)
		if !found {
			return 0, 0, 0, false
		}
		n, err := strconv.ParseUint(digits, 10, 32)
		if err != nil || n == 0 {
			return 0, 0, 0, false
		}
		values[i] = uint32(n)
	}
	if values[0] > maxMemoryKiB || values[1] > maxPasses || values[2] > maxThreads {
		return 0, 0, 0, false
	}
	return values[0], values[1], uint8(values[2]), true
}
