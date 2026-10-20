// Package id generates entity identifiers.
package id

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
)

// Crockford base32, as the ULID specification requires.
const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// Generator creates ULIDs: a 48-bit millisecond timestamp followed by 80
// random bits, written as 26 characters that sort by creation time. IDs
// made in the same millisecond increase by one, so they stay ordered. It is
// safe for concurrent use.
type Generator struct {
	now func() time.Time

	mu      sync.Mutex
	ms      uint64
	entropy [10]byte
}

// New returns a Generator reading the system clock.
func New() *Generator {
	return &Generator{now: time.Now}
}

// NewID returns an identifier greater than every one returned before it.
func (g *Generator) NewID() domain.ID {
	g.mu.Lock()
	defer g.mu.Unlock()

	// A clock that stands still or steps back keeps the last timestamp.
	if ms := uint64(g.now().UnixMilli()); ms > g.ms {
		g.ms = ms
		rand.Read(g.entropy[:])
	} else if !g.increment() {
		// The random part wrapped around; move on to the next millisecond.
		g.ms++
		rand.Read(g.entropy[:])
	}

	hi := g.ms<<16 | uint64(binary.BigEndian.Uint16(g.entropy[:2]))
	lo := binary.BigEndian.Uint64(g.entropy[2:])
	var out [26]byte
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = alphabet[lo&31]
		lo = lo>>5 | hi<<59
		hi >>= 5
	}
	return domain.ID(out[:])
}

// increment adds one to the random part and reports whether it fit.
func (g *Generator) increment() bool {
	for i := len(g.entropy) - 1; i >= 0; i-- {
		g.entropy[i]++
		if g.entropy[i] != 0 {
			return true
		}
	}
	return false
}
