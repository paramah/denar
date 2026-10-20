package usecase

import (
	"context"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
)

// sessionTouchInterval is how long a session's last-seen time may lag
// behind before a request writes it again.
const sessionTouchInterval = time.Hour

// Authenticated is the principal of a valid session.
type Authenticated struct {
	Principal Principal
	// ExpiresAt is when the session ends unless it is used again.
	ExpiresAt time.Time
	// Renewed tells that this call moved ExpiresAt forward.
	Renewed bool
}

// Authenticate resolves a session token to the user it belongs to. An
// unknown or expired session and a missing or inactive user all give the
// bare domain.ErrUnauthenticated. The session's expiry slides with use,
// written at most once per sessionTouchInterval.
type Authenticate struct {
	sessions SessionRepository
	users    UserRepository
	clock    Clock
	ttl      time.Duration
}

func NewAuthenticate(sessions SessionRepository, users UserRepository, clock Clock, sessionTTL time.Duration) *Authenticate {
	return &Authenticate{sessions: sessions, users: users, clock: clock, ttl: sessionTTL}
}

func (uc *Authenticate) Do(ctx context.Context, token string) (Authenticated, error) {
	if token == "" {
		return Authenticated{}, domain.ErrUnauthenticated
	}
	hash := hashToken(token)
	session, err := uc.sessions.ByTokenHash(ctx, hash)
	if found, err := ignoreNotFound(err); err != nil {
		return Authenticated{}, err
	} else if !found {
		return Authenticated{}, domain.ErrUnauthenticated
	}
	now := uc.clock.Now()
	if !session.ExpiresAt.After(now) {
		return Authenticated{}, domain.ErrUnauthenticated
	}
	u, err := uc.users.ByID(ctx, session.UserID)
	if found, err := ignoreNotFound(err); err != nil {
		return Authenticated{}, err
	} else if !found || !u.Active {
		return Authenticated{}, domain.ErrUnauthenticated
	}

	result := Authenticated{Principal: Principal{User: u}, ExpiresAt: session.ExpiresAt}
	if now.Sub(session.LastSeenAt) > sessionTouchInterval {
		expiresAt := now.Add(uc.ttl)
		err := uc.sessions.Touch(ctx, hash, now, expiresAt)
		if found, err := ignoreNotFound(err); err != nil {
			return Authenticated{}, err
		} else if !found {
			// The session ended between the read and the write.
			return Authenticated{}, domain.ErrUnauthenticated
		}
		result.ExpiresAt, result.Renewed = expiresAt, true
	}
	return result, nil
}
