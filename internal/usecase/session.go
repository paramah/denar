package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/paramah/denar/internal/domain/user"
)

// Login is the outcome of a successful sign-in. Token is the session
// secret for the cookie; it is given out here only and never stored.
type Login struct {
	User      user.User
	Token     string
	ExpiresAt time.Time
}

// HashToken returns the form in which session, invitation and bot tokens
// and OIDC states are stored: the SHA-256 of the secret, in hex.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func hashToken(token string) string { return HashToken(token) }

// sessionStarter opens sessions for the sign-in use cases.
type sessionStarter struct {
	sessions SessionRepository
	tokens   TokenGenerator
	clock    Clock
	ttl      time.Duration
}

// start replaces the session of previousToken, if any, with a new one for
// the user. Every sign-in gets a fresh token. Call it in a transaction.
func (s sessionStarter) start(ctx context.Context, u user.User, previousToken string) (Login, error) {
	if previousToken != "" {
		if err := s.sessions.Delete(ctx, hashToken(previousToken)); err != nil {
			return Login{}, err
		}
	}
	token, err := s.tokens.NewToken()
	if err != nil {
		return Login{}, err
	}
	now := s.clock.Now()
	session := Session{
		TokenHash:  hashToken(token),
		UserID:     u.ID,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(s.ttl),
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return Login{}, err
	}
	return Login{User: u, Token: token, ExpiresAt: session.ExpiresAt}, nil
}
