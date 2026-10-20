package usecase

import "context"

// Logout ends the session of a token. A token without a session, or no
// token at all, is fine: signing out twice is not an error.
type Logout struct {
	sessions SessionRepository
}

func NewLogout(sessions SessionRepository) *Logout { return &Logout{sessions: sessions} }

func (uc *Logout) Do(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, err := ignoreNotFound(uc.sessions.Delete(ctx, hashToken(token)))
	return err
}
