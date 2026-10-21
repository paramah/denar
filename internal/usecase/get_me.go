package usecase

import (
	"context"

	"github.com/paramah/denar/internal/domain/user"
)

// Me is who the caller is: a person with their households and roles, or a
// bot with what it may do now.
type Me struct {
	User       *user.User
	Households []HouseholdView
	Bot        *BotView
}

// GetMe describes the principal of the request.
type GetMe struct{ d Deps }

func NewGetMe(d Deps) *GetMe { return &GetMe{d: d} }

func (uc *GetMe) Do(ctx context.Context, p Principal) (Me, error) {
	if p.IsBot() {
		s, err := uc.d.access().household(ctx, p, p.Bot.HouseholdID)
		if err != nil {
			return Me{}, err
		}
		view := botView(p.Bot, s.Role)
		return Me{Bot: &view, Households: []HouseholdView{{Household: s.Household, Role: s.Role}}}, nil
	}
	hs, err := myHouseholds(ctx, uc.d, p.User.ID)
	if err != nil {
		return Me{}, err
	}
	u := p.User
	u.PasswordHash = ""
	return Me{User: &u, Households: hs}, nil
}
