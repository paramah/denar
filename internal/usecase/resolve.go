package usecase

import (
	"context"
	"fmt"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/budget"
	"git.eiac.dev/tools/denar/internal/domain/payee"
	"git.eiac.dev/tools/denar/internal/domain/subscription"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
)

// RefNotFound says that nothing the principal sees matches a reference by
// id or name. It matches domain.ErrNotFound, so a hidden resource and a
// missing one stay alike.
type RefNotFound struct {
	Kind  string // wallet, envelope, payee or subscription
	Field string // the input field of the reference
}

func (e RefNotFound) Error() string { return e.Kind + " not found" }

func (e RefNotFound) Is(target error) bool { return target == domain.ErrNotFound }

// maxCandidates is how many names an ambiguous reference lists.
const maxCandidates = 10

// Resolve finds a wallet, envelope, payee or subscription a principal
// names by id or by name, for clients that speak in names such as an
// assistant (AD-601). The scopes given are checked first, so a principal
// without them learns nothing of the names. Only what the principal sees
// is a candidate: an id, then a name ignoring case, then the one name
// that contains the reference. Several candidates are a
// domain.ValidationError listing them; none is RefNotFound. An empty
// reference gives the zero value.
type Resolve struct{ d Deps }

func NewResolve(d Deps) *Resolve { return &Resolve{d: d} }

func (uc *Resolve) scope(ctx context.Context, p Principal, householdID domain.ID, scopes []bot.Scope) (HouseholdScope, error) {
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return HouseholdScope{}, err
	}
	for _, sc := range scopes {
		if err := requireScope(s, sc); err != nil {
			return HouseholdScope{}, err
		}
	}
	return s, nil
}

// Wallet resolves a wallet the principal reaches, archived ones included.
func (uc *Resolve) Wallet(ctx context.Context, p Principal, householdID domain.ID, field, ref string, scopes ...bot.Scope) (wallet.Wallet, error) {
	s, err := uc.scope(ctx, p, householdID, scopes)
	if err != nil || strings.TrimSpace(ref) == "" {
		return wallet.Wallet{}, err
	}
	ws, err := uc.d.visibleWallets(ctx, s, p)
	if err != nil {
		return wallet.Wallet{}, err
	}
	return pickRef("wallet", field, ref, ws, func(w wallet.Wallet) (domain.ID, string) { return w.ID, w.Name })
}

// Envelope resolves an envelope of the household, hidden ones included.
func (uc *Resolve) Envelope(ctx context.Context, p Principal, householdID domain.ID, field, ref string, scopes ...bot.Scope) (budget.Envelope, error) {
	if _, err := uc.scope(ctx, p, householdID, scopes); err != nil || strings.TrimSpace(ref) == "" {
		return budget.Envelope{}, err
	}
	es, err := uc.d.Envelopes.ListByHousehold(ctx, householdID)
	if err != nil {
		return budget.Envelope{}, err
	}
	return pickRef("envelope", field, ref, es, func(e budget.Envelope) (domain.ID, string) { return e.ID, e.Name })
}

// Payee resolves a payee of the household by id or name (its patterns
// are not looked at).
func (uc *Resolve) Payee(ctx context.Context, p Principal, householdID domain.ID, field, ref string, scopes ...bot.Scope) (payee.Payee, error) {
	if _, err := uc.scope(ctx, p, householdID, scopes); err != nil || strings.TrimSpace(ref) == "" {
		return payee.Payee{}, err
	}
	ps, err := uc.d.Payees.ListByHousehold(ctx, householdID)
	if err != nil {
		return payee.Payee{}, err
	}
	return pickRef("payee", field, ref, ps, func(pe payee.Payee) (domain.ID, string) { return pe.ID, pe.Name })
}

// Subscription resolves a subscription the principal sees.
func (uc *Resolve) Subscription(ctx context.Context, p Principal, householdID domain.ID, field, ref string, scopes ...bot.Scope) (subscription.Subscription, error) {
	s, err := uc.scope(ctx, p, householdID, scopes)
	if err != nil || strings.TrimSpace(ref) == "" {
		return subscription.Subscription{}, err
	}
	subs, err := uc.d.visibleSubscriptions(ctx, s, p)
	if err != nil {
		return subscription.Subscription{}, err
	}
	return pickRef("subscription", field, ref, subs, func(sub subscription.Subscription) (domain.ID, string) { return sub.ID, sub.Name })
}

// pickRef chooses among the candidates the one ref names.
func pickRef[T any](kind, field, ref string, items []T, key func(T) (domain.ID, string)) (T, error) {
	var zero T
	ref = strings.TrimSpace(ref)
	for _, it := range items {
		if id, _ := key(it); string(id) == ref {
			return it, nil
		}
	}
	var exact, partial []T
	lower := strings.ToLower(ref)
	for _, it := range items {
		_, name := key(it)
		switch {
		case strings.EqualFold(name, ref):
			exact = append(exact, it)
		case strings.Contains(strings.ToLower(name), lower):
			partial = append(partial, it)
		}
	}
	for _, found := range [][]T{exact, partial} {
		switch {
		case len(found) == 1:
			return found[0], nil
		case len(found) > 1:
			names := make([]string, 0, min(len(found), maxCandidates))
			for _, it := range found[:min(len(found), maxCandidates)] {
				_, name := key(it)
				names = append(names, fmt.Sprintf("%q", name))
			}
			more := ""
			if len(found) > maxCandidates {
				more = fmt.Sprintf(" and %d more", len(found)-maxCandidates)
			}
			return zero, domain.ValidationError{Field: field, Message: fmt.Sprintf("is ambiguous: %q matches %s%s; give the id or the full name",
				ref, strings.Join(names, ", "), more)}
		}
	}
	return zero, RefNotFound{Kind: kind, Field: field}
}
