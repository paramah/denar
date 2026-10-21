package usecase

import (
	"context"
	"slices"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/notification"
)

// preferences returns the person's preferences in the household for every
// event type, without channels that no longer exist.
func (d Deps) preferences(ctx context.Context, userID, householdID domain.ID) ([]notification.Preference, error) {
	stored, err := d.Preferences.List(ctx, userID, householdID)
	if err != nil {
		return nil, err
	}
	channels, err := d.Channels.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	all := notification.Merge(stored)
	for i := range all {
		all[i].ChannelIDs = slices.DeleteFunc(all[i].ChannelIDs, func(id domain.ID) bool {
			return !slices.ContainsFunc(channels, func(c notification.Channel) bool { return c.ID == id })
		})
	}
	return all, nil
}

// GetNotificationPreferences returns what the person wants to be told in
// a household, for every event type (FR-551); people only, any role.
type GetNotificationPreferences struct{ d Deps }

func NewGetNotificationPreferences(d Deps) *GetNotificationPreferences { return &GetNotificationPreferences{d: d} }

func (uc *GetNotificationPreferences) Do(ctx context.Context, p Principal, householdID domain.ID) ([]notification.Preference, error) {
	if err := requireHuman(p); err != nil {
		return nil, err
	}
	if _, err := uc.d.access().household(ctx, p, householdID); err != nil {
		return nil, err
	}
	return uc.d.preferences(ctx, p.User.ID, householdID)
}

// SetNotificationPreferences replaces the person's preferences in a
// household; types left out get the default. Channels must be the
// person's.
type SetNotificationPreferences struct{ d Deps }

func NewSetNotificationPreferences(d Deps) *SetNotificationPreferences { return &SetNotificationPreferences{d: d} }

func (uc *SetNotificationPreferences) Do(ctx context.Context, p Principal, householdID domain.ID, in []notification.Preference) ([]notification.Preference, error) {
	if err := requireHuman(p); err != nil {
		return nil, err
	}
	prefs, err := notification.NormalizePreferences(in)
	if err != nil {
		return nil, err
	}
	var out []notification.Preference
	err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := uc.d.access().household(ctx, p, householdID); err != nil {
			return err
		}
		channels, err := uc.d.Channels.ListByUser(ctx, p.User.ID)
		if err != nil {
			return err
		}
		for _, pref := range prefs {
			for _, id := range pref.ChannelIDs {
				if !slices.ContainsFunc(channels, func(c notification.Channel) bool { return c.ID == id }) {
					return domain.ValidationError{Field: "items.channel_ids", Message: "is not one of your channels: " + string(id)}
				}
			}
		}
		if err := uc.d.Preferences.Replace(ctx, p.User.ID, householdID, prefs); err != nil {
			return err
		}
		out, err = uc.d.preferences(ctx, p.User.ID, householdID)
		return err
	})
	return out, err
}
