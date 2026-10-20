// Package bot models non-human principals: tokens a member hands to an
// assistant or an automation, limited by scopes, by a list of wallets and
// always by what their creator may do.
package bot

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/domain/user"
)

// Scope is one permission a bot can be given. A write scope does not imply
// the read scope of the same resource.
type Scope string

const (
	ScopeWalletsRead        Scope = "wallets:read"
	ScopeWalletsWrite       Scope = "wallets:write"
	ScopeTransactionsRead   Scope = "transactions:read"
	ScopeTransactionsWrite  Scope = "transactions:write"
	ScopeBudgetRead         Scope = "budget:read"
	ScopeBudgetWrite        Scope = "budget:write"
	ScopeSubscriptionsRead  Scope = "subscriptions:read"
	ScopeSubscriptionsWrite Scope = "subscriptions:write"
	ScopeReportsRead        Scope = "reports:read"
	ScopeNotificationsRead  Scope = "notifications:read"
)

// Scopes is the closed list of scopes, in a stable order.
var Scopes = []Scope{
	ScopeWalletsRead, ScopeWalletsWrite,
	ScopeTransactionsRead, ScopeTransactionsWrite,
	ScopeBudgetRead, ScopeBudgetWrite,
	ScopeSubscriptionsRead, ScopeSubscriptionsWrite,
	ScopeReportsRead, ScopeNotificationsRead,
}

// Valid reports whether s is a known scope.
func (s Scope) Valid() bool { return slices.Contains(Scopes, s) }

// IsRead reports whether s only reads.
func (s Scope) IsRead() bool { return strings.HasSuffix(string(s), ":read") }

// RolePermits reports whether a member with the role may hold the scope:
// a viewer only reads, every other role reads and writes.
func RolePermits(role membership.Role, s Scope) bool {
	if !role.Valid() || !s.Valid() {
		return false
	}
	return s.IsRead() || role.CanWrite()
}

// ParseScopes checks and normalises scopes given as input: every one must
// be known, duplicates go, and the result follows the order of Scopes. At
// least one scope is required.
func ParseScopes(field string, in []string) ([]Scope, error) {
	if len(in) == 0 {
		return nil, domain.ValidationError{Field: field, Message: "must name at least one scope"}
	}
	seen := map[Scope]bool{}
	for _, raw := range in {
		s := Scope(raw)
		if !s.Valid() {
			return nil, domain.ValidationError{Field: field, Message: "contains an unknown scope: " + quote(raw)}
		}
		seen[s] = true
	}
	out := make([]Scope, 0, len(seen))
	for _, s := range Scopes {
		if seen[s] {
			out = append(out, s)
		}
	}
	return out, nil
}

// quote keeps a scope from the input printable in a message.
func quote(s string) string {
	if utf8.RuneCountInString(s) > 40 || strings.ContainsFunc(s, user.Unprintable) {
		return "(unprintable)"
	}
	return `"` + s + `"`
}

// MaxWallets is the longest wallet list a bot may be limited to.
const MaxWallets = 100

// ParseWalletIDs checks the wallet list of a bot syntactically and drops
// duplicates, keeping the first occurrence. An empty list means every
// wallet the creator sees. Whether the wallets exist is checked by the
// use case (from stage 002).
func ParseWalletIDs(field string, in []string) ([]domain.ID, error) {
	out := make([]domain.ID, 0, len(in))
	for _, raw := range in {
		id := domain.ID(strings.TrimSpace(raw))
		if id == "" || len(id) > 64 || strings.ContainsFunc(string(id), user.Unprintable) {
			return nil, domain.ValidationError{Field: field, Message: "must hold wallet ids"}
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	if len(out) > MaxWallets {
		return nil, domain.ValidationError{Field: field, Message: "must hold at most 100 wallets"}
	}
	return out, nil
}

// Bot is a token holder acting in one household for its creator.
type Bot struct {
	ID          domain.ID
	HouseholdID domain.ID
	Name        string
	Scopes      []Scope
	// WalletIDs limits the bot to these wallets; empty means every wallet
	// its creator sees.
	WalletIDs []domain.ID
	CreatedBy domain.ID
	CreatedAt time.Time
	// ExpiresAt is zero for a token that does not expire (decision D-009).
	ExpiresAt time.Time
	Disabled  bool
	// TokenHash is the SHA-256 of the token; TokenPrefix its first
	// characters, shown to people to recognise it.
	TokenHash   string
	TokenPrefix string
	LastUsedAt  time.Time
}

// MaxNameLength is the longest bot name, in characters.
const MaxNameLength = 100

// ValidateName checks a trimmed bot name.
func ValidateName(name string) error {
	if err := user.ValidateName("name", name); err != nil {
		return err
	}
	return nil
}

// ValidateExpiry checks an expiry time given as input: zero for none, or
// a moment after now.
func ValidateExpiry(expiresAt, now time.Time) error {
	if !expiresAt.IsZero() && !expiresAt.After(now) {
		return domain.ValidationError{Field: "expires_at", Message: "must be in the future"}
	}
	return nil
}

// Usable reports whether the bot's token works at now.
func (b Bot) Usable(now time.Time) bool {
	return !b.Disabled && (b.ExpiresAt.IsZero() || now.Before(b.ExpiresAt))
}

// EffectiveScopes returns the bot's scopes its creator may hold with the
// given role. A creator who is no longer a member ("" role) leaves none.
func (b Bot) EffectiveScopes(creatorRole membership.Role) []Scope {
	out := []Scope{}
	for _, s := range b.Scopes {
		if RolePermits(creatorRole, s) {
			out = append(out, s)
		}
	}
	return out
}

// Validate checks a normalised bot as input.
func (b Bot) Validate(now time.Time) error {
	return errors.Join(ValidateName(b.Name), ValidateExpiry(b.ExpiresAt, now))
}
