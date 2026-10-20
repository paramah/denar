package fakes

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/user"
)

// UserRepository keeps users in memory.
type UserRepository struct {
	store *Store
	users map[domain.ID]user.User
}

func NewUserRepository(s *Store) *UserRepository {
	r := &UserRepository{store: s, users: map[domain.ID]user.User{}}
	s.Register(r)
	return r
}

func (r *UserRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.users)
	return func() { r.users = saved }
}

// checkUnique reports a conflict of u with a user other than itself.
func (r *UserRepository) checkUnique(u user.User) error {
	for _, other := range r.users {
		if other.ID == u.ID {
			continue
		}
		if other.Email == u.Email {
			return domain.ConflictError{Reason: domain.ConflictDuplicateEmail}
		}
		if u.OIDCSubject != "" && other.OIDCIssuer == u.OIDCIssuer && other.OIDCSubject == u.OIDCSubject {
			return domain.ErrConflict
		}
	}
	return nil
}

func (r *UserRepository) Create(ctx context.Context, u user.User) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.users[u.ID]; ok {
			err = domain.ErrConflict
			return
		}
		if err = r.checkUnique(u); err == nil {
			r.users[u.ID] = u
		}
	})
	return err
}

func (r *UserRepository) Update(ctx context.Context, u user.User) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.users[u.ID]; !ok {
			err = domain.ErrNotFound
			return
		}
		if err = r.checkUnique(u); err == nil {
			r.users[u.ID] = u
		}
	})
	return err
}

func (r *UserRepository) ByID(ctx context.Context, id domain.ID) (u user.User, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if u, ok = r.users[id]; !ok {
			err = domain.ErrNotFound
		}
	})
	return u, err
}

func (r *UserRepository) ByIDs(ctx context.Context, ids []domain.ID) (users []user.User, err error) {
	r.store.Do(ctx, func() {
		users = r.sorted(func(u user.User) bool { return slices.Contains(ids, u.ID) })
	})
	return users, nil
}

func (r *UserRepository) ByOIDCSubject(ctx context.Context, issuer, subject string) (user.User, error) {
	return r.find(ctx, func(u user.User) bool {
		return subject != "" && u.OIDCIssuer == issuer && u.OIDCSubject == subject
	})
}

func (r *UserRepository) ByEmail(ctx context.Context, email string) (user.User, error) {
	return r.find(ctx, func(u user.User) bool { return u.Email == email })
}

func (r *UserRepository) find(ctx context.Context, match func(user.User) bool) (found user.User, err error) {
	err = domain.ErrNotFound
	r.store.Do(ctx, func() {
		for _, u := range r.users {
			if match(u) {
				found, err = u, nil
				return
			}
		}
	})
	return found, err
}

// sorted returns the matching users, oldest first. The caller holds the
// store.
func (r *UserRepository) sorted(match func(user.User) bool) []user.User {
	users := []user.User{}
	for _, u := range r.users {
		if match(u) {
			users = append(users, u)
		}
	}
	slices.SortFunc(users, func(a, b user.User) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID, b.ID))
	})
	return users
}
