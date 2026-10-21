package fakes

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/connection"
)

// ConnectionRepository keeps connections and their accounts in memory.
type ConnectionRepository struct {
	store       *Store
	connections map[domain.ID]connection.Connection
	accounts    map[domain.ID]map[string]connection.Account
}

func NewConnectionRepository(s *Store) *ConnectionRepository {
	r := &ConnectionRepository{store: s, connections: map[domain.ID]connection.Connection{}, accounts: map[domain.ID]map[string]connection.Account{}}
	s.Register(r)
	return r
}

func (r *ConnectionRepository) Snapshot() (restore func()) {
	saved, accounts := maps.Clone(r.connections), map[domain.ID]map[string]connection.Account{}
	for k, v := range r.accounts {
		accounts[k] = maps.Clone(v)
	}
	return func() { r.connections, r.accounts = saved, accounts }
}

func (r *ConnectionRepository) dropHousehold(id domain.ID) {
	for k, c := range r.connections {
		if c.HouseholdID == id {
			delete(r.connections, k)
			delete(r.accounts, k)
		}
	}
}

func cloneConnection(c connection.Connection) connection.Connection {
	c.SealedCredentials = slices.Clone(c.SealedCredentials)
	return c
}

func (r *ConnectionRepository) Create(ctx context.Context, c connection.Connection) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.connections[c.ID]; ok {
			err = domain.ErrConflict
			return
		}
		r.connections[c.ID] = cloneConnection(c)
	})
	return err
}

func (r *ConnectionRepository) Update(ctx context.Context, c connection.Connection) (err error) {
	r.store.Do(ctx, func() {
		old, ok := r.connections[c.ID]
		if !ok || old.HouseholdID != c.HouseholdID {
			err = domain.ErrNotFound
			return
		}
		old.Status, old.DisplayName, old.SealedCredentials = c.Status, c.DisplayName, slices.Clone(c.SealedCredentials)
		old.LastSyncAt, old.LastError, old.UpdatedAt = c.LastSyncAt, c.LastError, c.UpdatedAt
		r.connections[c.ID] = old
	})
	return err
}

func (r *ConnectionRepository) ByID(ctx context.Context, householdID, id domain.ID) (c connection.Connection, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if c, ok = r.connections[id]; !ok || c.HouseholdID != householdID {
			c, err = connection.Connection{}, domain.ErrNotFound
		}
	})
	return cloneConnection(c), err
}

func (r *ConnectionRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]connection.Connection, error) {
	out := []connection.Connection{}
	r.store.Do(ctx, func() {
		for _, c := range r.connections {
			if c.HouseholdID == householdID {
				out = append(out, cloneConnection(c))
			}
		}
	})
	slices.SortFunc(out, func(a, b connection.Connection) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName)), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

func (r *ConnectionRepository) PutAccount(ctx context.Context, householdID domain.ID, a connection.Account) (err error) {
	r.store.Do(ctx, func() {
		if c, ok := r.connections[a.ConnectionID]; !ok || c.HouseholdID != householdID {
			err = domain.ErrNotFound
			return
		}
		if r.accounts[a.ConnectionID] == nil {
			r.accounts[a.ConnectionID] = map[string]connection.Account{}
		}
		r.accounts[a.ConnectionID][a.ExternalID] = a
	})
	return err
}

func (r *ConnectionRepository) ListAccounts(ctx context.Context, householdID, connectionID domain.ID) ([]connection.Account, error) {
	out := []connection.Account{}
	var err error
	r.store.Do(ctx, func() {
		if c, ok := r.connections[connectionID]; !ok || c.HouseholdID != householdID {
			err = domain.ErrNotFound
			return
		}
		for _, a := range r.accounts[connectionID] {
			out = append(out, a)
		}
	})
	slices.SortFunc(out, func(a, b connection.Account) int { return cmp.Compare(a.ExternalID, b.ExternalID) })
	return out, err
}
