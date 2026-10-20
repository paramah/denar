package fakes

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
)

// BotRepository keeps bots in memory. Set TouchUsedErr to make TouchUsed
// fail.
type BotRepository struct {
	TouchUsedErr error

	store *Store
	bots  map[domain.ID]bot.Bot
}

func NewBotRepository(s *Store) *BotRepository {
	r := &BotRepository{store: s, bots: map[domain.ID]bot.Bot{}}
	s.Register(r)
	return r
}

func (r *BotRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.bots)
	return func() { r.bots = saved }
}

func (r *BotRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.bots, func(_ domain.ID, b bot.Bot) bool { return b.HouseholdID == id })
}

func clone(b bot.Bot) bot.Bot {
	b.Scopes = slices.Clone(b.Scopes)
	b.WalletIDs = slices.Clone(b.WalletIDs)
	if b.WalletIDs == nil {
		b.WalletIDs = []domain.ID{}
	}
	return b
}

func (r *BotRepository) checkUnique(b bot.Bot) error {
	for _, other := range r.bots {
		if other.ID == b.ID {
			continue
		}
		if other.HouseholdID == b.HouseholdID && strings.EqualFold(other.Name, b.Name) {
			return domain.ConflictError{Reason: domain.ConflictDuplicateName}
		}
		if other.TokenHash == b.TokenHash {
			return domain.ErrConflict
		}
	}
	return nil
}

func (r *BotRepository) Create(ctx context.Context, b bot.Bot) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.bots[b.ID]; ok {
			err = domain.ErrConflict
			return
		}
		if err = r.checkUnique(b); err == nil {
			r.bots[b.ID] = clone(b)
		}
	})
	return err
}

func (r *BotRepository) Update(ctx context.Context, b bot.Bot) (err error) {
	r.store.Do(ctx, func() {
		old, ok := r.bots[b.ID]
		if !ok {
			err = domain.ErrNotFound
			return
		}
		if err = r.checkUnique(b); err != nil {
			return
		}
		old.Name, old.Scopes, old.WalletIDs, old.ExpiresAt, old.Disabled = b.Name, b.Scopes, b.WalletIDs, b.ExpiresAt, b.Disabled
		old.TokenHash, old.TokenPrefix = b.TokenHash, b.TokenPrefix
		r.bots[b.ID] = clone(old)
	})
	return err
}

func (r *BotRepository) Delete(ctx context.Context, id domain.ID) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.bots[id]; !ok {
			err = domain.ErrNotFound
			return
		}
		delete(r.bots, id)
	})
	return err
}

func (r *BotRepository) find(ctx context.Context, match func(bot.Bot) bool) []bot.Bot {
	out := []bot.Bot{}
	r.store.Do(ctx, func() {
		for _, b := range r.bots {
			if match(b) {
				out = append(out, clone(b))
			}
		}
	})
	slices.SortFunc(out, func(a, b bot.Bot) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return out
}

func (r *BotRepository) ByID(ctx context.Context, id domain.ID) (bot.Bot, error) {
	found := r.find(ctx, func(b bot.Bot) bool { return b.ID == id })
	if len(found) == 0 {
		return bot.Bot{}, domain.ErrNotFound
	}
	return found[0], nil
}

func (r *BotRepository) ByTokenHash(ctx context.Context, hash string) (bot.Bot, error) {
	found := r.find(ctx, func(b bot.Bot) bool { return hash != "" && b.TokenHash == hash })
	if len(found) == 0 {
		return bot.Bot{}, domain.ErrNotFound
	}
	return found[0], nil
}

func (r *BotRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]bot.Bot, error) {
	return r.find(ctx, func(b bot.Bot) bool { return b.HouseholdID == householdID }), nil
}

func (r *BotRepository) DisableByCreator(ctx context.Context, householdID, creatorID domain.ID) (out []bot.Bot, err error) {
	out = []bot.Bot{}
	r.store.Do(ctx, func() {
		for id, b := range r.bots {
			if b.HouseholdID == householdID && b.CreatedBy == creatorID && !b.Disabled {
				b.Disabled = true
				r.bots[id] = b
				out = append(out, clone(b))
			}
		}
	})
	slices.SortFunc(out, func(a, b bot.Bot) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}

func (r *BotRepository) TouchUsed(ctx context.Context, id domain.ID, at time.Time) (err error) {
	if r.TouchUsedErr != nil {
		return r.TouchUsedErr
	}
	r.store.Do(ctx, func() {
		b, ok := r.bots[id]
		if !ok {
			err = domain.ErrNotFound
			return
		}
		b.LastUsedAt = at
		r.bots[id] = b
	})
	return err
}
