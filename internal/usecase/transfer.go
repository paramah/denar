package usecase

import (
	"context"
	"errors"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/transaction"
)

type CreateTransferInput struct {
	FromWalletID string
	ToWalletID   string
	Date         domain.Date // zero for today
	// Amount leaves FromWallet, in its currency; it must be positive.
	Amount MoneyInput
	// ToAmount reaches ToWallet, in its currency. It is required when the
	// currencies differ and must equal Amount when they do not.
	ToAmount *MoneyInput
	// EnvelopeID categorises a transfer between an on-budget and an
	// off-budget wallet; it goes on the on-budget leg.
	EnvelopeID string
	Status     string
	Tags       []string
	Note       string
}

// TransferView is the two legs of a transfer.
type TransferView struct {
	From, To TransactionView
}

// CreateTransfer moves money between two wallets the principal sees: two
// transactions of kind transfer sharing a pair id, written together
// (FR-132).
type CreateTransfer struct{ d Deps }

func NewCreateTransfer(d Deps) *CreateTransfer { return &CreateTransfer{d: d} }

func (uc *CreateTransfer) Do(ctx context.Context, p Principal, householdID domain.ID, in CreateTransferInput) (TransferView, error) {
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return TransferView{}, err
	}
	if err := requireScope(s, bot.ScopeTransactionsWrite); err != nil {
		return TransferView{}, err
	}
	fromID, toID := domain.ID(strings.TrimSpace(in.FromWalletID)), domain.ID(strings.TrimSpace(in.ToWalletID))
	if fromID == toID {
		return TransferView{}, domain.ValidationError{Field: "to_wallet_id", Message: "must differ from from_wallet_id"}
	}
	from, err := uc.d.writableWallet(ctx, s, p, fromID)
	if err != nil {
		return TransferView{}, err
	}
	to, err := uc.d.writableWallet(ctx, s, p, toID)
	if err != nil {
		return TransferView{}, err
	}
	out, outErr := parseMoney("amount", in.Amount, from.Currency)
	if outErr == nil && out.Sign() <= 0 {
		outErr = domain.ValidationError{Field: "amount.amount", Message: "must be greater than zero"}
	}
	var errs []error
	errs = append(errs, outErr)
	inAmount := out.Amount
	switch {
	case in.ToAmount != nil:
		m, err := parseMoney("to_amount", *in.ToAmount, to.Currency)
		switch {
		case err != nil:
			errs = append(errs, err)
		case m.Sign() <= 0:
			errs = append(errs, domain.ValidationError{Field: "to_amount.amount", Message: "must be greater than zero"})
		case from.Currency == to.Currency && m.Amount != out.Amount:
			errs = append(errs, domain.ValidationError{Field: "to_amount.amount", Message: "must equal amount between wallets of one currency"})
		}
		inAmount = m.Amount
	case from.Currency != to.Currency:
		errs = append(errs, domain.ValidationError{Field: "to_amount", Message: "is required between wallets of different currencies"})
	}
	envelope := domain.ID(strings.TrimSpace(in.EnvelopeID))
	if envelope != "" && from.OnBudget == to.OnBudget {
		errs = append(errs, domain.ValidationError{Field: "envelope_id",
			Message: "only a transfer between an on-budget and an off-budget wallet has an envelope"})
	}
	if err := errors.Join(errs...); err != nil {
		return TransferView{}, err
	}
	now := uc.d.Clock.Now()
	date := in.Date
	if date.IsZero() {
		date = today(s.Household, now)
	}
	pair := uc.d.IDs.NewID()
	leg := func(walletID, otherID domain.ID, currency string, amount int64, onBudget bool) transaction.Transaction {
		t := transaction.Transaction{ID: uc.d.IDs.NewID(), HouseholdID: householdID, WalletID: walletID, Date: date,
			Amount: amount, Currency: currency, Kind: transaction.Transfer,
			Status: transaction.Status(cmpOr(in.Status, string(transaction.Cleared))), Source: sourceOf(p), Tags: in.Tags,
			Note: in.Note, TransferPairID: pair, TransferWalletID: otherID, CreatedBy: p.Actor(), CreatedAt: now, UpdatedAt: now}
		if onBudget {
			t.EnvelopeID = envelope
		}
		return t.Normalize()
	}
	legs := []transaction.Transaction{
		leg(from.ID, to.ID, from.Currency, -out.Amount, from.OnBudget),
		leg(to.ID, from.ID, to.Currency, inAmount, to.OnBudget),
	}
	for i := range legs {
		if err := legs[i].Validate(); err != nil {
			return TransferView{}, err
		}
		if legs[i], err = uc.d.fx().withBase(ctx, s.Household, legs[i]); err != nil {
			return TransferView{}, err
		}
	}
	var view TransferView
	err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireScope(s, bot.ScopeTransactionsWrite); err != nil {
			return err
		}
		for _, l := range legs {
			if _, err := uc.d.writableWallet(ctx, s, p, l.WalletID); err != nil {
				return err
			}
		}
		for _, l := range legs {
			if err := insertTransaction(ctx, uc.d, s.Household.BaseCurrency, l, p.Actor()); err != nil {
				return err
			}
		}
		v, err := uc.d.viewer(ctx, s, p)
		if err != nil {
			return err
		}
		view = TransferView{From: v.view(legs[0]), To: v.view(legs[1])}
		return nil
	})
	return view, err
}
