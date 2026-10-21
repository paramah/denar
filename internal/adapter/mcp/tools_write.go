package mcp

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/budget"
	"github.com/paramah/denar/internal/domain/wallet"
	"github.com/paramah/denar/internal/usecase"
)

type recordExpenseIn struct {
	Wallet   string   `json:"wallet" jsonschema:"wallet paid from, by id or name, such as Karta"`
	Amount   string   `json:"amount" jsonschema:"amount spent, a positive decimal such as \"42.50\""`
	Currency string   `json:"currency,omitempty" jsonschema:"ISO 4217 code; must be the wallet's currency, which is the default"`
	Date     string   `json:"date,omitempty" jsonschema:"day of the expense, YYYY-MM-DD; today in the household's time zone by default"`
	Payee    string   `json:"payee,omitempty" jsonschema:"where the money went, such as Biedronka; found by name or pattern, created when new"`
	Envelope string   `json:"envelope,omitempty" jsonschema:"budget envelope, by id or name; the payee's default envelope when omitted"`
	Note     string   `json:"note,omitempty" jsonschema:"free text"`
	Tags     []string `json:"tags,omitempty" jsonschema:"tags, such as [\"vacation\"]"`
	Status   string   `json:"status,omitempty" jsonschema:"cleared (the default), pending or reconciled"`
}

type recordIncomeIn struct {
	Wallet   string   `json:"wallet" jsonschema:"wallet the money came to, by id or name"`
	Amount   string   `json:"amount" jsonschema:"amount received, a positive decimal such as \"5200.00\""`
	Currency string   `json:"currency,omitempty" jsonschema:"ISO 4217 code; must be the wallet's currency, which is the default"`
	Date     string   `json:"date,omitempty" jsonschema:"day received, YYYY-MM-DD; today by default"`
	Payee    string   `json:"payee,omitempty" jsonschema:"who paid, such as the employer; found by name or pattern, created when new"`
	Note     string   `json:"note,omitempty" jsonschema:"free text"`
	Tags     []string `json:"tags,omitempty" jsonschema:"tags"`
	Status   string   `json:"status,omitempty" jsonschema:"cleared (the default), pending or reconciled"`
}

type expenseOut struct {
	Transaction transactionOut `json:"transaction"`
	// ChargeSuggestions are subscription charges the expense may pay, best
	// first; null when the bot may not see them.
	ChargeSuggestions *[]chargeSuggestionOut `json:"charge_suggestions"`
}

type transferIn struct {
	FromWallet string   `json:"from_wallet" jsonschema:"wallet the money leaves, by id or name"`
	ToWallet   string   `json:"to_wallet" jsonschema:"wallet the money reaches, by id or name"`
	Amount     string   `json:"amount" jsonschema:"amount leaving from_wallet, a positive decimal in its currency"`
	ToAmount   string   `json:"to_amount,omitempty" jsonschema:"amount reaching to_wallet in its currency; required when the currencies differ"`
	Date       string   `json:"date,omitempty" jsonschema:"day of the transfer, YYYY-MM-DD; today by default"`
	Envelope   string   `json:"envelope,omitempty" jsonschema:"envelope for a transfer between an on-budget and an off-budget wallet"`
	Note       string   `json:"note,omitempty" jsonschema:"free text"`
	Tags       []string `json:"tags,omitempty" jsonschema:"tags"`
}

type transferOut struct {
	From transactionOut `json:"from"`
	To   transactionOut `json:"to"`
}

type assignIn struct {
	Envelope string `json:"envelope" jsonschema:"envelope, by id or name"`
	Amount   string `json:"amount" jsonschema:"the envelope's assigned amount for the month in the base currency, such as \"800.00\"; it replaces the current one, 0 removes it"`
	Month    string `json:"month,omitempty" jsonschema:"budget month, YYYY-MM; the current one by default"`
}

type moveIn struct {
	FromEnvelope string `json:"from_envelope" jsonschema:"envelope money is taken from, by id or name"`
	ToEnvelope   string `json:"to_envelope" jsonschema:"envelope money goes to, by id or name"`
	Amount       string `json:"amount" jsonschema:"amount moved in the base currency, a positive decimal such as \"50.00\""`
	Month        string `json:"month,omitempty" jsonschema:"budget month, YYYY-MM; the current one by default"`
}

type assignedOut struct {
	Month         string             `json:"month"`
	ReadyToAssign moneyOut           `json:"ready_to_assign"`
	Envelope      *budgetEnvelopeOut `json:"envelope,omitempty"`
	From          *budgetEnvelopeOut `json:"from,omitempty"`
	To            *budgetEnvelopeOut `json:"to,omitempty"`
}

func (s *server) addWriteTools(m *sdk.Server) {
	addTool(s, m, "record_expense",
		"Record money spent from a wallet. Give the amount as a positive decimal; the date defaults to today in the household. "+
			"The answer has the transaction and, when you may read subscriptions and transactions, the subscription charges it may pay "+
			"(confirm one with confirm_charge_match). Needs transactions:write.",
		false, func(ctx context.Context, p usecase.Principal, hid domain.ID, in recordExpenseIn) (any, error) {
			scope := bot.ScopeTransactionsWrite
			w, err := s.requiredWallet(ctx, p, hid, "wallet", in.Wallet, scope)
			if err != nil {
				return nil, err
			}
			e, err := s.uc.resolve.Envelope(ctx, p, hid, "envelope", in.Envelope, scope)
			if err != nil {
				return nil, err
			}
			date, err1 := parseDate("date", in.Date)
			amount, err2 := signedAmount("amount", in.Amount, in.Currency, w, -1)
			if err := errors.Join(err1, err2); err != nil {
				return nil, err
			}
			v, err := s.uc.recordTransaction.Do(ctx, p, hid, usecase.RecordTransactionInput{WalletID: string(w.ID), Date: date,
				Amount: amount, PayeeName: in.Payee, EnvelopeID: string(e.ID), Kind: "expense", Status: in.Status, Tags: in.Tags,
				Note: in.Note})
			if err != nil {
				return nil, err
			}
			out := expenseOut{Transaction: transactionView(v)}
			suggestions, err := s.uc.suggestCharges.Do(ctx, p, hid, v.Transaction.ID)
			switch {
			case err == nil:
				list := []chargeSuggestionOut{}
				for _, sg := range suggestions {
					list = append(list, chargeSuggestionOut{Charge: chargeView(sg.Charge), Score: sg.Score})
				}
				out.ChargeSuggestions = &list
			case errors.Is(err, domain.ErrForbidden):
			default:
				// The expense is recorded; the suggestions are a bonus.
				s.log.LogAttrs(ctx, slog.LevelWarn, "charge suggestions failed",
					slog.String("bot_id", string(p.Bot.ID)), slog.String("error", err.Error()))
			}
			return out, nil
		})

	addTool(s, m, "record_income",
		"Record money received into a wallet, such as a salary; it goes to Ready to Assign of the budget. Give the amount as a positive "+
			"decimal. Needs transactions:write.",
		false, func(ctx context.Context, p usecase.Principal, hid domain.ID, in recordIncomeIn) (any, error) {
			w, err := s.requiredWallet(ctx, p, hid, "wallet", in.Wallet, bot.ScopeTransactionsWrite)
			if err != nil {
				return nil, err
			}
			date, err1 := parseDate("date", in.Date)
			amount, err2 := signedAmount("amount", in.Amount, in.Currency, w, +1)
			if err := errors.Join(err1, err2); err != nil {
				return nil, err
			}
			v, err := s.uc.recordTransaction.Do(ctx, p, hid, usecase.RecordTransactionInput{WalletID: string(w.ID), Date: date,
				Amount: amount, PayeeName: in.Payee, Kind: "income", Status: in.Status, Tags: in.Tags, Note: in.Note})
			if err != nil {
				return nil, err
			}
			return transactionView(v), nil
		})

	addTool(s, m, "transfer",
		"Move money between two of your wallets: two linked transactions, one out and one in. Between currencies give to_amount too. "+
			"Needs transactions:write.",
		false, func(ctx context.Context, p usecase.Principal, hid domain.ID, in transferIn) (any, error) {
			scope := bot.ScopeTransactionsWrite
			from, err := s.requiredWallet(ctx, p, hid, "from_wallet", in.FromWallet, scope)
			if err != nil {
				return nil, err
			}
			to, err := s.requiredWallet(ctx, p, hid, "to_wallet", in.ToWallet, scope)
			if err != nil {
				return nil, err
			}
			e, err := s.uc.resolve.Envelope(ctx, p, hid, "envelope", in.Envelope, scope)
			if err != nil {
				return nil, err
			}
			date, err := parseDate("date", in.Date)
			if err != nil {
				return nil, err
			}
			input := usecase.CreateTransferInput{FromWalletID: string(from.ID), ToWalletID: string(to.ID), Date: date,
				Amount: usecase.MoneyInput{Amount: decimal(in.Amount), Currency: from.Currency}, EnvelopeID: string(e.ID),
				Tags: in.Tags, Note: in.Note}
			if strings.TrimSpace(in.ToAmount) != "" {
				input.ToAmount = &usecase.MoneyInput{Amount: decimal(in.ToAmount), Currency: to.Currency}
			} else if from.Currency == to.Currency {
				input.ToAmount = &usecase.MoneyInput{Amount: decimal(in.Amount), Currency: to.Currency}
			}
			v, err := s.uc.createTransfer.Do(ctx, p, hid, input)
			if err != nil {
				return nil, err
			}
			return transferOut{From: transactionView(v.From), To: transactionView(v.To)}, nil
		})

	addTool(s, m, "assign_to_envelope",
		"Set how much an envelope is given in a budget month, in the base currency; the amount replaces what was assigned before "+
			"(to add 50 to 200, assign 250). The answer shows the envelope and Ready to Assign after the change. Needs budget:write.",
		false, func(ctx context.Context, p usecase.Principal, hid domain.ID, in assignIn) (any, error) {
			e, err := s.requiredEnvelope(ctx, p, hid, "envelope", in.Envelope)
			if err != nil {
				return nil, err
			}
			base, err := s.baseCurrency(ctx, p)
			if err != nil {
				return nil, err
			}
			v, err := s.uc.setAssignment.Do(ctx, p, hid, e.ID, usecase.SetAssignmentInput{Month: strings.TrimSpace(in.Month),
				Amount: usecase.MoneyInput{Amount: decimal(in.Amount), Currency: base}})
			if err != nil {
				return nil, err
			}
			month := budgetMonthView(v)
			return assignedOut{Month: month.Month, ReadyToAssign: month.ReadyToAssign, Envelope: envelopeLine(month, e.ID)}, nil
		})

	addTool(s, m, "move_between_envelopes",
		"Move assigned money from one envelope to another in a budget month, in the base currency, for example to cover overspending. "+
			"Needs budget:write.",
		false, func(ctx context.Context, p usecase.Principal, hid domain.ID, in moveIn) (any, error) {
			from, err := s.requiredEnvelope(ctx, p, hid, "from_envelope", in.FromEnvelope)
			if err != nil {
				return nil, err
			}
			to, err := s.requiredEnvelope(ctx, p, hid, "to_envelope", in.ToEnvelope)
			if err != nil {
				return nil, err
			}
			base, err := s.baseCurrency(ctx, p)
			if err != nil {
				return nil, err
			}
			v, err := s.uc.moveBetweenEnvelopes.Do(ctx, p, hid, usecase.MoveBetweenEnvelopesInput{Month: strings.TrimSpace(in.Month),
				FromEnvelopeID: string(from.ID), ToEnvelopeID: string(to.ID), Amount: usecase.MoneyInput{Amount: decimal(in.Amount), Currency: base}})
			if err != nil {
				return nil, err
			}
			month := budgetMonthView(v)
			return assignedOut{Month: month.Month, ReadyToAssign: month.ReadyToAssign, From: envelopeLine(month, from.ID),
				To: envelopeLine(month, to.ID)}, nil
		})
}

// requiredWallet resolves a wallet that must be named.
func (s *server) requiredWallet(ctx context.Context, p usecase.Principal, hid domain.ID, field, ref string, scopes ...bot.Scope) (wallet.Wallet, error) {
	w, err := s.uc.resolve.Wallet(ctx, p, hid, field, ref, scopes...)
	if err == nil && w.ID == "" {
		err = domain.ValidationError{Field: field, Message: "is required"}
	}
	return w, err
}

// requiredEnvelope resolves an envelope that must be named, for a budget
// write.
func (s *server) requiredEnvelope(ctx context.Context, p usecase.Principal, hid domain.ID, field, ref string) (budget.Envelope, error) {
	e, err := s.uc.resolve.Envelope(ctx, p, hid, field, ref, bot.ScopeBudgetWrite)
	if err == nil && e.ID == "" {
		err = domain.ValidationError{Field: field, Message: "is required"}
	}
	return e, err
}

// baseCurrency is the base currency of the bot's household.
func (s *server) baseCurrency(ctx context.Context, p usecase.Principal) (string, error) {
	c, err := s.uc.whoami.Do(ctx, p)
	return c.Household.BaseCurrency, err
}

// signedAmount reads a positive amount of the wallet's currency and gives
// it the sign of the direction: -1 for money out, +1 for money in (AD-608).
func signedAmount(field, amount, currency string, w wallet.Wallet, sign int) (usecase.MoneyInput, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency != "" && currency != w.Currency {
		return usecase.MoneyInput{}, domain.ValidationError{Field: "currency", Message: "must be " + w.Currency + ", the currency of the wallet"}
	}
	m, err := parseAmount(field, amount, w.Currency)
	if err != nil {
		return usecase.MoneyInput{}, err
	}
	if m.Amount <= 0 {
		return usecase.MoneyInput{}, domain.ValidationError{Field: field, Message: "must be greater than zero, without a sign"}
	}
	if sign < 0 {
		m = m.Neg()
	}
	return usecase.MoneyInput{Amount: m.String(), Currency: w.Currency}, nil
}
