package usecase

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/paramah/denar/internal/domain"
)

// botUsedInterval is how long a bot's last-used time may lag behind before
// a request writes it again (FR-034).
const botUsedInterval = time.Minute

// BotTokenPrefix starts every bot token, so that secret scanners and people
// can tell what the string is.
const BotTokenPrefix = "dnr_"

// AuthenticateBot resolves a bot token to its bot. A token that is
// unknown, replaced or malformed, and the token of a disabled, expired or
// deleted bot, all give the bare domain.ErrUnauthenticated. The bot is
// read from the store on every call; what its creator may do is read by
// access.household for each operation.
type AuthenticateBot struct {
	bots  BotRepository
	clock Clock
	log   *slog.Logger
}

// NewAuthenticateBot takes the logger for failed writes of the time last
// used; nil drops them.
func NewAuthenticateBot(bots BotRepository, clock Clock, log *slog.Logger) *AuthenticateBot {
	return &AuthenticateBot{bots: bots, clock: clock, log: log}
}

func (uc *AuthenticateBot) Do(ctx context.Context, token string) (Principal, error) {
	if !strings.HasPrefix(token, BotTokenPrefix) || len(token) > 200 {
		return Principal{}, domain.ErrUnauthenticated
	}
	b, err := uc.bots.ByTokenHash(ctx, hashToken(token))
	if found, err := ignoreNotFound(err); err != nil {
		return Principal{}, err
	} else if !found {
		return Principal{}, domain.ErrUnauthenticated
	}
	now := uc.clock.Now()
	if !b.Usable(now) {
		return Principal{}, domain.ErrUnauthenticated
	}
	if b.LastUsedAt.IsZero() || now.Sub(b.LastUsedAt) >= botUsedInterval {
		// Bookkeeping only: a write that fails must not lock the bot out.
		if err := uc.bots.TouchUsed(ctx, b.ID, now); err != nil {
			if uc.log != nil {
				uc.log.LogAttrs(ctx, slog.LevelWarn, "bot last used not stored",
					slog.String("bot_id", string(b.ID)), slog.String("error", err.Error()))
			}
		} else {
			b.LastUsedAt = now
		}
	}
	return Principal{Bot: b}, nil
}
