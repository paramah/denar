package rest

import (
	"time"

	"github.com/oapi-codegen/nullable"

	"git.eiac.dev/tools/denar/internal/adapter/rest/api"
	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/domain/user"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// deref returns the value p points to, or the zero value for nil.
func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

func optional[T any](p *T) domain.Optional[T] {
	if p == nil {
		return domain.Optional[T]{}
	}
	return domain.Some(*p)
}

func nullableTime(t time.Time) nullable.Nullable[time.Time] {
	if t.IsZero() {
		return nullable.NewNullNullable[time.Time]()
	}
	return nullable.NewNullableWithValue(t)
}

func nullableRole(r membership.Role) nullable.Nullable[api.Role] {
	if r == "" {
		return nullable.NewNullNullable[api.Role]()
	}
	return nullable.NewNullableWithValue(api.Role(r))
}

func apiUser(u user.User) api.User {
	return api.User{Id: string(u.ID), Email: u.Email, Name: u.Name, InstanceAdmin: u.InstanceAdmin, CreatedAt: u.CreatedAt}
}

func apiHousehold(v usecase.HouseholdView) api.Household {
	h := v.Household
	return api.Household{
		Id: string(h.ID), Name: h.Name, BaseCurrency: h.BaseCurrency, Timezone: h.Timezone,
		MonthStartDay: h.MonthStartDay, CreatedAt: h.CreatedAt, Role: nullableRole(v.Role),
	}
}

func apiHouseholds(vs []usecase.HouseholdView) []api.Household {
	out := make([]api.Household, len(vs))
	for i, v := range vs {
		out[i] = apiHousehold(v)
	}
	return out
}

func apiMember(v usecase.MemberView) api.Member {
	return api.Member{
		UserId: string(v.Membership.UserID), Email: v.User.Email, Name: v.User.Name,
		Role: api.Role(v.Membership.Role), JoinedAt: v.Membership.CreatedAt,
	}
}

func apiInvitation(v usecase.InvitationView) api.Invitation {
	i := v.Invitation
	return api.Invitation{
		Id: string(i.ID), Email: i.Email, Role: api.Role(i.Role), State: api.InvitationState(v.State),
		CreatedBy: string(i.CreatedBy), CreatedAt: i.CreatedAt, ExpiresAt: i.ExpiresAt,
		AcceptedAt: nullableTime(i.AcceptedAt), RevokedAt: nullableTime(i.RevokedAt),
	}
}

func apiScopes(scopes []bot.Scope) []api.Scope {
	out := make([]api.Scope, len(scopes))
	for i, s := range scopes {
		out[i] = api.Scope(s)
	}
	return out
}

func apiBot(v usecase.BotView) api.Bot {
	b := v.Bot
	wallets := make([]string, len(b.WalletIDs))
	for i, id := range b.WalletIDs {
		wallets[i] = string(id)
	}
	return api.Bot{
		Id: string(b.ID), HouseholdId: string(b.HouseholdID), Name: b.Name,
		Scopes: apiScopes(b.Scopes), EffectiveScopes: apiScopes(v.EffectiveScopes), WalletIds: wallets,
		CreatedBy: string(b.CreatedBy), CreatorRole: nullableRole(v.CreatorRole), CreatedAt: b.CreatedAt,
		ExpiresAt: nullableTime(b.ExpiresAt), Disabled: b.Disabled, TokenPrefix: b.TokenPrefix,
		LastUsedAt: nullableTime(b.LastUsedAt),
	}
}

func apiAuditEntry(e audit.Entry) api.AuditEntry {
	changes := make(map[string]struct {
		From interface{} `json:"from"`
		To   interface{} `json:"to"`
	}, len(e.Changes))
	for field, c := range e.Changes {
		changes[field] = struct {
			From interface{} `json:"from"`
			To   interface{} `json:"to"`
		}{From: c.From, To: c.To}
	}
	return api.AuditEntry{
		Id: string(e.ID), At: e.At, Actor: api.Actor{Kind: api.ActorKind(e.Actor.Kind), Id: string(e.Actor.ID)},
		Action: e.Action, ResourceType: e.ResourceType, ResourceId: string(e.ResourceID), Changes: changes,
	}
}
