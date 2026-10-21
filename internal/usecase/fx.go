package usecase

import (
	"context"
	"errors"
	"log/slog"
	"math/big"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/audit"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/household"
	"github.com/paramah/denar/internal/domain/membership"
	"github.com/paramah/denar/internal/domain/money"
	"github.com/paramah/denar/internal/domain/transaction"
)

// FxSource says where a rate came from.
type FxSource string

const (
	// FxIdentity: the currency is the base currency.
	FxIdentity FxSource = "identity"
	// FxManual: a rate the household set for the day.
	FxManual FxSource = "manual"
	// FxNBP: NBP table A, crossed through PLN when the base is not PLN.
	FxNBP FxSource = "nbp"
	// FxUnavailable: no rate is known for the day yet.
	FxUnavailable FxSource = "unavailable"
)

// FxQuote is the rate of a currency to a household's base currency on a
// day: one unit of Currency is Rate units of BaseCurrency.
type FxQuote struct {
	Currency     string
	BaseCurrency string
	Date         domain.Date
	Rate         *big.Rat // nil when unavailable
	// RateDate is the business day the rate is from (for NBP the newest one
	// on or before Date).
	RateDate domain.Date
	Source   FxSource
}

// rateWindow is how many days before the date a rate may come from: NBP
// publishes on business days only.
const rateWindow = 6

// fxSystemActor fills in rates in the background.
var fxSystemActor = domain.Actor{Kind: domain.ActorSystem, ID: "fx"}

type fx struct{ d Deps }

func (d Deps) fx() fx { return fx{d: d} }

// quote finds the rate of currency to the household's base currency on
// the day (FR-110). It returns an error matching ErrRateUnavailable when
// there is none.
func (f fx) quote(ctx context.Context, h household.Household, currency string, date domain.Date) (FxQuote, error) {
	q := FxQuote{Currency: currency, BaseCurrency: h.BaseCurrency, Date: date}
	if currency == h.BaseCurrency {
		q.Rate, q.RateDate, q.Source = big.NewRat(1, 1), date, FxIdentity
		return q, nil
	}
	if f.d.FxOverrides != nil {
		o, err := f.d.FxOverrides.Get(ctx, h.ID, currency, date)
		if found, err := ignoreNotFound(err); err != nil {
			return FxQuote{}, err
		} else if found {
			r, err := money.ParseRate("rate", o.Rate)
			if err != nil {
				return FxQuote{}, err
			}
			q.Rate, q.RateDate, q.Source = r, date, FxManual
			return q, nil
		}
	}
	from, fromDate, err := f.plnRate(ctx, currency, date)
	if err != nil {
		return FxQuote{}, err
	}
	to, toDate, err := f.plnRate(ctx, h.BaseCurrency, date)
	if err != nil {
		return FxQuote{}, err
	}
	q.Rate, q.RateDate, q.Source = money.CrossRate(from, to), fromDate, FxNBP
	if toDate.Before(fromDate) {
		q.RateDate = toDate
	}
	return q, nil
}

// plnRate is how many PLN one unit of the currency cost on the newest
// business day on or before date: from the cache, else from the provider,
// else (offline) the newest cached rate of the window.
func (f fx) plnRate(ctx context.Context, currency string, date domain.Date) (*big.Rat, domain.Date, error) {
	if currency == "PLN" {
		return big.NewRat(1, 1), date, nil
	}
	if f.d.FxRates != nil {
		cached, err := f.d.FxRates.Get(ctx, currency, date)
		if found, err := ignoreNotFound(err); err != nil {
			return nil, domain.Date{}, err
		} else if found {
			return f.parseCached(cached)
		}
	}
	if f.d.Rates != nil {
		rate, rateDate, err := f.d.Rates.PLNRate(ctx, currency, date)
		if err == nil {
			r, perr := money.ParseRate("rate", rate)
			if perr == nil {
				// The rate of a past day is final; today's table may still
				// come out.
				if f.d.FxRates != nil && date.Before(domain.DateOf(f.d.Clock.Now(), time.UTC)) {
					if err := f.d.FxRates.Put(ctx, FxRate{Currency: currency, Date: date, RateDate: rateDate, Rate: rate,
						FetchedAt: f.d.Clock.Now()}); err != nil {
						f.d.log().WarnContext(ctx, "exchange rate not cached", "currency", currency, "error", err.Error())
					}
				}
				return r, rateDate, nil
			}
			err = perr
		}
		f.d.log().LogAttrs(ctx, slog.LevelWarn, "exchange rate unavailable", slog.String("currency", currency),
			slog.String("date", date.String()), slog.String("error", err.Error()))
	}
	if f.d.FxRates != nil {
		cached, err := f.d.FxRates.Latest(ctx, currency, date.AddDays(-rateWindow), date)
		if found, err := ignoreNotFound(err); err != nil {
			return nil, domain.Date{}, err
		} else if found {
			return f.parseCached(cached)
		}
	}
	return nil, domain.Date{}, ErrRateUnavailable
}

func (f fx) parseCached(r FxRate) (*big.Rat, domain.Date, error) {
	rate, err := money.ParseRate("rate", r.Rate)
	if err != nil {
		return nil, domain.Date{}, err
	}
	return rate, r.RateDate, nil
}

// withBase fills in the base amounts of t, of its split lines and the rate
// used, or marks it pending when no rate is known (FR-111).
func (f fx) withBase(ctx context.Context, h household.Household, t transaction.Transaction) (transaction.Transaction, error) {
	q, err := f.quote(ctx, h, t.Currency, t.Date)
	if errors.Is(err, ErrRateUnavailable) {
		return markPending(t), nil
	}
	if err != nil {
		return transaction.Transaction{}, err
	}
	return applyRate(t, h, q)
}

func markPending(t transaction.Transaction) transaction.Transaction {
	t.RatePending, t.BaseAmount, t.FxRate = true, 0, ""
	t.Splits = cloneSplits(t.Splits)
	for i := range t.Splits {
		t.Splits[i].BaseAmount = 0
	}
	return t
}

func cloneSplits(s []transaction.Split) []transaction.Split {
	return append([]transaction.Split{}, s...)
}

var errAmountRange = domain.ValidationError{Field: "amount", Message: "is out of range in the base currency"}

// applyRate converts t into the base currency at the quote.
func applyRate(t transaction.Transaction, h household.Household, q FxQuote) (transaction.Transaction, error) {
	from, ok1 := money.LookupCurrency(t.Currency)
	base, ok2 := money.LookupCurrency(h.BaseCurrency)
	if !ok1 || !ok2 {
		return transaction.Transaction{}, domain.ValidationError{Field: "currency", Message: "is not supported"}
	}
	conv, err := money.Convert(money.Money{Amount: t.Amount, Currency: from}, q.Rate, base)
	if err != nil {
		return transaction.Transaction{}, errAmountRange
	}
	t.BaseAmount, t.RatePending, t.FxRate = conv.Amount, false, ""
	if q.Source != FxIdentity {
		t.FxRate = money.FormatRate(q.Rate)
	}
	t.Splits = cloneSplits(t.Splits)
	if len(t.Splits) > 0 {
		parts := make([]int64, len(t.Splits))
		for i, s := range t.Splits {
			parts[i] = s.Amount
		}
		bases, err := money.ConvertParts(parts, q.Rate, from, base, conv.Amount)
		if err != nil {
			return transaction.Transaction{}, errAmountRange
		}
		for i := range t.Splits {
			t.Splits[i].BaseAmount = bases[i]
		}
	}
	return t, nil
}

// today is the date in the household's time zone.
func today(h household.Household, now time.Time) domain.Date {
	loc, err := time.LoadLocation(h.Timezone)
	if err != nil {
		loc = time.UTC
	}
	return domain.DateOf(now, loc)
}

type QuoteFxRateInput struct {
	Currency string
	Date     domain.Date // zero for today in the household's time zone
}

// QuoteFxRate tells the rate a transaction of the day in the currency
// would get. An unknown rate is a quote with the source unavailable, not an
// error.
type QuoteFxRate struct{ d Deps }

func NewQuoteFxRate(d Deps) *QuoteFxRate { return &QuoteFxRate{d: d} }

func (uc *QuoteFxRate) Do(ctx context.Context, p Principal, householdID domain.ID, in QuoteFxRateInput) (FxQuote, error) {
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return FxQuote{}, err
	}
	if err := requireScope(s, bot.ScopeTransactionsRead); err != nil {
		return FxQuote{}, err
	}
	if err := money.ValidateCurrency("currency", in.Currency); err != nil {
		return FxQuote{}, err
	}
	date := in.Date
	if date.IsZero() {
		date = today(s.Household, uc.d.Clock.Now())
	}
	q, err := uc.d.fx().quote(ctx, s.Household, in.Currency, date)
	if errors.Is(err, ErrRateUnavailable) {
		return FxQuote{Currency: in.Currency, BaseCurrency: s.Household.BaseCurrency, Date: date, Source: FxUnavailable}, nil
	}
	return q, err
}

// ListFxOverrides returns the rates the household set by hand.
type ListFxOverrides struct{ d Deps }

func NewListFxOverrides(d Deps) *ListFxOverrides { return &ListFxOverrides{d: d} }

func (uc *ListFxOverrides) Do(ctx context.Context, p Principal, householdID domain.ID) ([]FxOverride, error) {
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return nil, err
	}
	if err := requireScope(s, bot.ScopeTransactionsRead); err != nil {
		return nil, err
	}
	return uc.d.FxOverrides.List(ctx, householdID)
}

type SetFxOverrideInput struct {
	Currency string
	Date     domain.Date
	Rate     string // base currency units per one unit of Currency
}

// SetFxOverride sets the rate of a currency for one day of the household
// and fills in the transactions of that day and currency waiting for a
// rate (FR-112). Owners and admins only; bots may not. Transactions that
// have a base amount already keep it.
type SetFxOverride struct{ d Deps }

func NewSetFxOverride(d Deps) *SetFxOverride { return &SetFxOverride{d: d} }

func (uc *SetFxOverride) Do(ctx context.Context, p Principal, householdID domain.ID, in SetFxOverrideInput) (FxOverride, error) {
	if err := requireHuman(p); err != nil {
		return FxOverride{}, err
	}
	var out FxOverride
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireRole(s, membership.RoleAdmin); err != nil {
			return err
		}
		var errs []error
		if err := money.ValidateCurrency("currency", in.Currency); err != nil {
			errs = append(errs, err)
		} else if in.Currency == s.Household.BaseCurrency {
			errs = append(errs, domain.ValidationError{Field: "currency", Message: "is the base currency, whose rate is always 1"})
		}
		if in.Date.IsZero() {
			errs = append(errs, domain.ValidationError{Field: "date", Message: "is required"})
		}
		rate, err := money.ParseRate("rate", in.Rate)
		if err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			return errors.Join(errs...)
		}
		old, err := uc.d.FxOverrides.Get(ctx, householdID, in.Currency, in.Date)
		if _, err := ignoreNotFound(err); err != nil {
			return err
		}
		out = FxOverride{HouseholdID: householdID, Currency: in.Currency, Date: in.Date, Rate: money.FormatRate(rate),
			CreatedBy: p.Actor(), CreatedAt: uc.d.Clock.Now()}
		if err := uc.d.FxOverrides.Put(ctx, out); err != nil {
			return err
		}
		var from any
		if old.Rate != "" {
			from = old.Rate
		}
		if err := uc.d.record(ctx, householdID, p.Actor(), "fx_rate.set", audit.ResourceFxRate,
			domain.ID(in.Currency+"/"+in.Date.String()), audit.Changes{"rate": {From: from, To: out.Rate}}); err != nil {
			return err
		}
		pending, err := uc.d.Transactions.ListRatePending(ctx, householdID, maxPendingPerOverride)
		if err != nil {
			return err
		}
		q := FxQuote{Currency: in.Currency, BaseCurrency: s.Household.BaseCurrency, Date: in.Date, Rate: rate, RateDate: in.Date, Source: FxManual}
		for _, t := range pending {
			if t.Currency != in.Currency || t.Date != in.Date {
				continue
			}
			if err := fillRate(ctx, uc.d, s.Household, t, q, p.Actor()); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

// maxPendingPerOverride bounds the transactions one override fills in at
// once; the background job takes the rest.
const maxPendingPerOverride = 1000

// fillRate stores the base amount of a pending transaction and records it.
func fillRate(ctx context.Context, d Deps, h household.Household, t transaction.Transaction, q FxQuote, actor domain.Actor) error {
	filled, err := applyRate(t, h, q)
	if err != nil {
		return err
	}
	filled.UpdatedAt = d.Clock.Now()
	if err := d.Transactions.Update(ctx, filled); err != nil {
		return err
	}
	return d.record(ctx, h.ID, actor, "transaction.rate_filled", audit.ResourceTransaction, t.ID, audit.Changes{}.
		Set("base_amount", formatMinor(filled.BaseAmount, h.BaseCurrency)).Set("fx_rate", filled.FxRate))
}

// pendingBatch is how many waiting transactions one run of the job looks at.
const pendingBatch = 200

// FillPendingRates is the background job of FR-111: it looks for rates of
// transactions saved without one and fills in their base amounts. It
// returns how many it filled.
type FillPendingRates struct{ d Deps }

func NewFillPendingRates(d Deps) *FillPendingRates { return &FillPendingRates{d: d} }

func (uc *FillPendingRates) Do(ctx context.Context) (int, error) {
	pending, err := uc.d.Transactions.ListRatePending(ctx, "", pendingBatch)
	if err != nil {
		return 0, err
	}
	households := map[domain.ID]household.Household{}
	filled := 0
	for _, t := range pending {
		h, ok := households[t.HouseholdID]
		if !ok {
			if h, err = uc.d.Households.ByID(ctx, t.HouseholdID); err != nil {
				if errors.Is(err, domain.ErrNotFound) {
					continue
				}
				return filled, err
			}
			households[t.HouseholdID] = h
		}
		// The rate is looked up outside the database transaction.
		q, err := uc.d.fx().quote(ctx, h, t.Currency, t.Date)
		if errors.Is(err, ErrRateUnavailable) {
			continue
		}
		if err != nil {
			return filled, err
		}
		done := false
		err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
			cur, err := uc.d.Transactions.ByID(ctx, t.HouseholdID, t.ID)
			if found, err := ignoreNotFound(err); err != nil || !found {
				return err
			}
			if !cur.RatePending || cur.Amount != t.Amount || cur.Date != t.Date || cur.Currency != t.Currency {
				return nil // changed meanwhile; the next run sees it again
			}
			done = true
			return fillRate(ctx, uc.d, h, cur, q, fxSystemActor)
		})
		if err != nil {
			return filled, err
		}
		if done {
			filled++
		}
	}
	return filled, nil
}

// formatMinor writes minor units of the currency as "12.34 PLN".
func formatMinor(amount int64, code string) string {
	c, ok := money.LookupCurrency(code)
	if !ok {
		return ""
	}
	return money.Money{Amount: amount, Currency: c}.String() + " " + code
}
