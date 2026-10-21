package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/budget"
	"github.com/paramah/denar/internal/domain/household"
	"github.com/paramah/denar/internal/domain/membership"
	"github.com/paramah/denar/internal/domain/user"
	"github.com/paramah/denar/internal/domain/wallet"
	"github.com/paramah/denar/internal/usecase"
	"github.com/paramah/denar/internal/usecase/fakes"
)

var ctx = context.Background()

// world is the use cases over fake ports, with helpers to set the stage.
type world struct {
	t *testing.T
	*fakes.Repos
	deps usecase.Deps
	// vars keeps ids a test set up, by name.
	vars map[string]domain.ID
}

func newWorld(t *testing.T) *world {
	r := fakes.NewRepos()
	return &world{t: t, Repos: r, deps: r.Deps(), vars: map[string]domain.ID{}}
}

func (w *world) must(err error) {
	w.t.Helper()
	if err != nil {
		w.t.Fatal(err)
	}
}

func passwordOf(id domain.ID) string { return "password-of-" + string(id) }

// addUser adds an active local account with the password passwordOf(id).
func (w *world) addUser(id domain.ID) user.User {
	w.t.Helper()
	hash, _ := w.Hasher.Hash(passwordOf(id))
	u := user.User{ID: id, Email: string(id) + "@example.com", Name: "User " + string(id), PasswordHash: hash, Active: true, CreatedAt: w.Clock.Now()}
	w.must(w.Users.Create(ctx, u))
	return u
}

// addHousehold adds household id with the owner.
func (w *world) addHousehold(id domain.ID, owner user.User) household.Household {
	w.t.Helper()
	h := household.Household{ID: id, Name: "Dom " + string(id), BaseCurrency: "PLN", Timezone: "Europe/Warsaw", MonthStartDay: 1, CreatedBy: owner.ID, CreatedAt: w.Clock.Now()}
	w.must(w.Households.Create(ctx, h))
	w.addMember(id, owner, membership.RoleOwner)
	return h
}

func (w *world) addMember(householdID domain.ID, u user.User, role membership.Role) {
	w.t.Helper()
	w.must(w.Memberships.Create(ctx, membership.Membership{HouseholdID: householdID, UserID: u.ID, Role: role, CreatedAt: w.Clock.Now()}))
	w.Clock.Advance(time.Second)
}

// addBot stores a bot with the token "dnr_token-of-<id>".
func (w *world) addBot(id, householdID domain.ID, creator user.User, scopes ...bot.Scope) bot.Bot {
	w.t.Helper()
	b := bot.Bot{ID: id, HouseholdID: householdID, Name: "Bot " + string(id), Scopes: scopes, WalletIDs: []domain.ID{},
		CreatedBy: creator.ID, CreatedAt: w.Clock.Now(), TokenHash: usecase.HashToken(botToken(id)), TokenPrefix: "dnr_toke"}
	w.must(w.Bots.Create(ctx, b))
	return b
}

// addWallet adds a PLN checking wallet on budget; a private one is seen by
// its creator and the members given.
func (w *world) addWallet(id, householdID domain.ID, creator user.User, v wallet.Visibility, members ...domain.ID) wallet.Wallet {
	w.t.Helper()
	return w.addWalletIn("PLN", id, householdID, creator, v, members...)
}

func (w *world) addWalletIn(currency string, id, householdID domain.ID, creator user.User, v wallet.Visibility, members ...domain.ID) wallet.Wallet {
	w.t.Helper()
	wl := wallet.Wallet{ID: id, HouseholdID: householdID, Name: "Wallet " + string(id), Type: wallet.TypeChecking, Currency: currency,
		OnBudget: true, Visibility: v, MemberIDs: members, CreatedBy: creator.ID, CreatedAt: w.Clock.Now()}.Normalize()
	w.must(w.Wallets.Create(ctx, wl))
	return wl
}

func botToken(id domain.ID) string { return "dnr_token-of-" + string(id) }

func human(u user.User) usecase.Principal { return usecase.Principal{User: u} }

func asBot(b bot.Bot) usecase.Principal { return usecase.Principal{Bot: b} }

// family is a household h1 with an owner, an admin, a member and a viewer,
// and an outsider with a household h2 of their own.
type family struct {
	*world
	owner, admin, member, viewer, outsider user.User
}

func newFamily(t *testing.T) *family {
	w := newWorld(t)
	f := &family{world: w,
		owner: w.addUser("owner"), admin: w.addUser("admin"), member: w.addUser("member"),
		viewer: w.addUser("viewer"), outsider: w.addUser("outsider")}
	w.addHousehold("h1", f.owner)
	w.addMember("h1", f.admin, membership.RoleAdmin)
	w.addMember("h1", f.member, membership.RoleMember)
	w.addMember("h1", f.viewer, membership.RoleViewer)
	w.addHousehold("h2", f.outsider)
	return f
}

func (f *family) byRole(r membership.Role) user.User {
	switch r {
	case membership.RoleOwner:
		return f.owner
	case membership.RoleAdmin:
		return f.admin
	case membership.RoleMember:
		return f.member
	case membership.RoleViewer:
		return f.viewer
	}
	return f.outsider
}

func wantErr(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("err = %v, want %v", err, target)
	}
}

func wantForbidden(t *testing.T, err error, reason domain.ForbiddenReason) {
	t.Helper()
	var fe domain.ForbiddenError
	if !errors.As(err, &fe) || fe.Reason != reason {
		t.Fatalf("err = %v, want forbidden %s", err, reason)
	}
}

func wantConflict(t *testing.T, err error, reason domain.ConflictReason) {
	t.Helper()
	var ce domain.ConflictError
	if !errors.As(err, &ce) || ce.Reason != reason {
		t.Fatalf("err = %v, want conflict %s", err, reason)
	}
}

func noErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// actions returns the audit actions recorded so far, oldest first.
func (w *world) actions() []string {
	var out []string
	for _, e := range w.Audit.All() {
		out = append(out, e.Action)
	}
	return out
}

func day(s string) domain.Date {
	d, err := domain.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func pln(amount string) usecase.MoneyInput {
	return usecase.MoneyInput{Amount: amount, Currency: "PLN"}
}

func usd(amount string) usecase.MoneyInput {
	return usecase.MoneyInput{Amount: amount, Currency: "USD"}
}

func wantField(t *testing.T, err error, field string) {
	t.Helper()
	if !errors.Is(err, domain.ErrInvalid) || !hasField(err, field) {
		t.Fatalf("err = %v, want a validation error on %s", err, field)
	}
}

func hasField(err error, field string) bool {
	switch e := err.(type) {
	case domain.ValidationError:
		return e.Field == field
	case interface{ Unwrap() []error }:
		for _, c := range e.Unwrap() {
			if hasField(c, field) {
				return true
			}
		}
	case interface{ Unwrap() error }:
		return hasField(e.Unwrap(), field)
	}
	return false
}

// spend records an expense in the wallet as the principal.
func (w *world) spend(p usecase.Principal, walletID string, amount usecase.MoneyInput, date string) usecase.TransactionView {
	w.t.Helper()
	v, err := usecase.NewRecordTransaction(w.deps).Do(ctx, p, "h1", usecase.RecordTransactionInput{WalletID: walletID, Amount: amount, Date: day(date)})
	w.must(err)
	return v
}

// addEnvelopes adds the envelopes, named after their ids, to a group
// "g-<household>" of the household, creating the group when needed.
func (w *world) addEnvelopes(householdID domain.ID, ids ...domain.ID) {
	w.t.Helper()
	gid := "g-" + householdID
	if _, err := w.EnvelopeGroups.ByID(ctx, householdID, gid); err != nil {
		w.must(w.EnvelopeGroups.Create(ctx, budget.Group{ID: gid, HouseholdID: householdID, Name: "Grupa " + string(householdID),
			CreatedAt: w.Clock.Now(), UpdatedAt: w.Clock.Now()}))
	}
	for _, id := range ids {
		w.must(w.Envelopes.Create(ctx, budget.Envelope{ID: id, HouseholdID: householdID, GroupID: gid, Name: string(id),
			CreatedAt: w.Clock.Now(), UpdatedAt: w.Clock.Now()}))
	}
}
