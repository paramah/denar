package usecase

import (
	"context"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/user"
)

type LoginLocalConfig struct {
	Enabled    bool          // false rejects every password login
	SessionTTL time.Duration // idle time after which a session ends
}

type LoginLocalInput struct {
	Email    string
	Password string
	// PreviousToken is the session token the browser sent, if any; that
	// session ends when the new one starts.
	PreviousToken string
}

// LoginLocal signs in an account that has a password. Every failure gives
// the bare domain.ErrUnauthenticated and takes one password check, so a
// caller cannot tell a wrong password from an unknown, password-less or
// inactive account.
type LoginLocal struct {
	users    UserRepository
	hasher   PasswordHasher
	sessions sessionStarter
	tx       TxManager
	enabled  bool
	// decoyHash is checked when the account has no hash of its own.
	decoyHash string
}

func NewLoginLocal(users UserRepository, hasher PasswordHasher, sessions SessionRepository,
	tokens TokenGenerator, clock Clock, tx TxManager, cfg LoginLocalConfig) *LoginLocal {
	// Without a decoy the check for an unknown account is only faster,
	// so a hashing failure here is not worth refusing to start for.
	decoy, _ := hasher.Hash("decoy password for unknown accounts")
	return &LoginLocal{
		users:     users,
		hasher:    hasher,
		sessions:  sessionStarter{sessions: sessions, tokens: tokens, clock: clock, ttl: cfg.SessionTTL},
		tx:        tx,
		enabled:   cfg.Enabled,
		decoyHash: decoy,
	}
}

func (uc *LoginLocal) Do(ctx context.Context, in LoginLocalInput) (Login, error) {
	if !uc.enabled {
		return Login{}, domain.ErrUnauthenticated
	}

	// The account is read and the password checked before the transaction:
	// a transaction holds the database's only writer connection, and a
	// password check is slow on purpose. Every refusal below costs exactly
	// one check, so they all take the same time.
	u, err := uc.users.ByEmail(ctx, user.NormalizeEmail(in.Email))
	found, err := ignoreNotFound(err)
	if err != nil {
		return Login{}, err
	}
	if !found || u.PasswordHash == "" {
		uc.hasher.Verify(in.Password, uc.decoyHash)
		return Login{}, domain.ErrUnauthenticated
	}
	if !uc.hasher.Verify(in.Password, u.PasswordHash) || !u.Active {
		return Login{}, domain.ErrUnauthenticated
	}

	// The account may have changed since it was read. Look again inside
	// the transaction, which is cheap: a session for an account that was
	// deactivated meanwhile would escape the removal of its sessions and
	// come back to life with the account.
	var login Login
	err = uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := uc.users.ByID(ctx, u.ID)
		if found, err := ignoreNotFound(err); err != nil {
			return err
		} else if !found || !current.Active || current.PasswordHash != u.PasswordHash {
			return domain.ErrUnauthenticated
		}
		login, err = uc.sessions.start(ctx, current, in.PreviousToken)
		return err
	})
	if err != nil {
		return Login{}, err
	}
	return login, nil
}
