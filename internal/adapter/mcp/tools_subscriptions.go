package mcp

import (
	"context"
	"errors"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/subscription"
	"github.com/paramah/denar/internal/usecase"
)

type listSubscriptionsIn struct {
	Status string `json:"status,omitempty" jsonschema:"trial, active, paused or cancelled"`
	Vendor string `json:"vendor,omitempty" jsonschema:"vendor key, such as netflix (see list_vendors)"`
	Wallet string `json:"wallet,omitempty" jsonschema:"only subscriptions paid from this wallet, by id or name"`
}

type subscriptionsOut struct {
	Subscriptions []subscriptionOut `json:"subscriptions"`
}

type subscriptionRefIn struct {
	Subscription string `json:"subscription" jsonschema:"subscription id or name, such as Netflix"`
}

type subscriptionDetailOut struct {
	Subscription subscriptionOut `json:"subscription"`
	Prices       []pricePointOut `json:"prices"`
	OpenCharges  []chargeOut     `json:"open_charges"`
}

type upcomingIn struct {
	Days int `json:"days,omitempty" jsonschema:"how many days ahead, 1 to 365; 30 by default"`
}

type renewalsOut struct {
	Currency string        `json:"currency"`
	Renewals []upcomingOut `json:"renewals"`
}

type listChargesIn struct {
	Subscription string   `json:"subscription,omitempty" jsonschema:"subscription id or name"`
	Status       []string `json:"status,omitempty" jsonschema:"any of expected, matched, missed, skipped, posted"`
	From         string   `json:"from,omitempty" jsonschema:"first due date, YYYY-MM-DD"`
	To           string   `json:"to,omitempty" jsonschema:"last due date, YYYY-MM-DD"`
	Limit        int      `json:"limit,omitempty" jsonschema:"page size, 1 to 200; 50 by default"`
	Cursor       string   `json:"cursor,omitempty" jsonschema:"next_cursor of the previous page, with the same filters"`
}

type chargesOut struct {
	Charges    []chargeOut `json:"charges"`
	NextCursor *string     `json:"next_cursor"`
}

type createSubscriptionIn struct {
	Name            string `json:"name" jsonschema:"name, unique in the household, such as Netflix Premium"`
	Vendor          string `json:"vendor,omitempty" jsonschema:"vendor key from list_vendors; other by default"`
	Amount          string `json:"amount" jsonschema:"price per term, a positive decimal such as \"43.00\""`
	Currency        string `json:"currency,omitempty" jsonschema:"ISO 4217 code of the price, such as USD; the base currency by default"`
	CycleUnit       string `json:"cycle_unit,omitempty" jsonschema:"day, week, month or year; month by default"`
	Every           int    `json:"every,omitempty" jsonschema:"terms are every N units, 1 to 365; 1 by default"`
	Anchor          string `json:"anchor,omitempty" jsonschema:"a day a payment is due, YYYY-MM-DD; the cycle counts from it; today by default"`
	TrialEndsAt     string `json:"trial_ends_at,omitempty" jsonschema:"last day of a free trial, YYYY-MM-DD; makes the status trial"`
	Wallet          string `json:"wallet,omitempty" jsonschema:"wallet it is paid from, by id or name"`
	Envelope        string `json:"envelope,omitempty" jsonschema:"envelope it belongs to, by id or name"`
	ReminderOffsets []int  `json:"reminder_offsets,omitempty" jsonschema:"days before a payment to remind, such as [7, 1]; [3] by default"`
	PayeePattern    string `json:"payee_pattern,omitempty" jsonschema:"text that identifies its payments on a bank statement, such as NETFLIX.COM"`
	Note            string `json:"note,omitempty" jsonschema:"free text"`
	ManageURL       string `json:"manage_url,omitempty" jsonschema:"where to manage or cancel it, http(s) URL"`
}

type updateSubscriptionIn struct {
	Subscription    string  `json:"subscription" jsonschema:"subscription id or name"`
	Name            *string `json:"name,omitempty" jsonschema:"new name"`
	Vendor          *string `json:"vendor,omitempty" jsonschema:"new vendor key"`
	Status          *string `json:"status,omitempty" jsonschema:"trial, active, paused or cancelled; cancelling keeps access until ends_at"`
	CycleUnit       *string `json:"cycle_unit,omitempty" jsonschema:"day, week, month or year"`
	Every           *int    `json:"every,omitempty" jsonschema:"terms are every N units"`
	Anchor          *string `json:"anchor,omitempty" jsonschema:"a day a payment is due, YYYY-MM-DD"`
	TrialEndsAt     *string `json:"trial_ends_at,omitempty" jsonschema:"last day of the trial, YYYY-MM-DD; empty string clears it"`
	EndsAt          *string `json:"ends_at,omitempty" jsonschema:"for a cancelled subscription, the last day of access, YYYY-MM-DD"`
	Wallet          *string `json:"wallet,omitempty" jsonschema:"wallet it is paid from, by id or name; empty string clears it"`
	Envelope        *string `json:"envelope,omitempty" jsonschema:"envelope, by id or name; empty string clears it"`
	ReminderOffsets []int   `json:"reminder_offsets,omitempty" jsonschema:"days before a payment to remind"`
	PayeePattern    *string `json:"payee_pattern,omitempty" jsonschema:"text that identifies its payments on a bank statement"`
	Note            *string `json:"note,omitempty" jsonschema:"free text"`
	ManageURL       *string `json:"manage_url,omitempty" jsonschema:"where to manage or cancel it; empty string clears it"`
}

type changePriceIn struct {
	Subscription  string `json:"subscription" jsonschema:"subscription id or name"`
	Amount        string `json:"amount" jsonschema:"new price per term, a positive decimal such as \"49.00\""`
	Currency      string `json:"currency,omitempty" jsonschema:"ISO 4217 code; the currency of the current price by default"`
	EffectiveFrom string `json:"effective_from,omitempty" jsonschema:"first day of the new price, YYYY-MM-DD; today by default"`
}

type chargeRefIn struct {
	ChargeID string `json:"charge_id" jsonschema:"id of the charge, from list_charges or upcoming_renewals"`
}

type confirmMatchIn struct {
	ChargeID      string `json:"charge_id" jsonschema:"id of the charge"`
	TransactionID string `json:"transaction_id" jsonschema:"id of the expense that paid it"`
}

type postChargeIn struct {
	ChargeID string `json:"charge_id" jsonschema:"id of an expected or missed charge"`
	Wallet   string `json:"wallet,omitempty" jsonschema:"wallet it was paid from, by id or name; the subscription's by default"`
	Amount   string `json:"amount,omitempty" jsonschema:"what was paid, a positive decimal in the wallet's currency; the charge's amount by default (converted at that day's rate)"`
	Date     string `json:"date,omitempty" jsonschema:"day it was paid, YYYY-MM-DD; the due date by default"`
	Payee    string `json:"payee,omitempty" jsonschema:"payee name; the vendor's by default"`
	Note     string `json:"note,omitempty" jsonschema:"note of the transaction"`
}

type postedOut struct {
	Charge      chargeOut      `json:"charge"`
	Transaction transactionOut `json:"transaction"`
}

func (s *server) addSubscriptionTools(m *sdk.Server) {
	addTool(s, m, "list_subscriptions",
		"List the subscriptions you can see by name, with price, cycle, status, next payment and the cost per month and year in the "+
			"price's currency. Needs subscriptions:read.",
		true, func(ctx context.Context, p usecase.Principal, hid domain.ID, in listSubscriptionsIn) (any, error) {
			w, err := s.uc.resolve.Wallet(ctx, p, hid, "wallet", in.Wallet, bot.ScopeSubscriptionsRead)
			if err != nil {
				return nil, err
			}
			subs, err := s.uc.listSubscriptions.Do(ctx, p, hid, usecase.ListSubscriptionsInput{Status: in.Status, Vendor: in.Vendor,
				WalletID: string(w.ID)})
			if err != nil {
				return nil, err
			}
			out := subscriptionsOut{Subscriptions: []subscriptionOut{}}
			for _, v := range subs {
				out.Subscriptions = append(out.Subscriptions, subscriptionView(v))
			}
			return out, nil
		})

	addTool(s, m, "get_subscription",
		"Show one subscription with its price history and its open (expected or missed) charges. Needs subscriptions:read.",
		true, func(ctx context.Context, p usecase.Principal, hid domain.ID, in subscriptionRefIn) (any, error) {
			sub, err := s.uc.resolve.Subscription(ctx, p, hid, "subscription", in.Subscription, bot.ScopeSubscriptionsRead)
			if err != nil {
				return nil, err
			}
			if sub.ID == "" {
				return nil, domain.ValidationError{Field: "subscription", Message: "is required"}
			}
			v, err := s.uc.getSubscription.Do(ctx, p, hid, sub.ID)
			if err != nil {
				return nil, err
			}
			prices, err := s.uc.listSubscriptionPrices.Do(ctx, p, hid, sub.ID)
			if err != nil {
				return nil, err
			}
			charges, err := s.uc.listCharges.Do(ctx, p, hid, usecase.ListChargesInput{SubscriptionID: string(sub.ID),
				Statuses: []string{string(subscription.Expected), string(subscription.Missed)}, Limit: domain.Some(20)})
			if err != nil {
				return nil, err
			}
			out := subscriptionDetailOut{Subscription: subscriptionView(v), Prices: []pricePointOut{}, OpenCharges: []chargeOut{}}
			for _, pp := range prices {
				out.Prices = append(out.Prices, pricePointView(pp))
			}
			for _, c := range charges.Items {
				out.OpenCharges = append(out.OpenCharges, chargeView(c))
			}
			return out, nil
		})

	addTool(s, m, "upcoming_renewals",
		"List the expected subscription payments due in the next days (30 by default), soonest first, each with an estimate in the "+
			"base currency at today's rate (null when no rate is known). Needs subscriptions:read.",
		true, func(ctx context.Context, p usecase.Principal, hid domain.ID, in upcomingIn) (any, error) {
			days := 30
			if in.Days != 0 {
				days = in.Days
			}
			rs, err := s.uc.upcomingRenewals.Do(ctx, p, hid, domain.Some(days))
			if err != nil {
				return nil, err
			}
			c, err := s.uc.whoami.Do(ctx, p)
			if err != nil {
				return nil, err
			}
			out := renewalsOut{Currency: c.Household.BaseCurrency, Renewals: []upcomingOut{}}
			for _, r := range rs {
				out.Renewals = append(out.Renewals, upcomingView(r, out.Currency))
			}
			return out, nil
		})

	addTool(s, m, "subscription_costs",
		"Summarise what the active and trial subscriptions cost per month and per year in the base currency, by vendor and by "+
			"envelope, with payments due in 30 days, trials ending in 14 days and price changes of the last 90 days. incomplete means "+
			"some prices could not be converted (missing_currencies). Needs subscriptions:read.",
		true, func(ctx context.Context, p usecase.Principal, hid domain.ID, _ noArgs) (any, error) {
			sum, err := s.uc.subscriptionSummary.Do(ctx, p, hid)
			if err != nil {
				return nil, err
			}
			names := map[domain.ID]string{}
			// Envelope names only for a bot that may read the budget.
			if es, err := s.uc.listEnvelopes.Do(ctx, p, hid, usecase.ListEnvelopesInput{IncludeHidden: true}); err == nil {
				for _, e := range es {
					names[e.ID] = e.Name
				}
			} else if !errors.Is(err, domain.ErrForbidden) {
				return nil, err
			}
			return summaryView(sum, names), nil
		})

	addTool(s, m, "list_charges",
		"List the charges (payments expected from subscriptions) by due date: expected, matched to a transaction, missed, skipped or "+
			"posted. Needs subscriptions:read.",
		true, func(ctx context.Context, p usecase.Principal, hid domain.ID, in listChargesIn) (any, error) {
			from, err1 := parseDate("from", in.From)
			to, err2 := parseDate("to", in.To)
			if err := errors.Join(err1, err2); err != nil {
				return nil, err
			}
			sub, err := s.uc.resolve.Subscription(ctx, p, hid, "subscription", in.Subscription, bot.ScopeSubscriptionsRead)
			if err != nil {
				return nil, err
			}
			page, err := s.uc.listCharges.Do(ctx, p, hid, usecase.ListChargesInput{SubscriptionID: string(sub.ID), Statuses: in.Status,
				From: from, To: to, Cursor: in.Cursor, Limit: optLimit(in.Limit)})
			if err != nil {
				return nil, err
			}
			out := chargesOut{Charges: []chargeOut{}, NextCursor: optString(page.NextCursor)}
			for _, c := range page.Items {
				out.Charges = append(out.Charges, chargeView(c))
			}
			return out, nil
		})

	addTool(s, m, "create_subscription",
		"Record a subscription: its price per term (in any currency), billing cycle, optional trial, the wallet it is paid from and "+
			"its envelope. Expected charges are created from today on. Needs subscriptions:write.",
		false, func(ctx context.Context, p usecase.Principal, hid domain.ID, in createSubscriptionIn) (any, error) {
			scope := bot.ScopeSubscriptionsWrite
			anchor, err1 := parseDate("anchor", in.Anchor)
			trial, err2 := parseDate("trial_ends_at", in.TrialEndsAt)
			if err := errors.Join(err1, err2); err != nil {
				return nil, err
			}
			w, err := s.uc.resolve.Wallet(ctx, p, hid, "wallet", in.Wallet, scope)
			if err != nil {
				return nil, err
			}
			e, err := s.uc.resolve.Envelope(ctx, p, hid, "envelope", in.Envelope, scope)
			if err != nil {
				return nil, err
			}
			c, err := s.uc.whoami.Do(ctx, p)
			if err != nil {
				return nil, err
			}
			if anchor.IsZero() {
				anchor = c.Today
			}
			currency := strings.ToUpper(strings.TrimSpace(in.Currency))
			if currency == "" {
				currency = c.Household.BaseCurrency
			}
			input := usecase.CreateSubscriptionInput{Name: in.Name, Vendor: cmpOr(in.Vendor, "other"),
				Price:       usecase.MoneyInput{Amount: decimal(in.Amount), Currency: currency},
				Cycle:       usecase.CycleInput{Unit: cmpOr(in.CycleUnit, string(subscription.Month)), Every: in.Every, Anchor: anchor},
				TrialEndsAt: trial, WalletID: string(w.ID), EnvelopeID: string(e.ID), Note: in.Note, ManageURL: in.ManageURL}
			if in.ReminderOffsets != nil {
				input.ReminderOffsets = domain.Some(in.ReminderOffsets)
			}
			if in.PayeePattern != "" {
				input.Match = &usecase.MatchRuleInput{PayeePattern: in.PayeePattern}
			}
			v, err := s.uc.createSubscription.Do(ctx, p, hid, input)
			if err != nil {
				return nil, err
			}
			return subscriptionView(v), nil
		})

	addTool(s, m, "update_subscription",
		"Change a subscription: only the fields given change. Use change_subscription_price for a new price. Pausing or cancelling "+
			"drops its future expected charges. Needs subscriptions:write.",
		false, func(ctx context.Context, p usecase.Principal, hid domain.ID, in updateSubscriptionIn) (any, error) {
			return s.updateSubscription(ctx, p, hid, in)
		})

	addTool(s, m, "change_subscription_price",
		"Record a new price of a subscription from a day on; the price history keeps the old one and future expected charges follow. "+
			"Needs subscriptions:write.",
		false, func(ctx context.Context, p usecase.Principal, hid domain.ID, in changePriceIn) (any, error) {
			from, err := parseDate("effective_from", in.EffectiveFrom)
			if err != nil {
				return nil, err
			}
			sub, err := s.requiredSubscription(ctx, p, hid, in.Subscription, bot.ScopeSubscriptionsWrite)
			if err != nil {
				return nil, err
			}
			currency := strings.ToUpper(strings.TrimSpace(in.Currency))
			if currency == "" && len(sub.Prices) > 0 {
				currency = sub.Prices[len(sub.Prices)-1].Currency
			}
			v, err := s.uc.changeSubscriptionPrice.Do(ctx, p, hid, sub.ID, usecase.ChangeSubscriptionPriceInput{EffectiveFrom: from,
				Price: usecase.MoneyInput{Amount: decimal(in.Amount), Currency: currency}})
			if err != nil {
				return nil, err
			}
			return subscriptionView(v), nil
		})

	addTool(s, m, "confirm_charge_match",
		"Confirm that an expense paid an expected or missed charge; the charge becomes matched. record_expense suggests candidates. "+
			"Needs subscriptions:write and transactions:read.",
		false, func(ctx context.Context, p usecase.Principal, hid domain.ID, in confirmMatchIn) (any, error) {
			v, err := s.uc.confirmChargeMatch.Do(ctx, p, hid, domain.ID(strings.TrimSpace(in.ChargeID)), domain.ID(strings.TrimSpace(in.TransactionID)))
			if err != nil {
				return nil, err
			}
			return chargeView(v), nil
		})

	addTool(s, m, "post_charge",
		"Record the payment of an expected or missed charge as a new expense (source subscription) and link it; for a payment that "+
			"is not in Denar yet. Needs subscriptions:write and transactions:write.",
		false, func(ctx context.Context, p usecase.Principal, hid domain.ID, in postChargeIn) (any, error) {
			return s.postCharge(ctx, p, hid, in)
		})

	addTool(s, m, "skip_charge",
		"Mark an expected or missed charge as skipped: it will not be paid (for example a free month). Needs subscriptions:write.",
		false, func(ctx context.Context, p usecase.Principal, hid domain.ID, in chargeRefIn) (any, error) {
			v, err := s.uc.skipCharge.Do(ctx, p, hid, domain.ID(strings.TrimSpace(in.ChargeID)))
			if err != nil {
				return nil, err
			}
			return chargeView(v), nil
		})
}

// requiredSubscription resolves a subscription that must be named.
func (s *server) requiredSubscription(ctx context.Context, p usecase.Principal, hid domain.ID, ref string, scopes ...bot.Scope) (subscription.Subscription, error) {
	sub, err := s.uc.resolve.Subscription(ctx, p, hid, "subscription", ref, scopes...)
	if err == nil && sub.ID == "" {
		err = domain.ValidationError{Field: "subscription", Message: "is required"}
	}
	return sub, err
}

func (s *server) updateSubscription(ctx context.Context, p usecase.Principal, hid domain.ID, in updateSubscriptionIn) (any, error) {
	scope := bot.ScopeSubscriptionsWrite
	sub, err := s.requiredSubscription(ctx, p, hid, in.Subscription, scope)
	if err != nil {
		return nil, err
	}
	var input usecase.UpdateSubscriptionInput
	var errs []error
	setString := func(dst *domain.Optional[string], v *string) {
		if v != nil {
			*dst = domain.Some(*v)
		}
	}
	setDate := func(dst *domain.Optional[domain.Date], field string, v *string) {
		if v != nil {
			d, err := parseDate(field, *v)
			errs = append(errs, err)
			*dst = domain.Some(d)
		}
	}
	setString(&input.Name, in.Name)
	setString(&input.Vendor, in.Vendor)
	setString(&input.Status, in.Status)
	setString(&input.Note, in.Note)
	setString(&input.ManageURL, in.ManageURL)
	setDate(&input.TrialEndsAt, "trial_ends_at", in.TrialEndsAt)
	setDate(&input.EndsAt, "ends_at", in.EndsAt)
	if in.CycleUnit != nil || in.Every != nil || in.Anchor != nil {
		// The cycle is replaced as a whole; what is not given stays.
		c := usecase.CycleInput{Unit: string(sub.Cycle.Unit), Every: sub.Cycle.Every, Anchor: sub.Cycle.Anchor}
		if in.CycleUnit != nil {
			c.Unit = *in.CycleUnit
		}
		if in.Every != nil {
			c.Every = *in.Every
		}
		if in.Anchor != nil {
			d, err := parseDate("anchor", *in.Anchor)
			errs = append(errs, err)
			c.Anchor = d
		}
		input.Cycle = domain.Some(c)
	}
	if in.ReminderOffsets != nil {
		input.ReminderOffsets = domain.Some(in.ReminderOffsets)
	}
	if in.PayeePattern != nil {
		input.Match = domain.Some(usecase.MatchRuleInput{PayeePattern: *in.PayeePattern,
			AmountTolerancePct: domain.Some(sub.Match.AmountTolerancePct), DateWindowDays: domain.Some(sub.Match.DateWindowDays)})
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	if in.Wallet != nil {
		id := ""
		if strings.TrimSpace(*in.Wallet) != "" {
			w, err := s.uc.resolve.Wallet(ctx, p, hid, "wallet", *in.Wallet, scope)
			if err != nil {
				return nil, err
			}
			id = string(w.ID)
		}
		input.WalletID = domain.Some(id)
	}
	if in.Envelope != nil {
		id := ""
		if strings.TrimSpace(*in.Envelope) != "" {
			e, err := s.uc.resolve.Envelope(ctx, p, hid, "envelope", *in.Envelope, scope)
			if err != nil {
				return nil, err
			}
			id = string(e.ID)
		}
		input.EnvelopeID = domain.Some(id)
	}
	v, err := s.uc.updateSubscription.Do(ctx, p, hid, sub.ID, input)
	if err != nil {
		return nil, err
	}
	return subscriptionView(v), nil
}

func (s *server) postCharge(ctx context.Context, p usecase.Principal, hid domain.ID, in postChargeIn) (any, error) {
	date, err := parseDate("date", in.Date)
	if err != nil {
		return nil, err
	}
	scopes := []bot.Scope{bot.ScopeSubscriptionsWrite, bot.ScopeTransactionsWrite}
	w, err := s.uc.resolve.Wallet(ctx, p, hid, "wallet", in.Wallet, scopes...)
	if err != nil {
		return nil, err
	}
	input := usecase.PostChargeInput{WalletID: string(w.ID), Date: date, PayeeName: in.Payee, Note: in.Note}
	if strings.TrimSpace(in.Amount) != "" {
		currency := w.Currency
		if w.ID == "" {
			// The subscription's wallet pays.
			c, err := s.uc.getCharge.Do(ctx, p, hid, domain.ID(strings.TrimSpace(in.ChargeID)))
			if err != nil {
				return nil, err
			}
			if c.WalletID == "" {
				return nil, domain.ValidationError{Field: "wallet", Message: "is required: the subscription names no wallet"}
			}
			sw, err := s.uc.resolve.Wallet(ctx, p, hid, "wallet", string(c.WalletID), scopes...)
			if err != nil {
				return nil, err
			}
			currency = sw.Currency
		}
		input.Amount = &usecase.MoneyInput{Amount: decimal(in.Amount), Currency: currency}
	}
	posted, err := s.uc.postCharge.Do(ctx, p, hid, domain.ID(strings.TrimSpace(in.ChargeID)), input)
	if err != nil {
		return nil, err
	}
	return postedOut{Charge: chargeView(posted.Charge), Transaction: transactionView(posted.Transaction)}, nil
}

// decimal writes an amount the way the use cases read it: trimmed, with a
// dot.
func decimal(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), ",", ".") }

func cmpOr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return strings.TrimSpace(s)
}
