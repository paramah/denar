// Package fakes holds in-memory implementations of the usecase ports for
// tests. Repository fakes keep their state behind a shared Store, so that
// the fake TxManager can roll all of them back together.
package fakes

import (
	"context"
	"sync"
)

// Snapshotter is a piece of fake state that the Store can roll back.
type Snapshotter interface {
	// Snapshot copies the current state and returns a function that puts
	// the copy back. The Store calls both while it holds its lock.
	Snapshot() (restore func())
}

// Store serialises access to the state of the repository fakes built on it
// and lets a transaction undo their changes. A repository registers itself
// with Register and runs every operation through Do.
type Store struct {
	mu    sync.Mutex
	parts []Snapshotter
}

func NewStore() *Store { return &Store{} }

// Register adds a repository's state to what a transaction rolls back.
// Call it while setting a test up, not inside a transaction.
func (s *Store) Register(p Snapshotter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.parts = append(s.parts, p)
}

// Do runs fn with exclusive access to the store. With the context of a
// running transaction it takes part in that transaction; otherwise it
// waits until no transaction is running. Like the single SQLite writer,
// a call inside a transaction that does not pass the transaction's context
// on blocks forever.
func (s *Store) Do(ctx context.Context, fn func()) {
	if !s.inTx(ctx) {
		s.mu.Lock()
		defer s.mu.Unlock()
	}
	fn()
}

type txKey struct{}

func (s *Store) inTx(ctx context.Context) bool {
	owner, _ := ctx.Value(txKey{}).(*Store)
	return owner == s
}
