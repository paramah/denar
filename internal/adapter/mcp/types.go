package mcp

import (
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/budget"
	"github.com/paramah/denar/internal/domain/money"
	"github.com/paramah/denar/internal/domain/payee"
	"github.com/paramah/denar/internal/domain/subscription"
	"github.com/paramah/denar/internal/domain/wallet"
	"github.com/paramah/denar/internal/usecase"
)

// The types of api/mcp-tools.md, as the tools answer with them. A field
// that may be absent is null, never left out, so a model always sees the
// same shape.

// moneyOut is Money: a decimal string, the currency, and the amount in
// minor units.
type moneyOut struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
	Minor    int64  `json:"minor"`
}

func minor(amount int64, code string) moneyOut {
	c, ok := money.LookupCurrency(code)
	if !ok {
		c = money.Currency{Code: code, Decimals: 2}
	}
	m := money.Money{Amount: amount, Currency: c}
	return moneyOut{Amount: m.String(), Currency: code, Minor: amount}
}

func fromMoney(m money.Money) moneyOut { return minor(m.Amount, m.Currency.Code) }

// optID is an id that may be absent.
func optID(id domain.ID) *string {
	if id == "" {
		return nil
	}
	s := string(id)
	return &s
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optDate(d domain.Date) *string {
	if d.IsZero() {
		return nil
	}
	s := d.String()
	return &s
}

type actorOut struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type walletOut struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Currency   string `json:"currency"`
	OnBudget   bool   `json:"on_budget"`
	Visibility string `json:"visibility"`
	Archived   bool   `json:"archived"`
}

func walletView(w wallet.Wallet) walletOut {
	return walletOut{ID: string(w.ID), Name: w.Name, Type: string(w.Type), Currency: w.Currency, OnBudget: w.OnBudget,
		Visibility: string(w.Visibility), Archived: w.Archived()}
}

type walletBalanceOut struct {
	walletOut
	Balance     moneyOut `json:"balance"`
	Cleared     moneyOut `json:"cleared"`
	BaseBalance moneyOut `json:"base_balance"`
	BasePending bool     `json:"base_pending"`
}

type splitOut struct {
	EnvelopeID *string  `json:"envelope_id"`
	Amount     moneyOut `json:"amount"`
	Note       string   `json:"note"`
}

type transactionOut struct {
	ID               string     `json:"id"`
	WalletID         string     `json:"wallet_id"`
	Date             string     `json:"date"`
	Amount           moneyOut   `json:"amount"`
	BaseAmount       *moneyOut  `json:"base_amount"`
	Kind             string     `json:"kind"`
	Status           string     `json:"status"`
	Source           string     `json:"source"`
	PayeeID          *string    `json:"payee_id"`
	PayeeName        *string    `json:"payee_name"`
	EnvelopeID       *string    `json:"envelope_id"`
	Tags             []string   `json:"tags"`
	Note             string     `json:"note"`
	TransferWalletID *string    `json:"transfer_wallet_id"`
	Splits           []splitOut `json:"splits"`
	CreatedBy        actorOut   `json:"created_by"`
}

func transactionView(v usecase.TransactionView) transactionOut {
	t := v.Transaction
	out := transactionOut{ID: string(t.ID), WalletID: string(t.WalletID), Date: t.Date.String(), Amount: minor(t.Amount, t.Currency),
		Kind: string(t.Kind), Status: string(t.Status), Source: string(t.Source), PayeeID: optID(t.PayeeID),
		PayeeName: optString(v.PayeeName), EnvelopeID: optID(t.EnvelopeID), Tags: append([]string{}, t.Tags...), Note: t.Note,
		TransferWalletID: optID(t.TransferWalletID), Splits: []splitOut{},
		CreatedBy: actorOut{Kind: string(t.CreatedBy.Kind), ID: string(t.CreatedBy.ID)}}
	if !t.RatePending {
		base := minor(t.BaseAmount, v.BaseCurrency)
		out.BaseAmount = &base
	}
	for _, s := range t.Splits {
		out.Splits = append(out.Splits, splitOut{EnvelopeID: optID(s.EnvelopeID), Amount: minor(s.Amount, t.Currency), Note: s.Note})
	}
	return out
}

type payeeOut struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Patterns          []string `json:"patterns"`
	DefaultEnvelopeID *string  `json:"default_envelope_id"`
}

func payeeView(p payee.Payee) payeeOut {
	return payeeOut{ID: string(p.ID), Name: p.Name, Patterns: append([]string{}, p.Patterns...), DefaultEnvelopeID: optID(p.DefaultEnvelopeID)}
}

type goalOut struct {
	Type       string   `json:"type"`
	Amount     moneyOut `json:"amount"`
	TargetDate *string  `json:"target_date"`
}

func goalView(g budget.Goal, base string) *goalOut {
	if g.IsZero() {
		return nil
	}
	return &goalOut{Type: string(g.Type), Amount: minor(g.Amount, base), TargetDate: optDate(g.TargetDate)}
}

type envelopeOut struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	GroupID   string   `json:"group_id"`
	GroupName string   `json:"group_name"`
	Hidden    bool     `json:"hidden"`
	Goal      *goalOut `json:"goal"`
}

type goalStatusOut struct {
	Needed      moneyOut `json:"needed"`
	Underfunded moneyOut `json:"underfunded"`
	Funded      bool     `json:"funded"`
	Progress    int      `json:"progress"`
}

type budgetEnvelopeOut struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Hidden     bool           `json:"hidden"`
	CarriedIn  moneyOut       `json:"carried_in"`
	Assigned   moneyOut       `json:"assigned"`
	Activity   moneyOut       `json:"activity"`
	Available  moneyOut       `json:"available"`
	Overspent  bool           `json:"overspent"`
	GoalStatus *goalStatusOut `json:"goal_status"`
}

type budgetGroupOut struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Assigned  moneyOut            `json:"assigned"`
	Activity  moneyOut            `json:"activity"`
	Available moneyOut            `json:"available"`
	Envelopes []budgetEnvelopeOut `json:"envelopes"`
}

type budgetMonthOut struct {
	Month               string           `json:"month"`
	Start               string           `json:"start"`
	End                 string           `json:"end"`
	Currency            string           `json:"currency"`
	ReadyToAssign       moneyOut         `json:"ready_to_assign"`
	AssignedInFuture    moneyOut         `json:"assigned_in_future"`
	OverspentLastMonth  moneyOut         `json:"overspent_last_month"`
	Income              moneyOut         `json:"income"`
	Uncategorized       moneyOut         `json:"uncategorized"`
	Assigned            moneyOut         `json:"assigned"`
	Activity            moneyOut         `json:"activity"`
	Available           moneyOut         `json:"available"`
	Underfunded         moneyOut         `json:"underfunded"`
	Incomplete          bool             `json:"incomplete"`
	PendingTransactions int              `json:"pending_transactions"`
	Groups              []budgetGroupOut `json:"groups"`
}

func budgetMonthView(v usecase.BudgetMonthView) budgetMonthOut {
	c := v.Currency
	m := func(amount int64) moneyOut { return minor(amount, c) }
	out := budgetMonthOut{Month: v.Month.String(), Start: v.Start.String(), End: v.End.String(), Currency: c,
		ReadyToAssign: m(v.ReadyToAssign), AssignedInFuture: m(v.AssignedInFuture), OverspentLastMonth: m(v.OverspentLastMonth),
		Income: m(v.Income), Uncategorized: m(v.Uncategorized), Assigned: m(v.Assigned), Activity: m(v.Activity),
		Available: m(v.Available), Underfunded: m(v.Underfunded), Incomplete: v.Incomplete,
		PendingTransactions: v.PendingTransactions, Groups: []budgetGroupOut{}}
	for _, g := range v.Groups {
		gv := budgetGroupOut{ID: string(g.Group.ID), Name: g.Group.Name, Assigned: m(g.Assigned), Activity: m(g.Activity),
			Available: m(g.Available), Envelopes: []budgetEnvelopeOut{}}
		for _, e := range g.Envelopes {
			ev := budgetEnvelopeOut{ID: string(e.Envelope.ID), Name: e.Envelope.Name, Hidden: e.Envelope.Hidden,
				CarriedIn: m(e.CarriedIn), Assigned: m(e.Assigned), Activity: m(e.Activity), Available: m(e.Available),
				Overspent: e.Available < 0}
			if e.Goal != nil {
				ev.GoalStatus = &goalStatusOut{Needed: m(e.Goal.Needed), Underfunded: m(e.Goal.Underfunded), Funded: e.Goal.Funded,
					Progress: e.Goal.Progress}
			}
			gv.Envelopes = append(gv.Envelopes, ev)
		}
		out.Groups = append(out.Groups, gv)
	}
	return out
}

// envelopeLine finds one envelope of a budget month, for the answers of
// the assignment tools.
func envelopeLine(v budgetMonthOut, id domain.ID) *budgetEnvelopeOut {
	for _, g := range v.Groups {
		for i := range g.Envelopes {
			if g.Envelopes[i].ID == string(id) {
				return &g.Envelopes[i]
			}
		}
	}
	return nil
}

type cycleOut struct {
	Unit   string `json:"unit"`
	Every  int    `json:"every"`
	Anchor string `json:"anchor"`
}

type matchRuleOut struct {
	PayeePattern       string `json:"payee_pattern"`
	AmountTolerancePct int    `json:"amount_tolerance_pct"`
	DateWindowDays     int    `json:"date_window_days"`
}

type subscriptionOut struct {
	ID                 string       `json:"id"`
	Name               string       `json:"name"`
	Vendor             string       `json:"vendor"`
	VendorName         string       `json:"vendor_name"`
	Price              moneyOut     `json:"price"`
	PriceEffectiveFrom string       `json:"price_effective_from"`
	Cycle              cycleOut     `json:"cycle"`
	Status             string       `json:"status"`
	TrialEndsAt        *string      `json:"trial_ends_at"`
	EndsAt             *string      `json:"ends_at"`
	WalletID           *string      `json:"wallet_id"`
	EnvelopeID         *string      `json:"envelope_id"`
	ReminderOffsets    []int        `json:"reminder_offsets"`
	MatchRule          matchRuleOut `json:"match_rule"`
	Note               string       `json:"note"`
	ManageURL          *string      `json:"manage_url"`
	NextDue            *string      `json:"next_due"`
	Monthly            moneyOut     `json:"monthly"`
	Yearly             moneyOut     `json:"yearly"`
}

func subscriptionView(v usecase.SubscriptionView) subscriptionOut {
	s := v.Subscription
	c := v.CurrentPrice.Currency
	return subscriptionOut{ID: string(s.ID), Name: s.Name, Vendor: s.Vendor, VendorName: v.VendorName,
		Price: minor(v.CurrentPrice.Amount, c), PriceEffectiveFrom: v.CurrentPrice.EffectiveFrom.String(),
		Cycle:  cycleOut{Unit: string(s.Cycle.Unit), Every: s.Cycle.Every, Anchor: s.Cycle.Anchor.String()},
		Status: string(s.Status), TrialEndsAt: optDate(s.TrialEndsAt), EndsAt: optDate(s.EndsAt), WalletID: optID(s.WalletID),
		EnvelopeID: optID(s.EnvelopeID), ReminderOffsets: append([]int{}, s.ReminderOffsets...),
		MatchRule: matchRuleOut{PayeePattern: s.Match.PayeePattern, AmountTolerancePct: s.Match.AmountTolerancePct,
			DateWindowDays: s.Match.DateWindowDays},
		Note: s.Note, ManageURL: optString(s.ManageURL), NextDue: optDate(v.NextDue),
		Monthly: minor(v.Cost.Monthly, c), Yearly: minor(v.Cost.Yearly, c)}
}

type pricePointOut struct {
	EffectiveFrom string   `json:"effective_from"`
	Price         moneyOut `json:"price"`
}

func pricePointView(p subscription.PricePoint) pricePointOut {
	return pricePointOut{EffectiveFrom: p.EffectiveFrom.String(), Price: minor(p.Amount, p.Currency)}
}

type chargeOut struct {
	ID               string   `json:"id"`
	SubscriptionID   string   `json:"subscription_id"`
	SubscriptionName string   `json:"subscription_name"`
	Vendor           string   `json:"vendor"`
	WalletID         *string  `json:"wallet_id"`
	DueDate          string   `json:"due_date"`
	Amount           moneyOut `json:"amount"`
	Status           string   `json:"status"`
	TransactionID    *string  `json:"transaction_id"`
}

func chargeView(v usecase.ChargeView) chargeOut {
	c := v.Charge
	return chargeOut{ID: string(c.ID), SubscriptionID: string(c.SubscriptionID), SubscriptionName: v.SubscriptionName, Vendor: v.Vendor,
		WalletID: optID(v.WalletID), DueDate: c.DueDate.String(), Amount: minor(c.Amount, c.Currency), Status: string(c.Status),
		TransactionID: optID(c.TransactionID)}
}

type upcomingOut struct {
	Charge     chargeOut `json:"charge"`
	BaseAmount *moneyOut `json:"base_amount"`
}

func upcomingView(u usecase.UpcomingRenewal, base string) upcomingOut {
	out := upcomingOut{Charge: chargeView(u.Charge)}
	if u.BaseKnown {
		b := minor(u.BaseAmount, base)
		out.BaseAmount = &b
	}
	return out
}

type chargeSuggestionOut struct {
	Charge chargeOut `json:"charge"`
	Score  int       `json:"score"`
}

type costGroupOut struct {
	Key     *string  `json:"key"`
	Name    *string  `json:"name"`
	Count   int      `json:"count"`
	Monthly moneyOut `json:"monthly"`
	Yearly  moneyOut `json:"yearly"`
}

type trialEndingOut struct {
	SubscriptionID string   `json:"subscription_id"`
	Name           string   `json:"name"`
	TrialEndsAt    string   `json:"trial_ends_at"`
	Price          moneyOut `json:"price"`
}

type priceChangeOut struct {
	SubscriptionID string   `json:"subscription_id"`
	Name           string   `json:"name"`
	EffectiveFrom  string   `json:"effective_from"`
	OldPrice       moneyOut `json:"old_price"`
	NewPrice       moneyOut `json:"new_price"`
}

type summaryOut struct {
	Currency          string           `json:"currency"`
	Monthly           moneyOut         `json:"monthly"`
	Yearly            moneyOut         `json:"yearly"`
	Incomplete        bool             `json:"incomplete"`
	MissingCurrencies []string         `json:"missing_currencies"`
	Counts            map[string]int   `json:"counts"`
	ByVendor          []costGroupOut   `json:"by_vendor"`
	ByEnvelope        []costGroupOut   `json:"by_envelope"`
	Upcoming          []upcomingOut    `json:"upcoming"`
	TrialsEnding      []trialEndingOut `json:"trials_ending"`
	PriceChanges      []priceChangeOut `json:"price_changes"`
}

func summaryView(s usecase.SubscriptionSummary, envelopeNames map[domain.ID]string) summaryOut {
	b := s.Currency
	m := func(amount int64) moneyOut { return minor(amount, b) }
	out := summaryOut{Currency: b, Monthly: m(s.Total.Monthly), Yearly: m(s.Total.Yearly), Incomplete: s.Incomplete,
		MissingCurrencies: append([]string{}, s.MissingCurrencies...), Counts: map[string]int{},
		ByVendor: []costGroupOut{}, ByEnvelope: []costGroupOut{}, Upcoming: []upcomingOut{}, TrialsEnding: []trialEndingOut{},
		PriceChanges: []priceChangeOut{}}
	for _, st := range subscription.Statuses {
		out.Counts[string(st)] = s.Counts[st]
	}
	for _, g := range s.ByVendor {
		out.ByVendor = append(out.ByVendor, costGroupOut{Key: optString(g.Key), Name: optString(subscription.VendorName(g.Key)),
			Count: g.Count, Monthly: m(g.Cost.Monthly), Yearly: m(g.Cost.Yearly)})
	}
	for _, g := range s.ByEnvelope {
		out.ByEnvelope = append(out.ByEnvelope, costGroupOut{Key: optString(g.Key), Name: optString(envelopeNames[domain.ID(g.Key)]),
			Count: g.Count, Monthly: m(g.Cost.Monthly), Yearly: m(g.Cost.Yearly)})
	}
	for _, u := range s.Upcoming {
		out.Upcoming = append(out.Upcoming, upcomingView(u, b))
	}
	for _, t := range s.TrialsEnding {
		out.TrialsEnding = append(out.TrialsEnding, trialEndingOut{SubscriptionID: string(t.Subscription.ID), Name: t.Subscription.Name,
			TrialEndsAt: t.EndsAt.String(), Price: minor(t.Price.Amount, t.Price.Currency)})
	}
	for _, c := range s.PriceChanges {
		out.PriceChanges = append(out.PriceChanges, priceChangeOut{SubscriptionID: string(c.Subscription.ID), Name: c.Subscription.Name,
			EffectiveFrom: c.Price.EffectiveFrom.String(), OldPrice: minor(c.Previous.Amount, c.Previous.Currency),
			NewPrice: minor(c.Price.Amount, c.Price.Currency)})
	}
	return out
}

type spendingGroupOut struct {
	Key   *string  `json:"key"`
	Name  *string  `json:"name"`
	Count int      `json:"count"`
	Total moneyOut `json:"total"`
}

type spendingOut struct {
	GroupBy             string             `json:"group_by"`
	From                string             `json:"from"`
	To                  string             `json:"to"`
	Currency            string             `json:"currency"`
	Total               moneyOut           `json:"total"`
	Count               int                `json:"count"`
	Groups              []spendingGroupOut `json:"groups"`
	Incomplete          bool               `json:"incomplete"`
	PendingTransactions int                `json:"pending_transactions"`
	Truncated           bool               `json:"truncated"`
}

func spendingView(r usecase.SpendingReport) spendingOut {
	out := spendingOut{GroupBy: r.GroupBy, From: r.From.String(), To: r.To.String(), Currency: r.Currency,
		Total: minor(r.Total, r.Currency), Count: r.Count, Groups: []spendingGroupOut{}, Incomplete: r.Incomplete,
		PendingTransactions: r.PendingTransactions, Truncated: r.Truncated}
	for _, g := range r.Groups {
		out.Groups = append(out.Groups, spendingGroupOut{Key: optID(g.Key), Name: optString(g.Name), Count: g.Count,
			Total: minor(g.Total, r.Currency)})
	}
	return out
}

// parseAmount reads an amount as a model writes it: a decimal string with
// a dot or a comma, without a sign or with one.
func parseAmount(field, s, currency string) (money.Money, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", ".")
	return money.Parse(field, s, currency)
}
