package usecase_test

import (
	"errors"
	"strings"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/payee"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// resolveWorld has shared wallets "Karta" and "Karta firmowa", a cash
// wallet, and a private wallet of the owner.
func resolveWorld(t *testing.T) *family {
	f := newFamily(t)
	for id, name := range map[domain.ID]string{"w-card": "Karta", "w-biz": "Karta firmowa", "w-cash": "Gotówka"} {
		w := f.addWallet(id, "h1", f.owner, wallet.Shared)
		w.Name = name
		f.must(f.Wallets.Update(ctx, w))
	}
	p := f.addWallet("w-priv", "h1", f.owner, wallet.Private)
	p.Name = "Oszczędności"
	f.must(f.Wallets.Update(ctx, p))
	f.addEnvelopes("h1", "Jedzenie", "Jedzenie na mieście", "Paliwo")
	return f
}

func TestResolveWallet(t *testing.T) {
	f := resolveWorld(t)
	r := usecase.NewResolve(f.deps)
	for _, tc := range []struct {
		ref, want string
	}{
		{"w-card", "w-card"},          // id
		{"karta", "w-card"},           // name, any case
		{"  KARTA FIRMOWA ", "w-biz"}, // trimmed
		{"gotów", "w-cash"},           // the only name containing it
		{"firm", "w-biz"},
	} {
		w, err := r.Wallet(ctx, human(f.member), "h1", "wallet", tc.ref)
		if err != nil || string(w.ID) != tc.want {
			t.Errorf("%q: %v %v, want %s", tc.ref, w.ID, err, tc.want)
		}
	}

	_, err := r.Wallet(ctx, human(f.member), "h1", "wallet", "kart")
	wantField(t, err, "wallet")
	if !strings.Contains(err.Error(), `"Karta"`) || !strings.Contains(err.Error(), `"Karta firmowa"`) {
		t.Errorf("ambiguity names no candidates: %v", err)
	}

	// The member does not see the owner's private wallet, by id or name.
	for _, ref := range []string{"w-priv", "Oszczędności", "nope"} {
		_, err = r.Wallet(ctx, human(f.member), "h1", "wallet", ref)
		wantErr(t, err, domain.ErrNotFound)
		var nf usecase.RefNotFound
		if !errors.As(err, &nf) || nf.Kind != "wallet" {
			t.Errorf("%q: err = %#v", ref, err)
		}
	}
	w, err := r.Wallet(ctx, human(f.owner), "h1", "wallet", "oszczędności")
	if err != nil || w.ID != "w-priv" {
		t.Errorf("owner: %v %v", w.ID, err)
	}

	// No reference, no wallet.
	w, err = r.Wallet(ctx, human(f.member), "h1", "wallet", " ")
	if err != nil || w.ID != "" {
		t.Errorf("empty: %+v %v", w, err)
	}
}

func TestResolveForBots(t *testing.T) {
	f := resolveWorld(t)
	r := usecase.NewResolve(f.deps)
	b := f.addBot("b1", "h1", f.owner, bot.ScopeTransactionsWrite)
	b.WalletIDs = []domain.ID{"w-cash"}
	f.must(f.Bots.Update(ctx, b))

	// The scopes come first: without them nothing is revealed.
	_, err := r.Wallet(ctx, asBot(b), "h1", "wallet", "kart", bot.ScopeBudgetWrite)
	wantForbidden(t, err, domain.ForbiddenScope)

	// Outside its wallet list a wallet does not exist for the bot.
	_, err = r.Wallet(ctx, asBot(b), "h1", "wallet", "w-card", bot.ScopeTransactionsWrite)
	wantErr(t, err, domain.ErrNotFound)
	// So "kart" is no longer ambiguous, it matches nothing.
	_, err = r.Wallet(ctx, asBot(b), "h1", "wallet", "kart", bot.ScopeTransactionsWrite)
	wantErr(t, err, domain.ErrNotFound)
	w, err := r.Wallet(ctx, asBot(b), "h1", "wallet", "gotówka", bot.ScopeTransactionsWrite)
	if err != nil || w.ID != "w-cash" {
		t.Errorf("own wallet: %v %v", w.ID, err)
	}
	// Another household's bot.
	other := f.addBot("b2", "h2", f.outsider, bot.ScopeTransactionsWrite)
	_, err = r.Wallet(ctx, asBot(other), "h1", "wallet", "w-cash")
	wantErr(t, err, domain.ErrNotFound)
}

func TestResolveEnvelopePayeeSubscription(t *testing.T) {
	f := resolveWorld(t)
	r := usecase.NewResolve(f.deps)
	e, err := r.Envelope(ctx, human(f.member), "h1", "envelope", "jedzenie")
	if err != nil || e.ID != "Jedzenie" {
		t.Errorf("exact name wins over a longer one: %v %v", e.ID, err)
	}
	e, err = r.Envelope(ctx, human(f.member), "h1", "envelope", "mieście")
	if err != nil || e.ID != "Jedzenie na mieście" {
		t.Errorf("substring: %v %v", e.ID, err)
	}
	_, err = r.Envelope(ctx, human(f.member), "h1", "envelope", "jedz")
	wantField(t, err, "envelope")
	_, err = r.Envelope(ctx, human(f.member), "h1", "envelope", "czynsz")
	wantErr(t, err, domain.ErrNotFound)

	f.must(f.Payees.Create(ctx, payee.Payee{ID: "p1", HouseholdID: "h1", Name: "Biedronka", Patterns: []string{}}))
	pe, err := r.Payee(ctx, human(f.member), "h1", "payee", "BIEDRONKA")
	if err != nil || pe.ID != "p1" {
		t.Errorf("payee: %v %v", pe.ID, err)
	}

	sub := f.subscribeAs(t, human(f.member), "Netflix", "")
	s, err := r.Subscription(ctx, human(f.member), "h1", "subscription", "netflix")
	if err != nil || s.ID != sub {
		t.Errorf("subscription: %v %v", s.ID, err)
	}
	// A subscription paid from a wallet the member does not see is hidden.
	hidden := f.subscribeAs(t, human(f.owner), "Spotify", "w-priv")
	_, err = r.Subscription(ctx, human(f.member), "h1", "subscription", string(hidden))
	wantErr(t, err, domain.ErrNotFound)
	_, err = r.Subscription(ctx, human(f.member), "h1", "subscription", "spotify")
	wantErr(t, err, domain.ErrNotFound)
}

// subscribeAs records a monthly 10.00 PLN subscription and returns its id.
func (f *family) subscribeAs(t *testing.T, p usecase.Principal, name, walletID string) domain.ID {
	t.Helper()
	v, err := usecase.NewCreateSubscription(f.deps).Do(ctx, p, "h1", usecase.CreateSubscriptionInput{Name: name, Vendor: "other",
		Price: pln("10.00"), Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-10-01")}, WalletID: walletID})
	noErr(t, err)
	return v.Subscription.ID
}
