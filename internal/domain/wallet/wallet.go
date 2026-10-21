// Package wallet models where money is kept: cash, accounts, cards,
// savings, investments and loans, each in one currency.
package wallet

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/money"
)

// Type is the kind of a wallet.
type Type string

const (
	TypeCash       Type = "cash"
	TypeChecking   Type = "checking"
	TypeCard       Type = "card"
	TypeSavings    Type = "savings"
	TypeInvestment Type = "investment"
	TypeLoan       Type = "loan"
)

// Types is the closed list of wallet types.
var Types = []Type{TypeCash, TypeChecking, TypeCard, TypeSavings, TypeInvestment, TypeLoan}

func (t Type) Valid() bool { return slices.Contains(Types, t) }

// Visibility says who sees a wallet.
type Visibility string

const (
	// Shared wallets are seen by every member of the household.
	Shared Visibility = "shared"
	// Private wallets are seen only by the members on their list; the
	// creator is always on it. Roles do not open them: an owner does not
	// see someone else's private wallet.
	Private Visibility = "private"
)

func (v Visibility) Valid() bool { return v == Shared || v == Private }

// Limits of a wallet.
const (
	MaxNameLength = 100
	MaxMembers    = 50
)

// Wallet holds money in one currency. Its balance is the sum of its
// transactions, the opening one included; the currency never changes.
type Wallet struct {
	ID          domain.ID
	HouseholdID domain.ID
	Name        string
	Type        Type
	Currency    string
	// OnBudget wallets take part in the envelope budget (stage 003);
	// others are only tracked.
	OnBudget   bool
	Visibility Visibility
	// MemberIDs are who sees a private wallet, sorted; empty for a shared one.
	MemberIDs  []domain.ID
	CreatedBy  domain.ID
	CreatedAt  time.Time
	ArchivedAt time.Time // zero while in use
}

// Normalize trims the name and puts the member list in shape: none for a
// shared wallet; for a private one, without duplicates, sorted, with the
// creator.
func (w Wallet) Normalize() Wallet {
	w.Name = strings.TrimSpace(w.Name)
	if w.Visibility != Private {
		w.MemberIDs = []domain.ID{}
		return w
	}
	ids := append(slices.Clone(w.MemberIDs), w.CreatedBy)
	slices.Sort(ids)
	w.MemberIDs = slices.Compact(ids)
	return w
}

// Validate checks the fields people set.
func (w Wallet) Validate() error {
	var errs []error
	if n := utf8.RuneCountInString(w.Name); n == 0 || n > MaxNameLength {
		errs = append(errs, domain.ValidationError{Field: "name", Message: "must be 1 to 100 characters"})
	}
	if !w.Type.Valid() {
		errs = append(errs, domain.ValidationError{Field: "type", Message: "must be one of cash, checking, card, savings, investment, loan"})
	}
	if err := money.ValidateCurrency("currency", w.Currency); err != nil {
		errs = append(errs, err)
	}
	if !w.Visibility.Valid() {
		errs = append(errs, domain.ValidationError{Field: "visibility", Message: "must be shared or private"})
	}
	if len(w.MemberIDs) > MaxMembers {
		errs = append(errs, domain.ValidationError{Field: "member_ids", Message: "may name at most 50 members"})
	}
	return errors.Join(errs...)
}

// VisibleTo reports whether the person sees the wallet.
func (w Wallet) VisibleTo(userID domain.ID) bool {
	return w.Visibility == Shared || slices.Contains(w.MemberIDs, userID)
}

// Archived reports whether the wallet is archived.
func (w Wallet) Archived() bool { return !w.ArchivedAt.IsZero() }
