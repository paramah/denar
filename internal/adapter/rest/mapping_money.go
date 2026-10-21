package rest

import (
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"git.eiac.dev/tools/denar/internal/adapter/rest/api"
	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/money"
	"git.eiac.dev/tools/denar/internal/domain/payee"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func apiMoney(m money.Money) api.Money {
	return api.Money{Amount: m.String(), Currency: m.Currency.Code, Minor: m.Amount}
}

// minor is an amount of minor units in the currency with the code.
func minor(amount int64, code string) api.Money {
	c, _ := money.LookupCurrency(code)
	return apiMoney(money.Money{Amount: amount, Currency: c})
}

func moneyInput(m api.MoneyInput) usecase.MoneyInput {
	return usecase.MoneyInput{Amount: m.Amount, Currency: m.Currency}
}

func moneyInputPtr(m *api.MoneyInput) *usecase.MoneyInput {
	if m == nil {
		return nil
	}
	in := moneyInput(*m)
	return &in
}

func apiDate(d domain.Date) openapi_types.Date { return openapi_types.Date{Time: d.Time()} }

func fromAPIDate(d *openapi_types.Date) domain.Date {
	if d == nil {
		return domain.Date{}
	}
	return domain.NewDate(d.Year(), d.Month(), d.Day())
}

func nullableString(s string) nullable.Nullable[string] {
	if s == "" {
		return nullable.NewNullNullable[string]()
	}
	return nullable.NewNullableWithValue(s)
}

func nullableID(id domain.ID) nullable.Nullable[string] { return nullableString(string(id)) }

// optionalNullable reads a field that may be absent, null (stored as "")
// or a value.
func optionalNullable(n nullable.Nullable[string]) domain.Optional[string] {
	if !n.IsSpecified() {
		return domain.Optional[string]{}
	}
	if n.IsNull() {
		return domain.Some("")
	}
	return domain.Some(n.MustGet())
}

func apiWallet(v usecase.WalletView) api.Wallet {
	w := v.Wallet
	members := make([]string, len(w.MemberIDs))
	for i, id := range w.MemberIDs {
		members[i] = string(id)
	}
	return api.Wallet{
		Id: string(w.ID), Name: w.Name, Type: api.WalletType(w.Type), Currency: w.Currency, OnBudget: w.OnBudget,
		Visibility: api.Visibility(w.Visibility), MemberIds: members, CreatedBy: string(w.CreatedBy), CreatedAt: w.CreatedAt,
		ArchivedAt: nullableTime(w.ArchivedAt), Balance: apiMoney(v.Balance), ClearedBalance: apiMoney(v.Cleared),
		BaseBalance: apiMoney(v.Base), RatePending: v.BasePending,
	}
}

func apiTransaction(v usecase.TransactionView) api.Transaction {
	t, baseCurrency := v.Transaction, v.BaseCurrency
	base := nullable.NewNullNullable[api.Money]()
	if !t.RatePending {
		base = nullable.NewNullableWithValue(minor(t.BaseAmount, baseCurrency))
	}
	splits := make([]api.Split, len(t.Splits))
	for i, s := range t.Splits {
		sb := nullable.NewNullNullable[api.Money]()
		if !t.RatePending {
			sb = nullable.NewNullableWithValue(minor(s.BaseAmount, baseCurrency))
		}
		splits[i] = api.Split{EnvelopeId: nullableID(s.EnvelopeID), Amount: minor(s.Amount, t.Currency), BaseAmount: sb, Note: s.Note}
	}
	tags := t.Tags
	if tags == nil {
		tags = []string{}
	}
	return api.Transaction{
		Id: string(t.ID), WalletId: string(t.WalletID), Date: apiDate(t.Date), Amount: minor(t.Amount, t.Currency),
		BaseAmount: base, FxRate: nullableString(t.FxRate), RatePending: t.RatePending, PayeeId: nullableID(t.PayeeID),
		PayeeName: nullableString(v.PayeeName), EnvelopeId: nullableID(t.EnvelopeID), Kind: api.TransactionKind(t.Kind),
		Status: api.TransactionStatus(t.Status), Source: api.TransactionSource(t.Source), Tags: tags, Note: t.Note,
		ExternalId: nullableString(t.ExternalID), TransferPairId: nullableID(t.TransferPairID),
		TransferWalletId: nullableID(t.TransferWalletID), Splits: splits,
		CreatedBy: api.Actor{Kind: api.ActorKind(t.CreatedBy.Kind), Id: string(t.CreatedBy.ID)},
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func splitInputs(in *[]api.SplitInput) []usecase.SplitInput {
	if in == nil {
		return nil
	}
	out := make([]usecase.SplitInput, len(*in))
	for i, s := range *in {
		out[i] = usecase.SplitInput{EnvelopeID: deref(s.EnvelopeId), Amount: moneyInput(s.Amount), Note: deref(s.Note)}
	}
	return out
}

func apiPayee(p payee.Payee) api.Payee {
	patterns := p.Patterns
	if patterns == nil {
		patterns = []string{}
	}
	return api.Payee{Id: string(p.ID), Name: p.Name, Patterns: patterns, DefaultEnvelopeId: nullableID(p.DefaultEnvelopeID),
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

func apiFxQuote(q usecase.FxQuote) api.FxQuote {
	out := api.FxQuote{Currency: q.Currency, BaseCurrency: q.BaseCurrency, Date: apiDate(q.Date), Source: api.FxSource(q.Source),
		Rate: nullable.NewNullNullable[string](), RateDate: nullable.NewNullNullable[openapi_types.Date]()}
	if q.Rate != nil {
		out.Rate.Set(money.FormatRate(q.Rate))
		out.RateDate.Set(apiDate(q.RateDate))
	}
	return out
}

func apiFxOverride(o usecase.FxOverride) api.FxOverride {
	return api.FxOverride{Currency: o.Currency, Date: apiDate(o.Date), Rate: o.Rate,
		CreatedBy: api.Actor{Kind: api.ActorKind(o.CreatedBy.Kind), Id: string(o.CreatedBy.ID)}, CreatedAt: o.CreatedAt}
}
