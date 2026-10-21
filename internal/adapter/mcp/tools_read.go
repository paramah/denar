package mcp

import (
	"context"
	"errors"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/imports"
	"github.com/paramah/denar/internal/domain/money"
	"github.com/paramah/denar/internal/usecase"
)

type noArgs struct{}

type whoamiOut struct {
	Bot          whoamiBotOut       `json:"bot"`
	Household    whoamiHouseholdOut `json:"household"`
	Today        string             `json:"today"`
	CurrentMonth string             `json:"current_month"`
	Wallets      []walletOut        `json:"wallets"`
}

type whoamiBotOut struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	WalletIDs []string `json:"wallet_ids"`
	ExpiresAt *string  `json:"expires_at"`
}

type whoamiHouseholdOut struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	BaseCurrency  string `json:"base_currency"`
	Timezone      string `json:"timezone"`
	MonthStartDay int    `json:"month_start_day"`
}

type includeArchivedIn struct {
	IncludeArchived bool `json:"include_archived,omitempty" jsonschema:"also list archived wallets; false by default"`
}

type balancesOut struct {
	Wallets     []walletBalanceOut `json:"wallets"`
	TotalBase   moneyOut           `json:"total_base"`
	BasePending bool               `json:"base_pending"`
}

type searchTransactionsIn struct {
	Wallet   string `json:"wallet,omitempty" jsonschema:"wallet id or name; all your wallets when omitted"`
	Payee    string `json:"payee,omitempty" jsonschema:"payee id or name"`
	Envelope string `json:"envelope,omitempty" jsonschema:"envelope id or name; matches split lines too"`
	From     string `json:"from,omitempty" jsonschema:"first day, YYYY-MM-DD"`
	To       string `json:"to,omitempty" jsonschema:"last day, YYYY-MM-DD"`
	Text     string `json:"text,omitempty" jsonschema:"text in the note or the payee's name, ignoring case"`
	Kind     string `json:"kind,omitempty" jsonschema:"expense, income or transfer"`
	Status   string `json:"status,omitempty" jsonschema:"pending, cleared or reconciled"`
	Tag      string `json:"tag,omitempty" jsonschema:"one tag"`
	Limit    int    `json:"limit,omitempty" jsonschema:"page size, 1 to 200; 50 by default"`
	Cursor   string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous page, with the same filters"`
}

type transactionsOut struct {
	Transactions []transactionOut `json:"transactions"`
	NextCursor   *string          `json:"next_cursor"`
}

type payeesOut struct {
	Payees []payeeOut `json:"payees"`
}

type spendingByIn struct {
	GroupBy string `json:"group_by" jsonschema:"envelope, payee, vendor (the subscription vendor a payment belongs to) or wallet"`
	From    string `json:"from,omitempty" jsonschema:"first day, YYYY-MM-DD; the start of the budget month of 'to' by default"`
	To      string `json:"to,omitempty" jsonschema:"last day, YYYY-MM-DD; today by default; at most 366 days after 'from'"`
	Wallet  string `json:"wallet,omitempty" jsonschema:"only this wallet, by id or name"`
}

type listImportsIn struct {
	Wallet string `json:"wallet,omitempty" jsonschema:"wallet id or name; all your wallets when omitted"`
	Limit  int    `json:"limit,omitempty" jsonschema:"page size, 1 to 200; 50 by default"`
	Cursor string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous page, with the same filters"`
}

type importsOut struct {
	Imports    []importOut `json:"imports"`
	NextCursor *string     `json:"next_cursor"`
}

type importOut struct {
	ID         string  `json:"id"`
	WalletID   string  `json:"wallet_id"`
	Source     string  `json:"source"`
	SourceRef  string  `json:"source_ref"`
	Status     string  `json:"status"`
	Rows       int     `json:"rows"`
	Created    int     `json:"created"`
	Duplicates int     `json:"duplicates"`
	Failed     int     `json:"failed"`
	Matched    int     `json:"matched"`
	Reverted   int     `json:"reverted"`
	CreatedAt  string  `json:"created_at"`
	RevertedAt *string `json:"reverted_at"`
}

func importView(b imports.Batch) importOut {
	out := importOut{ID: string(b.ID), WalletID: string(b.WalletID), Source: string(b.Source), SourceRef: b.SourceRef, Status: string(b.Status),
		Rows: b.Rows, Created: b.Created, Duplicates: b.Duplicates, Failed: b.Failed, Matched: b.Matched, Reverted: b.Reverted,
		CreatedAt: b.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")}
	if !b.RevertedAt.IsZero() {
		at := b.RevertedAt.UTC().Format("2006-01-02T15:04:05Z")
		out.RevertedAt = &at
	}
	return out
}

type vendorsOut struct {
	Vendors []vendorOut `json:"vendors"`
}

type vendorOut struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

func (s *server) addReadTools(m *sdk.Server) {
	addTool(s, m, "whoami",
		"Describe who you are in Denar: your bot, its effective scopes and wallet limit, the household with its base currency and "+
			"time zone, today's date and the current budget month there, and the wallets you can use (names and ids, without balances). "+
			"Call it first.",
		true, func(ctx context.Context, p usecase.Principal, _ domain.ID, _ noArgs) (any, error) {
			c, err := s.uc.whoami.Do(ctx, p)
			if err != nil {
				return nil, err
			}
			b := c.Bot.Bot
			out := whoamiOut{
				Bot: whoamiBotOut{ID: string(b.ID), Name: b.Name, Scopes: []string{}, WalletIDs: []string{}},
				Household: whoamiHouseholdOut{ID: string(c.Household.ID), Name: c.Household.Name, BaseCurrency: c.Household.BaseCurrency,
					Timezone: c.Household.Timezone, MonthStartDay: c.Household.MonthStartDay},
				Today: c.Today.String(), CurrentMonth: c.Month.String(), Wallets: []walletOut{},
			}
			for _, sc := range c.Bot.EffectiveScopes {
				out.Bot.Scopes = append(out.Bot.Scopes, string(sc))
			}
			for _, id := range b.WalletIDs {
				out.Bot.WalletIDs = append(out.Bot.WalletIDs, string(id))
			}
			if !b.ExpiresAt.IsZero() {
				at := b.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
				out.Bot.ExpiresAt = &at
			}
			for _, w := range c.Wallets {
				out.Wallets = append(out.Wallets, walletView(w))
			}
			return out, nil
		})

	addTool(s, m, "list_wallets",
		"List the wallets you can see: id, name, type (cash, checking, card, savings, investment, loan), currency, whether it is on "+
			"the budget, and whether it is archived. Needs wallets:read.",
		true, func(ctx context.Context, p usecase.Principal, hid domain.ID, in includeArchivedIn) (any, error) {
			ws, err := s.uc.listWallets.Do(ctx, p, hid, usecase.ListWalletsInput{IncludeArchived: in.IncludeArchived})
			if err != nil {
				return nil, err
			}
			out := struct {
				Wallets []walletOut `json:"wallets"`
			}{Wallets: []walletOut{}}
			for _, w := range ws {
				out.Wallets = append(out.Wallets, walletView(w.Wallet))
			}
			return out, nil
		})

	addTool(s, m, "get_balances",
		"Show the balance of each wallet you can see in its own currency (all and cleared) and in the household's base currency, "+
			"with the total in the base currency. base_pending means some transactions still wait for an exchange rate. Needs wallets:read.",
		true, func(ctx context.Context, p usecase.Principal, hid domain.ID, in includeArchivedIn) (any, error) {
			ws, err := s.uc.listWallets.Do(ctx, p, hid, usecase.ListWalletsInput{IncludeArchived: in.IncludeArchived})
			if err != nil {
				return nil, err
			}
			c, err := s.uc.whoami.Do(ctx, p)
			if err != nil {
				return nil, err
			}
			base := c.Household.BaseCurrency
			out := balancesOut{Wallets: []walletBalanceOut{}}
			var total int64
			for _, w := range ws {
				out.Wallets = append(out.Wallets, walletBalanceOut{walletOut: walletView(w.Wallet), Balance: fromMoney(w.Balance),
					Cleared: fromMoney(w.Cleared), BaseBalance: fromMoney(w.Base), BasePending: w.BasePending})
				if sum, ok := money.AddInt64(total, w.Base.Amount); ok {
					total = sum
				}
				out.BasePending = out.BasePending || w.BasePending
			}
			out.TotalBase = minor(total, base)
			return out, nil
		})

	addTool(s, m, "search_transactions",
		"Search the transactions of your wallets, newest first. Every filter is optional; amounts are negative for money out. "+
			"Pass next_cursor as cursor for the next page. Needs transactions:read.",
		true, func(ctx context.Context, p usecase.Principal, hid domain.ID, in searchTransactionsIn) (any, error) {
			from, err1 := parseDate("from", in.From)
			to, err2 := parseDate("to", in.To)
			if err := errors.Join(err1, err2); err != nil {
				return nil, err
			}
			input := usecase.SearchTransactionsInput{From: from, To: to, Text: in.Text, Kind: in.Kind, Status: in.Status, Tag: in.Tag,
				Cursor: in.Cursor, Limit: optLimit(in.Limit)}
			scope := bot.ScopeTransactionsRead
			w, err := s.uc.resolve.Wallet(ctx, p, hid, "wallet", in.Wallet, scope)
			if err != nil {
				return nil, err
			}
			pe, err := s.uc.resolve.Payee(ctx, p, hid, "payee", in.Payee, scope)
			if err != nil {
				return nil, err
			}
			e, err := s.uc.resolve.Envelope(ctx, p, hid, "envelope", in.Envelope, scope)
			if err != nil {
				return nil, err
			}
			input.WalletID, input.PayeeID, input.EnvelopeID = string(w.ID), string(pe.ID), string(e.ID)
			page, err := s.uc.searchTransactions.Do(ctx, p, hid, input)
			if err != nil {
				return nil, err
			}
			out := transactionsOut{Transactions: []transactionOut{}, NextCursor: optString(page.NextCursor)}
			for _, t := range page.Items {
				out.Transactions = append(out.Transactions, transactionView(t))
			}
			return out, nil
		})

	addTool(s, m, "list_payees",
		"List the household's payees (shops, people, companies) with their matching patterns and default envelope. Needs transactions:read.",
		true, func(ctx context.Context, p usecase.Principal, hid domain.ID, _ noArgs) (any, error) {
			ps, err := s.uc.listPayees.Do(ctx, p, hid)
			if err != nil {
				return nil, err
			}
			out := payeesOut{Payees: []payeeOut{}}
			for _, pe := range ps {
				out.Payees = append(out.Payees, payeeView(pe))
			}
			return out, nil
		})

	addTool(s, m, "list_imports",
		"List the import batches (CSV files, bank syncs) of your wallets, newest first, with how many rows each had, created, "+
			"skipped as duplicates, failed and matched to subscription charges. Pass next_cursor as cursor for the next page. "+
			"Needs transactions:read.",
		true, func(ctx context.Context, p usecase.Principal, hid domain.ID, in listImportsIn) (any, error) {
			w, err := s.uc.resolve.Wallet(ctx, p, hid, "wallet", in.Wallet, bot.ScopeTransactionsRead)
			if err != nil {
				return nil, err
			}
			page, err := s.uc.listImports.Do(ctx, p, hid, usecase.ListImportsInput{WalletID: string(w.ID), Cursor: in.Cursor,
				Limit: optLimit(in.Limit)})
			if err != nil {
				return nil, err
			}
			out := importsOut{Imports: []importOut{}, NextCursor: optString(page.NextCursor)}
			for _, b := range page.Items {
				out.Imports = append(out.Imports, importView(b))
			}
			return out, nil
		})

	addTool(s, m, "spending_by",
		"Sum what was spent in a period, in the base currency, grouped by envelope, payee, vendor or wallet, over the wallets you can "+
			"see. Refunds lower the sums; transfers and income are not spending. A group with key null collects expenses without one "+
			"(no envelope, no payee, not a subscription). Needs reports:read.",
		true, func(ctx context.Context, p usecase.Principal, hid domain.ID, in spendingByIn) (any, error) {
			from, err1 := parseDate("from", in.From)
			to, err2 := parseDate("to", in.To)
			if err := errors.Join(err1, err2); err != nil {
				return nil, err
			}
			w, err := s.uc.resolve.Wallet(ctx, p, hid, "wallet", in.Wallet, bot.ScopeReportsRead)
			if err != nil {
				return nil, err
			}
			r, err := s.uc.spending.Do(ctx, p, hid, usecase.SpendingReportInput{GroupBy: strings.TrimSpace(in.GroupBy), From: from, To: to,
				WalletID: string(w.ID)})
			if err != nil {
				return nil, err
			}
			return spendingView(r), nil
		})

	addTool(s, m, "list_vendors",
		"List the known subscription vendors (key and name), such as openai, netflix or spotify; use the key in create_subscription.",
		true, func(ctx context.Context, p usecase.Principal, _ domain.ID, _ noArgs) (any, error) {
			out := vendorsOut{Vendors: []vendorOut{}}
			for _, v := range s.uc.listVendors.Do(ctx, p) {
				out.Vendors = append(out.Vendors, vendorOut{Key: v.Key, Name: v.Name})
			}
			return out, nil
		})
}

// parseDate reads an optional date written YYYY-MM-DD.
func parseDate(field, s string) (domain.Date, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return domain.Date{}, nil
	}
	d, err := domain.ParseDate(s)
	if err != nil {
		return domain.Date{}, domain.ValidationError{Field: field, Message: "must be a date written YYYY-MM-DD"}
	}
	return d, nil
}

// optLimit is a page size given as 0 for the default.
func optLimit(n int) domain.Optional[int] {
	if n == 0 {
		return domain.Optional[int]{}
	}
	return domain.Some(n)
}
