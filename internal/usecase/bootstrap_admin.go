package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain/user"
)

type BootstrapAdminInput struct {
	Email    string
	Password string
	Name     string
}

// BootstrapAdmin creates the instance admin account at start-up, unless an
// account with that email exists already. An existing account is left
// alone: changing the configured password does not reset it (FR-003).
type BootstrapAdmin struct {
	users  UserRepository
	hasher PasswordHasher
	ids    IDGenerator
	clock  Clock
	tx     TxManager
}

func NewBootstrapAdmin(users UserRepository, hasher PasswordHasher, ids IDGenerator, clock Clock, tx TxManager) *BootstrapAdmin {
	return &BootstrapAdmin{users: users, hasher: hasher, ids: ids, clock: clock, tx: tx}
}

// Do reports whether it created the account.
func (uc *BootstrapAdmin) Do(ctx context.Context, in BootstrapAdminInput) (created bool, err error) {
	email := user.NormalizeEmail(in.Email)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "Admin"
	}
	if err := errors.Join(user.ValidateNewUser(email, name), user.ValidatePassword("password", in.Password)); err != nil {
		return false, err
	}
	err = uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		_, err := uc.users.ByEmail(ctx, email)
		if exists, err := ignoreNotFound(err); err != nil || exists {
			return err
		}
		hash, err := uc.hasher.Hash(in.Password)
		if err != nil {
			return fmt.Errorf("hash the admin password: %w", err)
		}
		created = true
		return uc.users.Create(ctx, user.User{
			ID:            uc.ids.NewID(),
			Email:         email,
			Name:          name,
			PasswordHash:  hash,
			InstanceAdmin: true,
			Active:        true,
			CreatedAt:     uc.clock.Now(),
		})
	})
	if err != nil {
		return false, err
	}
	return created, nil
}
