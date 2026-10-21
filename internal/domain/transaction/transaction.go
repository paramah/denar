// Package transaction models money moving in or out of a wallet: expenses,
// income, the two legs of a transfer, and the split of an expense across
// envelopes.
package transaction

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/money"
)

// Kind says what a transaction is for the budget.
type Kind string

const (
	// Expense is spending, or a refund of it (positive), against an
	// envelope or uncategorised.
	Expense Kind = "expense"
	// Income goes to Ready-to-Assign; it has no envelope.
	Income Kind = "income"
	// Transfer is one leg of a move between two wallets.
	Transfer Kind = "transfer"
)

var Kinds = []Kind{Expense, Income, Transfer}

func (k Kind) Valid() bool { return slices.Contains(Kinds, k) }

// Status is how far the bank has confirmed a transaction.
type Status string

const (
	Pending    Status = "pending"
	Cleared    Status = "cleared"
	Reconciled Status = "reconciled"
)

var Statuses = []Status{Pending, Cleared, Reconciled}

func (s Status) Valid() bool { return slices.Contains(Statuses, s) }

// Source says where a transaction came from.
type Source string

const (
	SourceManual       Source = "manual"
	SourceImport       Source = "import"
	SourceSubscription Source = "subscription"
	SourceBot          Source = "bot"
)

var Sources = []Source{SourceManual, SourceImport, SourceSubscription, SourceBot}

func (s Source) Valid() bool { return slices.Contains(Sources, s) }

// Limits of a transaction.
const (
	MaxNoteLength       = 1000
	MaxExternalIDLength = 200
	MaxTags             = 20
	MaxTagLength        = 40
	MaxSplits           = 50
	MaxEnvelopeIDLength = 64
)

// Split is one line of a split expense: part of the amount for one
// envelope.
type Split struct {
	EnvelopeID domain.ID // "" uncategorised
	Amount     int64     // minor units of the wallet's currency
	BaseAmount int64     // minor units of the base currency; meaningless while the rate is pending
	Note       string
}

// Transaction is one entry of a wallet.
type Transaction struct {
	ID          domain.ID
	HouseholdID domain.ID
	WalletID    domain.ID
	Date        domain.Date
	// Amount is in minor units of the wallet's currency: negative out,
	// positive in. The sign does not depend on the kind.
	Amount   int64
	Currency string // the wallet's currency
	// BaseAmount is Amount in the household's base currency at the rate of
	// the transaction's day, kept as it was then. It is unknown while
	// RatePending.
	BaseAmount  int64
	RatePending bool
	// FxRate is the rate used, for people to see ("" for the base currency
	// and while pending).
	FxRate     string
	PayeeID    domain.ID
	EnvelopeID domain.ID
	Kind       Kind
	Status     Status
	Source     Source
	Tags       []string
	Note       string
	// ExternalID identifies the transaction at its source (a bank, a file,
	// a bot); unique within the wallet.
	ExternalID string
	// TransferPairID is shared by the two legs of a transfer;
	// TransferWalletID is the wallet of the other leg.
	TransferPairID   domain.ID
	TransferWalletID domain.ID
	Splits           []Split
	CreatedBy        domain.Actor
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IsTransfer reports whether the transaction is a leg of a transfer.
func (t Transaction) IsTransfer() bool { return t.Kind == Transfer }

// NormalizeTags trims, lower-cases, removes duplicates and sorts tags.
func NormalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		out = append(out, strings.ToLower(strings.TrimSpace(t)))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Normalize puts the free-text fields in shape.
func (t Transaction) Normalize() Transaction {
	t.Tags = NormalizeTags(t.Tags)
	t.Note = strings.TrimSpace(t.Note)
	t.ExternalID = strings.TrimSpace(t.ExternalID)
	t.Splits = slices.Clone(t.Splits)
	for i := range t.Splits {
		t.Splits[i].Note = strings.TrimSpace(t.Splits[i].Note)
	}
	return t
}

func invalid(field, msg string) error { return domain.ValidationError{Field: field, Message: msg} }

// Validate checks a normalised transaction.
func (t Transaction) Validate() error {
	var errs []error
	if t.Date.IsZero() {
		errs = append(errs, invalid("date", "is required"))
	}
	if !t.Kind.Valid() {
		errs = append(errs, invalid("kind", "must be expense, income or transfer"))
	}
	if !t.Status.Valid() {
		errs = append(errs, invalid("status", "must be pending, cleared or reconciled"))
	}
	if !t.Source.Valid() {
		errs = append(errs, invalid("source", "must be manual, import, subscription or bot"))
	}
	if utf8.RuneCountInString(t.Note) > MaxNoteLength {
		errs = append(errs, invalid("note", "must be at most 1000 characters"))
	}
	if utf8.RuneCountInString(t.ExternalID) > MaxExternalIDLength {
		errs = append(errs, invalid("external_id", "must be at most 200 characters"))
	}
	if len(t.EnvelopeID) > MaxEnvelopeIDLength {
		errs = append(errs, invalid("envelope_id", "is not an envelope id"))
	}
	errs = append(errs, validateTags(t.Tags))
	switch t.Kind {
	case Income:
		if t.EnvelopeID != "" {
			errs = append(errs, invalid("envelope_id", "income goes to Ready-to-Assign and has no envelope"))
		}
		if len(t.Splits) > 0 {
			errs = append(errs, invalid("splits", "only an expense can be split"))
		}
	case Transfer:
		if t.TransferPairID == "" || t.TransferWalletID == "" {
			errs = append(errs, invalid("kind", "a transfer is made with the transfer operation"))
		}
		if len(t.Splits) > 0 {
			errs = append(errs, invalid("splits", "only an expense can be split"))
		}
		if t.PayeeID != "" {
			errs = append(errs, invalid("payee_id", "a transfer has no payee"))
		}
	}
	if len(t.Splits) > 0 && t.Kind == Expense {
		errs = append(errs, t.validateSplits())
	}
	return errors.Join(errs...)
}

func validateTags(tags []string) error {
	if len(tags) > MaxTags {
		return invalid("tags", "may have at most 20 tags")
	}
	for _, tag := range tags {
		if n := utf8.RuneCountInString(tag); n == 0 || n > MaxTagLength {
			return invalid("tags", "must each be 1 to 40 characters")
		}
	}
	return nil
}

func (t Transaction) validateSplits() error {
	var errs []error
	if t.EnvelopeID != "" {
		errs = append(errs, invalid("envelope_id", "a split transaction takes its envelopes from its lines"))
	}
	if len(t.Splits) > MaxSplits {
		errs = append(errs, invalid("splits", "may have at most 50 lines"))
	}
	var sum int64
	overflow := false
	for i, s := range t.Splits {
		var ok bool
		if sum, ok = money.AddInt64(sum, s.Amount); !ok {
			overflow = true
		}
		if len(s.EnvelopeID) > MaxEnvelopeIDLength {
			errs = append(errs, invalid(fmt.Sprintf("splits[%d].envelope_id", i), "is not an envelope id"))
		}
		if utf8.RuneCountInString(s.Note) > MaxNoteLength {
			errs = append(errs, invalid(fmt.Sprintf("splits[%d].note", i), "must be at most 1000 characters"))
		}
	}
	if overflow || sum != t.Amount {
		errs = append(errs, invalid("splits", "must add up to the amount of the transaction"))
	}
	return errors.Join(errs...)
}
