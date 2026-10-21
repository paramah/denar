package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/paramah/denar/internal/adapter/sqldb"
	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/budget"
	"github.com/paramah/denar/internal/domain/subscription"
	"github.com/paramah/denar/internal/domain/user"
	"github.com/paramah/denar/internal/platform/config"
	"github.com/paramah/denar/internal/usecase"
)

// The demo account and household (stage 009). The password is fixed and
// easy to type: the demo is for a local look around only.
const (
	demoEmail         = "demo@denar.local"
	demoName          = "Demo"
	demoPassword      = "denar-demo-2026"
	demoHousehold     = "Dom (demo)"
	demoTimezone      = "Europe/Warsaw"
	demoHistoryDays   = 92
	demoBotName       = "Asystent AI"
	demoLeaveOpenDays = 2 // charges due this recently are left to be matched
)

const demoUsage = `usage: denar demo [--reset] [--password <password>]

Fills the database (DENAR_DB, migrated if needed) with a demo household,
"Dom (demo)", of the account demo@denar.local: wallets, three months of
transactions, envelopes, subscriptions, notifications and a bot. It is
made through the use cases, so the audit log and the rules apply. It
refuses when the demo household exists, unless --reset, which deletes it
first. For local use only: the password is not secret.`

// demoScopes are the bot's: reading everything, writing transactions.
var demoScopes = []string{"wallets:read", "transactions:read", "transactions:write", "budget:read", "subscriptions:read",
	"reports:read", "notifications:read"}

// demoClock is the clock of the seed: it walks from the first day of the
// history to today.
type demoClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *demoClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *demoClock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// demoResult is what the seed made, for the summary and the tests.
type demoResult struct {
	HouseholdID domain.ID
	Password    string
	BotToken    string
	Unread      int
}

func demoCommand(ctx context.Context, cfg config.Config, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, demoUsage) }
	reset := fs.Bool("reset", false, "delete the demo household first")
	password := fs.String("password", "", "password of demo@denar.local (default "+demoPassword+", or DENAR_DEMO_PASSWORD)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	pw := *password
	if pw == "" {
		pw = os.Getenv("DENAR_DEMO_PASSWORD")
	}
	if pw == "" {
		pw = demoPassword
	}

	db, err := sqldb.Open(cfg.Database)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := sqldb.Migrate(ctx, db, db.Migrations()); err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	r, err := seedDemo(ctx, db, log, time.Now(), pw, *reset)
	if err != nil {
		return describe(err)
	}
	printDemo(stdout, cfg.BaseURL, r)
	return nil
}

func printDemo(w io.Writer, baseURL string, r demoResult) {
	fmt.Fprintf(w, `Dane demo gotowe (gospodarstwo %q, %d nieprzeczytanych powiadomień).

  URL:      %s
  Login:    %s
  Hasło:    %s
  Token bota %q (pokazywany tylko raz):
            %s

Podłącz asystenta (Claude Code):
  claude mcp add --transport http denar %s/mcp --header "Authorization: Bearer %s"

Tylko do lokalnego oglądania: hasło demo nie jest tajne.
`, demoHousehold, r.Unread, baseURL, demoEmail, r.Password, demoBotName, r.BotToken, baseURL, r.BotToken)
}

// seedDemo makes the demo household through the use cases, walking a
// clock through the last three months so that charges, reminders, missed
// payments and notifications come out of the jobs as they would have.
func seedDemo(ctx context.Context, db *sqldb.DB, log *slog.Logger, now time.Time, password string, reset bool) (demoResult, error) {
	loc, err := time.LoadLocation(demoTimezone)
	if err != nil {
		return demoResult{}, err
	}
	now = now.In(loc)
	today := domain.DateOf(now, loc)
	start := today.AddDays(-demoHistoryDays)
	clock := &demoClock{}
	clock.set(noon(start, loc))
	d := useCaseDeps(db, log, clock, nil)
	d.ChargeHorizonDays = 90
	s := &seeder{d: d, clock: clock, loc: loc, today: today, start: start, now: now}

	if err := s.account(ctx, password, reset); err != nil {
		return demoResult{}, err
	}
	for day := start; !day.After(today); day = day.AddDays(1) {
		if day == today {
			clock.set(now)
		} else {
			clock.set(noon(day, loc))
		}
		if err := s.day(ctx, day); err != nil {
			return demoResult{}, fmt.Errorf("demo day %s: %w", day, err)
		}
	}
	return s.finish(ctx, password)
}

func noon(d domain.Date, loc *time.Location) time.Time {
	t := d.Time()
	return time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, loc)
}

type demoSub struct {
	key        string
	in         usecase.CreateSubscriptionInput
	createOn   domain.Date
	lastDue    int // days from today of the newest charge, used to set the anchor
	id         domain.ID
	walletID   domain.ID
	envelopeID domain.ID
	payee      string
}

type seeder struct {
	d     usecase.Deps
	clock *demoClock
	loc   *time.Location
	today domain.Date
	start domain.Date
	now   time.Time

	p   usecase.Principal
	hid domain.ID
	// wallets and envelopes by name
	wallets   map[string]domain.ID
	envelopes map[string]domain.ID
	subs      []*demoSub
	rnd       uint32
}

// account creates the demo account, or finds it, and deals with an
// existing demo household.
func (s *seeder) account(ctx context.Context, password string, reset bool) error {
	created, err := usecase.NewBootstrapAdmin(s.d.Users, s.d.Hasher, s.d.IDs, s.d.Clock, s.d.Tx).Do(ctx,
		usecase.BootstrapAdminInput{Email: demoEmail, Password: password, Name: demoName})
	if err != nil {
		return fmt.Errorf("demo account: %w", err)
	}
	u, err := s.d.Users.ByEmail(ctx, user.NormalizeEmail(demoEmail))
	if err != nil {
		return err
	}
	if !created {
		if !s.d.Hasher.Verify(password, u.PasswordHash) {
			return errors.New("the account " + demoEmail + " exists with another password; use that one with --password, or start from an empty database")
		}
	}
	s.p = usecase.Principal{User: u}
	hs, err := usecase.NewListHouseholds(s.d).Do(ctx, s.p)
	if err != nil {
		return err
	}
	for _, h := range hs {
		if h.Household.Name != demoHousehold {
			continue
		}
		if !reset {
			return errors.New("the demo household exists already; run with --reset to make it again")
		}
		if err := usecase.NewDeleteHousehold(s.d).Do(ctx, s.p, h.Household.ID); err != nil {
			return fmt.Errorf("deleting the old demo household: %w", err)
		}
	}
	return nil
}

// next is a small deterministic random number: the demo is the same every
// time.
func (s *seeder) next(n int) int64 {
	s.rnd = s.rnd*1664525 + 1013904223
	return int64((s.rnd >> 8) % uint32(n))
}

func amount(minor int64) string {
	sign := ""
	if minor < 0 {
		sign, minor = "-", -minor
	}
	return fmt.Sprintf("%s%d.%02d", sign, minor/100, minor%100)
}

func pln(minor int64) usecase.MoneyInput {
	return usecase.MoneyInput{Amount: amount(minor), Currency: "PLN"}
}

// rates of the demo, in 1/10000 PLN: a gentle wave around 3.98 and 4.30.
func (s *seeder) rate(base int64, day domain.Date) string {
	i := int64(day.Time().Sub(s.start.Time()).Hours() / 24)
	wave := (i*37)%160 - 80
	if (i/40)%2 == 1 {
		wave = -wave
	}
	r := base + wave
	return fmt.Sprintf("%d.%04d", r/10000, r%10000)
}

func (s *seeder) setRates(ctx context.Context, day domain.Date) error {
	fx := usecase.NewSetFxOverride(s.d)
	for cur, base := range map[string]int64{"USD": 39800, "EUR": 43000} {
		if _, err := fx.Do(ctx, s.p, s.hid, usecase.SetFxOverrideInput{Currency: cur, Date: day, Rate: s.rate(base, day)}); err != nil {
			return fmt.Errorf("rate %s %s: %w", cur, day, err)
		}
	}
	return nil
}

// setup is the first day: household, wallets, rates, envelopes, payees
// and subscriptions.
func (s *seeder) setup(ctx context.Context) error {
	h, err := usecase.NewCreateHousehold(s.d).Do(ctx, s.p, usecase.CreateHouseholdInput{Name: demoHousehold, BaseCurrency: "PLN",
		Timezone: demoTimezone, MonthStartDay: 1})
	if err != nil {
		return err
	}
	s.hid = h.Household.ID
	s.wallets, s.envelopes = map[string]domain.ID{}, map[string]domain.ID{}

	// Rates a week back, so that every day of the history has one.
	for day := s.start.AddDays(-7); day.Before(s.start); day = day.AddDays(1) {
		if err := s.setRates(ctx, day); err != nil {
			return err
		}
	}

	wallets := []usecase.CreateWalletInput{
		{Name: "Konto osobiste", Type: "checking", Currency: "PLN", OnBudget: true, OpeningBalance: &usecase.MoneyInput{Amount: "8000.00", Currency: "PLN"}},
		{Name: "Karta kredytowa", Type: "card", Currency: "PLN", OnBudget: true},
		{Name: "Gotówka", Type: "cash", Currency: "PLN", OnBudget: true, OpeningBalance: &usecase.MoneyInput{Amount: "400.00", Currency: "PLN"}},
		{Name: "Konto USD", Type: "checking", Currency: "USD", OnBudget: true, OpeningBalance: &usecase.MoneyInput{Amount: "300.00", Currency: "USD"}},
		{Name: "Gotówka EUR", Type: "cash", Currency: "EUR", OnBudget: true},
		{Name: "Oszczędności", Type: "savings", Currency: "PLN", OnBudget: false, Visibility: "private",
			OpeningBalance: &usecase.MoneyInput{Amount: "15000.00", Currency: "PLN"}},
	}
	for _, in := range wallets {
		in.OpenedOn = s.start
		w, err := usecase.NewCreateWallet(s.d).Do(ctx, s.p, s.hid, in)
		if err != nil {
			return fmt.Errorf("wallet %s: %w", in.Name, err)
		}
		s.wallets[in.Name] = w.Wallet.ID
	}

	groups := []struct {
		name      string
		envelopes []string
	}{
		{"Stałe", []string{"Czynsz", "Media", "Internet i telefon"}},
		{"Codzienne", []string{"Jedzenie", "Transport", "Zdrowie"}},
		{"Subskrypcje", []string{"Streaming", "AI i narzędzia", "Chmura"}},
		{"Cele", []string{"Wakacje", "Poduszka finansowa"}},
		{"Przyjemności", []string{"Restauracje", "Hobby"}},
	}
	for gi, g := range groups {
		group, err := usecase.NewCreateEnvelopeGroup(s.d).Do(ctx, s.p, s.hid, usecase.CreateEnvelopeGroupInput{Name: g.name, Position: gi})
		if err != nil {
			return fmt.Errorf("group %s: %w", g.name, err)
		}
		for ei, name := range g.envelopes {
			in := usecase.CreateEnvelopeInput{GroupID: string(group.ID), Name: name, Position: ei}
			switch name {
			case "Wakacje":
				in.Goal = &usecase.GoalInput{Type: string(budget.GoalTargetByDate), Amount: pln(600000), TargetDate: s.today.AddDays(250)}
			case "Poduszka finansowa":
				in.Goal = &usecase.GoalInput{Type: string(budget.GoalMonthly), Amount: pln(100000)}
			}
			e, err := usecase.NewCreateEnvelope(s.d).Do(ctx, s.p, s.hid, in)
			if err != nil {
				return fmt.Errorf("envelope %s: %w", name, err)
			}
			s.envelopes[name] = e.ID
		}
	}

	payees := []struct {
		name, envelope string
		patterns       []string
	}{
		{"Biedronka", "Jedzenie", []string{"BIEDRONKA", "JMP S.A."}},
		{"Lidl", "Jedzenie", []string{"LIDL"}},
		{"Orlen", "Transport", []string{"ORLEN", "PKN ORLEN"}},
		{"PGNiG", "Media", []string{"PGNIG", "PGE OBROT"}},
		{"Orange", "Internet i telefon", []string{"ORANGE POLSKA"}},
		{"Netflix", "Streaming", []string{"NETFLIX"}},
		{"Spotify", "Streaming", []string{"SPOTIFY"}},
		{"OpenAI", "AI i narzędzia", []string{"OPENAI", "CHATGPT"}},
		{"Anthropic", "AI i narzędzia", []string{"ANTHROPIC", "CLAUDE.AI"}},
		{"Apple", "Chmura", []string{"APPLE.COM/BILL", "ICLOUD"}},
		{"Google", "Streaming", []string{"GOOGLE", "YOUTUBE"}},
		{"Adobe", "Hobby", []string{"ADOBE"}},
		{"Wynajmujący", "Czynsz", nil},
		{"Apteka", "Zdrowie", nil},
		{"Empik", "Hobby", []string{"EMPIK"}},
		{"Pizzeria Roma", "Restauracje", nil},
		{"Bistro Pod Lipami", "Restauracje", nil},
		{"Ramen Shop", "Restauracje", nil},
		{"Sushi Bar Kioto", "Restauracje", nil},
	}
	for _, pe := range payees {
		if _, err := usecase.NewCreatePayee(s.d).Do(ctx, s.p, s.hid, usecase.CreatePayeeInput{Name: pe.name, Patterns: pe.patterns,
			DefaultEnvelopeID: string(s.envelopes[pe.envelope])}); err != nil {
			return fmt.Errorf("payee %s: %w", pe.name, err)
		}
	}

	card, usd := s.wallets["Karta kredytowa"], s.wallets["Konto USD"]
	monthly := func(anchorFromToday int) usecase.CycleInput {
		return usecase.CycleInput{Unit: "month", Every: 1, Anchor: s.anchor(anchorFromToday)}
	}
	s.subs = []*demoSub{
		{key: "netflix", lastDue: -1, walletID: card, envelopeID: s.envelopes["Streaming"], payee: "Netflix",
			in: usecase.CreateSubscriptionInput{Name: "Netflix Standard", Vendor: "netflix", Price: pln(4900),
				Match: &usecase.MatchRuleInput{PayeePattern: "NETFLIX"}, ManageURL: "https://www.netflix.com/account"}},
		{key: "spotify", lastDue: -9, walletID: card, envelopeID: s.envelopes["Streaming"], payee: "Spotify",
			in: usecase.CreateSubscriptionInput{Name: "Spotify Premium", Vendor: "spotify", Price: pln(2399),
				Match: &usecase.MatchRuleInput{PayeePattern: "SPOTIFY"}}},
		{key: "chatgpt", lastDue: -14, walletID: usd, envelopeID: s.envelopes["AI i narzędzia"], payee: "OpenAI",
			in: usecase.CreateSubscriptionInput{Name: "ChatGPT Plus", Vendor: "openai", Price: usecase.MoneyInput{Amount: "20.00", Currency: "USD"},
				Match: &usecase.MatchRuleInput{PayeePattern: "OPENAI"}}},
		{key: "claude", lastDue: -27, walletID: usd, envelopeID: s.envelopes["AI i narzędzia"], payee: "Anthropic",
			in: usecase.CreateSubscriptionInput{Name: "Claude Pro", Vendor: "anthropic", Price: usecase.MoneyInput{Amount: "20.00", Currency: "USD"},
				Match: &usecase.MatchRuleInput{PayeePattern: "ANTHROPIC"}, ReminderOffsets: domain.Some([]int{3, 1})}},
		{key: "icloud", lastDue: -13, walletID: card, envelopeID: s.envelopes["Chmura"], payee: "Apple",
			in: usecase.CreateSubscriptionInput{Name: "iCloud+ 200 GB", Vendor: "apple", Price: pln(1199),
				Match: &usecase.MatchRuleInput{PayeePattern: "APPLE"}}},
		{key: "adobe", lastDue: -50, walletID: card, envelopeID: s.envelopes["Hobby"], payee: "Adobe",
			in: usecase.CreateSubscriptionInput{Name: "Adobe Photography", Vendor: "adobe", Price: pln(5149),
				Match: &usecase.MatchRuleInput{PayeePattern: "ADOBE"}}},
	}
	for _, sub := range s.subs {
		sub.in.Cycle = monthly(sub.lastDue)
		sub.createOn = s.start
	}
	// A trial that started three and a half weeks ago and ends in four
	// days; the first payment is due when it ends.
	s.subs = append(s.subs, &demoSub{key: "youtube", createOn: s.today.AddDays(-26), walletID: card, envelopeID: s.envelopes["Streaming"],
		payee: "Google", in: usecase.CreateSubscriptionInput{Name: "YouTube Premium", Vendor: "youtube", Price: pln(2599),
			Cycle: usecase.CycleInput{Unit: "month", Every: 1, Anchor: s.today.AddDays(4)}, TrialEndsAt: s.today.AddDays(4),
			Match: &usecase.MatchRuleInput{PayeePattern: "YOUTUBE"}, ReminderOffsets: domain.Some([]int{7, 3, 1})}})
	return nil
}

// anchor is the first due date of a monthly subscription whose newest
// charge is offset days from today: that date, as many months back as
// fit after the first day of the history.
func (s *seeder) anchor(offset int) domain.Date {
	last := s.today.AddDays(offset).Time()
	for n := 6; n >= 0; n-- {
		t := last.AddDate(0, -n, 0)
		d := domain.NewDate(t.Year(), t.Month(), t.Day())
		if d.After(s.start.AddDays(2)) {
			return d
		}
	}
	return s.today.AddDays(offset)
}

// assign sets the month's assignments.
func (s *seeder) assign(ctx context.Context, day domain.Date) error {
	month := budget.MonthOf(day, 1).String()
	current := budget.MonthOf(s.today, 1).String() == month
	plan := map[string]int64{
		"Czynsz": 240000, "Media": 20000, "Internet i telefon": 9000,
		"Jedzenie": 150000, "Transport": 80000, "Zdrowie": 15000,
		"Streaming": 12000, "AI i narzędzia": 18000, "Chmura": 2000,
		"Wakacje": 80000, "Poduszka finansowa": 100000,
		"Restauracje": 30000, "Hobby": 30000,
	}
	if current {
		// Too little for the month's restaurants: the envelope goes red.
		plan["Restauracje"] = 10000
	}
	for name, minor := range plan {
		if _, err := usecase.NewSetAssignment(s.d).Do(ctx, s.p, s.hid, s.envelopes[name],
			usecase.SetAssignmentInput{Month: month, Amount: pln(minor)}); err != nil {
			return fmt.Errorf("assignment %s %s: %w", name, month, err)
		}
	}
	return nil
}

func (s *seeder) spend(ctx context.Context, day domain.Date, wallet, payee string, minor int64, currency, note string) error {
	in := usecase.RecordTransactionInput{WalletID: string(s.wallets[wallet]), Date: day, PayeeName: payee,
		Amount: usecase.MoneyInput{Amount: amount(-minor), Currency: currency}, Note: note}
	if _, err := usecase.NewRecordTransaction(s.d).Do(ctx, s.p, s.hid, in); err != nil {
		return fmt.Errorf("%s %s: %w", payee, amount(minor), err)
	}
	return nil
}

func (s *seeder) transfer(ctx context.Context, day domain.Date, from, to string, out, in usecase.MoneyInput, envelope, note string) error {
	t := usecase.CreateTransferInput{FromWalletID: string(s.wallets[from]), ToWalletID: string(s.wallets[to]), Date: day, Amount: out,
		EnvelopeID: string(s.envelopes[envelope]), Note: note}
	if in != out {
		t.ToAmount = &in
	}
	if _, err := usecase.NewCreateTransfer(s.d).Do(ctx, s.p, s.hid, t); err != nil {
		return fmt.Errorf("transfer %s → %s: %w", from, to, err)
	}
	return nil
}

// day plays one day of the history.
func (s *seeder) day(ctx context.Context, day domain.Date) error {
	if day == s.start {
		if err := s.setup(ctx); err != nil {
			return err
		}
	}
	if err := s.setRates(ctx, day); err != nil {
		return err
	}
	if day == s.today {
		// Two weeks of rates ahead: the demo works offline for a while.
		for ahead := day.AddDays(1); !ahead.After(day.AddDays(14)); ahead = ahead.AddDays(1) {
			if err := s.setRates(ctx, ahead); err != nil {
				return err
			}
		}
	}
	if day == s.start || day.Time().Day() == 1 {
		if err := s.assign(ctx, day); err != nil {
			return err
		}
	}
	if err := s.everyday(ctx, day); err != nil {
		return err
	}
	if err := s.subscriptions(ctx, day); err != nil {
		return err
	}
	if _, err := usecase.NewRunSubscriptionJobs(s.d).Do(ctx); err != nil {
		return err
	}
	if day == s.today {
		if _, err := usecase.NewCheckBudgets(s.d).Do(ctx); err != nil {
			return err
		}
	}
	if _, err := usecase.NewRunNotificationJobs(s.d).Do(ctx); err != nil {
		return err
	}
	if day == s.today.AddDays(-20) {
		// What happened until three weeks ago has been read.
		if _, err := usecase.NewMarkAllNotificationsRead(s.d).Do(ctx, s.p, s.hid); err != nil {
			return err
		}
	}
	return nil
}

// everyday is the money of a day that is not a subscription.
func (s *seeder) everyday(ctx context.Context, day domain.Date) error {
	i := int(day.Time().Sub(s.start.Time()).Hours() / 24)
	dom := day.Time().Day()
	type op func() error
	var ops []op
	if dom == 10 {
		ops = append(ops, func() error {
			_, err := usecase.NewRecordTransaction(s.d).Do(ctx, s.p, s.hid, usecase.RecordTransactionInput{
				WalletID: string(s.wallets["Konto osobiste"]), Date: day, Kind: "income", PayeeName: "Pracodawca sp. z o.o.",
				Amount: pln(950000), Note: "Wynagrodzenie"})
			return err
		})
	}
	if dom == 2 {
		ops = append(ops, func() error { return s.spend(ctx, day, "Konto osobiste", "Wynajmujący", 240000, "PLN", "Czynsz") })
	}
	if dom == 15 {
		ops = append(ops,
			func() error { return s.spend(ctx, day, "Konto osobiste", "PGNiG", 16000+s.next(3000), "PLN", "") },
			func() error {
				return s.spend(ctx, day, "Konto osobiste", "Orange", 8900, "PLN", "Światłowód + telefon")
			})
	}
	if dom == 11 {
		ops = append(ops, func() error {
			return s.transfer(ctx, day, "Konto osobiste", "Oszczędności", pln(100000), pln(100000), "Poduszka finansowa", "Co miesiąc na poduszkę")
		})
	}
	if dom == 25 {
		ops = append(ops, func() error {
			w, err := usecase.NewGetWallet(s.d).Do(ctx, s.p, s.hid, s.wallets["Karta kredytowa"])
			if err != nil || w.Balance.Amount >= 0 {
				return err
			}
			return s.transfer(ctx, day, "Konto osobiste", "Karta kredytowa", pln(-w.Balance.Amount), pln(-w.Balance.Amount), "", "Spłata karty")
		})
	}
	if i%3 == 0 {
		shop, wallet := "Biedronka", "Konto osobiste"
		if (i/3)%2 == 1 {
			shop, wallet = "Lidl", "Karta kredytowa"
		}
		ops = append(ops, func() error { return s.spend(ctx, day, wallet, shop, 9000+s.next(9000), "PLN", "") })
	}
	if i%9 == 4 {
		ops = append(ops, func() error {
			return s.spend(ctx, day, "Karta kredytowa", "Orlen", 22000+s.next(6000), "PLN", "Paliwo")
		})
	}
	if i%14 == 6 {
		place := []string{"Pizzeria Roma", "Bistro Pod Lipami", "Ramen Shop"}[s.next(3)]
		ops = append(ops, func() error { return s.spend(ctx, day, "Karta kredytowa", place, 9000+s.next(5000), "PLN", "") })
	}
	if dom == 20 {
		ops = append(ops, func() error { return s.spend(ctx, day, "Gotówka", "Apteka", 4500+s.next(3000), "PLN", "") })
	}
	if dom == 5 {
		ops = append(ops, func() error {
			return s.spend(ctx, day, "Karta kredytowa", "Empik", 6000+s.next(6000), "PLN", "Książki")
		})
	}
	// The month's extra dinner out, two days ago or today early in a month:
	// Restauracje goes over in the current month.
	overspendOn := s.today.AddDays(-2)
	if budget.MonthOf(overspendOn, 1) != budget.MonthOf(s.today, 1) {
		overspendOn = s.today
	}
	if day == overspendOn {
		ops = append(ops, func() error {
			available, err := s.available(ctx, day, "Restauracje")
			if err != nil {
				return err
			}
			return s.spend(ctx, day, "Karta kredytowa", "Sushi Bar Kioto", max(available, 0)+8537, "PLN", "Urodziny")
		})
	}
	switch day {
	case s.start.AddDays(5):
		// Dollars for the AI subscriptions.
		ops = append(ops, func() error {
			return s.transfer(ctx, day, "Konto osobiste", "Konto USD", pln(200000), usecase.MoneyInput{Amount: "500.00", Currency: "USD"}, "",
				"Wymiana PLN → USD")
		})
	case s.today.AddDays(-36):
		ops = append(ops, func() error {
			return s.transfer(ctx, day, "Konto osobiste", "Gotówka EUR", pln(86000), usecase.MoneyInput{Amount: "200.00", Currency: "EUR"}, "",
				"Euro na wyjazd do Berlina")
		})
	case s.today.AddDays(-33):
		ops = append(ops, func() error {
			_, err := usecase.NewRecordTransaction(s.d).Do(ctx, s.p, s.hid, usecase.RecordTransactionInput{
				WalletID: string(s.wallets["Gotówka EUR"]), Date: day, PayeeName: "Restaurant Berlin Mitte",
				Amount: usecase.MoneyInput{Amount: "-64.50", Currency: "EUR"}, EnvelopeID: string(s.envelopes["Wakacje"]), Note: "Kolacja w Berlinie"})
			return err
		})
	}
	for _, o := range ops {
		if err := o(); err != nil {
			return err
		}
	}
	return nil
}

// available is what an envelope has left in the month of day.
func (s *seeder) available(ctx context.Context, day domain.Date, envelope string) (int64, error) {
	m, err := usecase.NewGetBudgetMonth(s.d).Do(ctx, s.p, s.hid, budget.MonthOf(day, 1).String())
	if err != nil {
		return 0, err
	}
	for _, g := range m.Groups {
		for _, e := range g.Envelopes {
			if e.Envelope.ID == s.envelopes[envelope] {
				return e.Available, nil
			}
		}
	}
	return 0, errors.New("no envelope " + envelope)
}

// subscriptions creates the subscriptions on their day, changes Spotify's
// price, cancels Adobe and pays the charges due: matched to the payment,
// except the newest ones (left for the person to match) and one iCloud
// payment that never came (missed).
func (s *seeder) subscriptions(ctx context.Context, day domain.Date) error {
	for _, sub := range s.subs {
		if day == sub.createOn {
			sub.in.WalletID, sub.in.EnvelopeID = string(sub.walletID), string(sub.envelopeID)
			v, err := usecase.NewCreateSubscription(s.d).Do(ctx, s.p, s.hid, sub.in)
			if err != nil {
				return fmt.Errorf("subscription %s: %w", sub.key, err)
			}
			sub.id = v.Subscription.ID
		}
		if sub.id == "" {
			continue
		}
		if sub.key == "adobe" && day == s.today.AddDays(-45) {
			if _, err := usecase.NewUpdateSubscription(s.d).Do(ctx, s.p, s.hid, sub.id,
				usecase.UpdateSubscriptionInput{Status: domain.Some(string(subscription.Cancelled)), Note: domain.Some("Zrezygnowałem — za drogo")}); err != nil {
				return fmt.Errorf("cancelling adobe: %w", err)
			}
		}
		page, err := usecase.NewListCharges(s.d).Do(ctx, s.p, s.hid, usecase.ListChargesInput{SubscriptionID: string(sub.id),
			Statuses: []string{string(subscription.Expected)}, From: day, To: day})
		if err != nil {
			return err
		}
		for _, c := range page.Items {
			if sub.key == "icloud" && !day.Before(s.today.AddDays(-20)) && !day.After(s.today.AddDays(-5)) {
				continue // never paid: the job marks it missed
			}
			if sub.key == "spotify" && !day.Before(s.today.AddDays(-45)) && !day.After(s.today.AddDays(-25)) {
				if _, err := usecase.NewChangeSubscriptionPrice(s.d).Do(ctx, s.p, s.hid, sub.id,
					usecase.ChangeSubscriptionPriceInput{EffectiveFrom: day, Price: pln(2699)}); err != nil {
					return fmt.Errorf("spotify price: %w", err)
				}
				c.Charge.Amount = 2699
			}
			cur := c.Charge.Currency
			wallet := "Karta kredytowa"
			if sub.walletID == s.wallets["Konto USD"] {
				wallet = "Konto USD"
			}
			t, err := usecase.NewRecordTransaction(s.d).Do(ctx, s.p, s.hid, usecase.RecordTransactionInput{WalletID: string(sub.walletID),
				Date: day, PayeeName: sub.payee, Amount: usecase.MoneyInput{Amount: amount(-c.Charge.Amount), Currency: cur},
				EnvelopeID: string(sub.envelopeID), Note: sub.in.Name})
			if err != nil {
				return fmt.Errorf("payment of %s from %s: %w", sub.key, wallet, err)
			}
			if day.After(s.today.AddDays(-demoLeaveOpenDays - 1)) {
				continue // left for the person to match
			}
			if _, err := usecase.NewConfirmChargeMatch(s.d).Do(ctx, s.p, s.hid, c.Charge.ID, t.Transaction.ID); err != nil {
				return fmt.Errorf("matching %s: %w", sub.key, err)
			}
		}
	}
	return nil
}

// finish creates the bot and counts the notifications.
func (s *seeder) finish(ctx context.Context, password string) (demoResult, error) {
	b, err := usecase.NewCreateBot(s.d).Do(ctx, s.p, s.hid, usecase.CreateBotInput{Name: demoBotName, Scopes: demoScopes})
	if err != nil {
		return demoResult{}, fmt.Errorf("bot: %w", err)
	}
	unread, err := usecase.NewCountUnreadNotifications(s.d).Do(ctx, s.p, s.hid)
	if err != nil {
		return demoResult{}, err
	}
	return demoResult{HouseholdID: s.hid, Password: password, BotToken: b.Token, Unread: unread}, nil
}
