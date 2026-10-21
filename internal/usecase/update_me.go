package usecase

import (
	"context"

	"github.com/paramah/denar/internal/domain/user"
)

// UpdateMeInput are the preferences a person may change. Locale nil means
// automatic (no preference).
type UpdateMeInput struct {
	Locale *string
}

// UpdateMe changes the preferences of the signed in person. Bots have no
// profile. The language is a personal setting, not household data, so no
// household audit entry is written (AD-1304); the change is logged.
type UpdateMe struct{ d Deps }

func NewUpdateMe(d Deps) *UpdateMe { return &UpdateMe{d: d} }

func (uc *UpdateMe) Do(ctx context.Context, p Principal, in UpdateMeInput) (user.User, error) {
	if p.IsBot() {
		return user.User{}, errHumanOnly
	}
	var locale *user.Locale
	if in.Locale != nil {
		l, err := user.ParseLocale(*in.Locale)
		if err != nil {
			return user.User{}, err
		}
		locale = &l
	}
	var out user.User
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		u, err := uc.d.Users.ByID(ctx, p.User.ID)
		if err != nil {
			return err
		}
		u.Locale = locale
		if err := uc.d.Users.Update(ctx, u); err != nil {
			return err
		}
		out = u
		return nil
	})
	if err != nil {
		return user.User{}, err
	}
	uc.d.log().InfoContext(ctx, "user preferences updated", "user_id", out.ID, "locale", string(out.EffectiveLocale()))
	out.PasswordHash = ""
	return out, nil
}
