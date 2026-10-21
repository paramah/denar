package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/adapter/sqlite"
	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/budget"
	"git.eiac.dev/tools/denar/internal/domain/subscription"
	"git.eiac.dev/tools/denar/internal/domain/user"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func openDemoDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "demo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlite.Migrate(context.Background(), db, sqlite.Migrations()); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestDemoSeed: the demo household has what the README promises, its
// budget holds the invariant, one envelope is over in the current month,
// the subscription summary is complete thanks to the rates set by hand,
// the bot's token works, and a second seed is refused unless --reset.
func TestDemoSeed(t *testing.T) {
	loc, _ := time.LoadLocation(demoTimezone)
	for _, now := range []time.Time{
		time.Date(2026, 10, 21, 22, 30, 0, 0, loc),
		time.Date(2026, 11, 1, 8, 0, 0, 0, loc), // first day of a month
	} {
		t.Run(now.Format("2006-01-02"), func(t *testing.T) { checkDemo(t, now) })
	}
}

func checkDemo(t *testing.T, now time.Time) {
	ctx := context.Background()
	db := openDemoDB(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	r, err := seedDemo(ctx, db, log, now, demoPassword, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r.BotToken, "dnr_") || r.Password != demoPassword || r.Unread == 0 {
		t.Errorf("result = %+v", r)
	}

	clock := &demoClock{}
	clock.set(now)
	d := useCaseDeps(db, log, clock, nil)
	u, err := d.Users.ByEmail(ctx, user.NormalizeEmail(demoEmail))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Hasher.Verify(demoPassword, u.PasswordHash) {
		t.Error("the demo password does not open the account")
	}
	p := usecase.Principal{User: u}

	wallets, err := usecase.NewListWallets(d).Do(ctx, p, r.HouseholdID, usecase.ListWalletsInput{})
	if err != nil {
		t.Fatal(err)
	}
	var onBudget int64
	for _, w := range wallets {
		if w.BasePending {
			t.Errorf("wallet %s waits for a rate", w.Wallet.Name)
		}
		if w.Wallet.OnBudget {
			onBudget += w.Base.Amount
		}
	}
	if len(wallets) != 6 {
		t.Errorf("%d wallets, want 6", len(wallets))
	}

	subs, err := usecase.NewListSubscriptions(d).Do(ctx, p, r.HouseholdID, usecase.ListSubscriptionsInput{})
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[subscription.Status]int{}
	for _, s := range subs {
		statuses[s.Subscription.Status]++
	}
	if len(subs) != 7 || statuses[subscription.Cancelled] != 1 || statuses[subscription.Trial] != 1 || statuses[subscription.Active] != 5 {
		t.Errorf("subscriptions = %d, statuses %v", len(subs), statuses)
	}
	summary, err := usecase.NewGetSubscriptionSummary(d).Do(ctx, p, r.HouseholdID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Incomplete || len(summary.MissingCurrencies) > 0 {
		t.Errorf("summary incomplete: %+v", summary.MissingCurrencies)
	}

	charges, err := usecase.NewListCharges(d).Do(ctx, p, r.HouseholdID, usecase.ListChargesInput{Limit: someInt(200)})
	if err != nil {
		t.Fatal(err)
	}
	byStatus := map[subscription.ChargeStatus]int{}
	for _, c := range charges.Items {
		byStatus[c.Charge.Status]++
	}
	if byStatus[subscription.Missed] != 1 || byStatus[subscription.Matched] < 10 {
		t.Errorf("charges by status = %v", byStatus)
	}

	month := budget.MonthOf(domainDate(now), 1).String()
	m, err := usecase.NewGetBudgetMonth(d).Do(ctx, p, r.HouseholdID, month)
	if err != nil {
		t.Fatal(err)
	}
	var over []string
	for _, g := range m.Groups {
		for _, e := range g.Envelopes {
			if e.Available < 0 {
				over = append(over, e.Envelope.Name)
			}
		}
	}
	if len(over) != 1 || over[0] != "Restauracje" {
		t.Errorf("overspent envelopes in %s = %v", month, over)
	}
	if m.Incomplete {
		t.Error("budget month incomplete")
	}
	if got := m.Available + m.ReadyToAssign + m.AssignedInFuture; got != onBudget {
		t.Errorf("invariant: available %d + ready %d + future %d = %d, on-budget balance %d",
			m.Available, m.ReadyToAssign, m.AssignedInFuture, got, onBudget)
	}
	if m.ReadyToAssign <= 0 {
		t.Errorf("ready to assign = %d", m.ReadyToAssign)
	}

	bot, err := usecase.NewAuthenticateBot(d.Bots, d.Clock, log).Do(ctx, r.BotToken)
	if err != nil || bot.Bot.Name != demoBotName {
		t.Errorf("bot = %+v, %v", bot.Bot, err)
	}

	// Idempotence: refused without --reset; --reset makes one household again.
	if _, err := seedDemo(ctx, db, log, now, demoPassword, false); err == nil || !strings.Contains(err.Error(), "--reset") {
		t.Errorf("second seed: %v", err)
	}
	if _, err := seedDemo(ctx, db, log, now, "another password", true); err == nil || !strings.Contains(err.Error(), "another password") {
		t.Errorf("seed with another password: %v", err)
	}
	r2, err := seedDemo(ctx, db, log, now, demoPassword, true)
	if err != nil {
		t.Fatal(err)
	}
	hs, err := usecase.NewListHouseholds(d).Do(ctx, p)
	if err != nil || len(hs) != 1 || hs[0].Household.ID != r2.HouseholdID {
		t.Errorf("households after reset = %+v, %v", hs, err)
	}
	if _, err := usecase.NewAuthenticateBot(d.Bots, d.Clock, log).Do(ctx, r.BotToken); err == nil {
		t.Error("the old bot's token still works after reset")
	}
}

// TestDemoCommand runs "denar demo" as the binary does.
func TestDemoCommand(t *testing.T) {
	setEnv(t)
	var out, errs bytes.Buffer
	if err := run(context.Background(), []string{"demo"}, &out, &errs); err != nil {
		t.Fatalf("demo: %v\n%s", err, errs.String())
	}
	for _, want := range []string{demoEmail, demoPassword, "dnr_", "claude mcp add --transport http denar http://localhost:8080/mcp"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if err := run(context.Background(), []string{"demo"}, &out, &errs); err == nil {
		t.Error("second demo without --reset succeeded")
	}
}

func someInt(n int) domain.Optional[int] { return domain.Some(n) }

func domainDate(t time.Time) domain.Date { return domain.DateOf(t, t.Location()) }
