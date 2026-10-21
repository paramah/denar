package fakes

import (
	"fmt"
	"sync"

	"github.com/paramah/denar/internal/domain"
)

// IDGenerator gives "id-000001", "id-000002" and so on. Like ULIDs, the
// ids sort in the order they were made.
type IDGenerator struct {
	mu sync.Mutex
	n  int
}

func NewIDGenerator() *IDGenerator { return &IDGenerator{} }

func (g *IDGenerator) NewID() domain.ID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return domain.ID(fmt.Sprintf("id-%06d", g.n))
}

// TokenGenerator gives "token-1", "token-2" and so on, or Err when set.
type TokenGenerator struct {
	Err error

	mu sync.Mutex
	n  int
}

func NewTokenGenerator() *TokenGenerator { return &TokenGenerator{} }

func (g *TokenGenerator) NewToken() (string, error) {
	if g.Err != nil {
		return "", g.Err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return fmt.Sprintf("token-%d", g.n), nil
}
