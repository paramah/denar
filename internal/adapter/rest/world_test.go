package rest

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/budget"
	"git.eiac.dev/tools/denar/internal/domain/household"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/domain/payee"
	"git.eiac.dev/tools/denar/internal/domain/transaction"
	"git.eiac.dev/tools/denar/internal/domain/user"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
	"git.eiac.dev/tools/denar/internal/usecase"
	"git.eiac.dev/tools/denar/internal/usecase/fakes"
)

const (
	baseURL    = "https://denar.example.com"
	sessionTTL = 168 * time.Hour
	inviteTTL  = 168 * time.Hour
)

// world is the real use cases on fake ports, as the composition root wires
// them, with a household h1 of owner, admin, member and viewer, a household
// h2 of outsider, and a bot b1 of member in h1 with wallets:read.
type world struct {
	t *testing.T
	*fakes.Repos
	oidc *fakes.OIDCProvider
	logs *bytes.Buffer
	deps Deps
	vars map[string]string
}

func newWorld(t *testing.T) *world {
	r := fakes.NewRepos()
	w := &world{t: t, Repos: r, oidc: fakes.NewOIDCProvider(), logs: &bytes.Buffer{}, vars: map[string]string{}}
	d := r.Deps()
	log := slog.New(slog.NewJSONHandler(w.logs, nil))
	d.Log = log
	inv := usecase.InvitationConfig{TTL: inviteTTL, SessionTTL: sessionTTL}
	w.deps = Deps{
		BaseURL:         baseURL,
		Logger:          log,
		Clock:           r.Clock,
		SessionTTL:      sessionTTL,
		Authenticate:    usecase.NewAuthenticate(r.Sessions, r.Users, r.Clock, sessionTTL),
		AuthenticateBot: usecase.NewAuthenticateBot(r.Bots, r.Clock, log),
		LoginLocal: usecase.NewLoginLocal(r.Users, r.Hasher, r.Sessions, r.Tokens, r.Clock, r.Tx,
			usecase.LoginLocalConfig{Enabled: true, SessionTTL: sessionTTL}),
		LoginOIDCStart: usecase.NewLoginOIDCStart(w.oidc, r.OIDCStates, r.Tokens, r.Clock),
		LoginOIDCCallback: usecase.NewLoginOIDCCallback(w.oidc, r.OIDCStates, r.Users, r.Invitations, r.Sessions, r.Tokens, r.IDs,
			r.Clock, r.Tx, usecase.LoginOIDCCallbackConfig{SessionTTL: sessionTTL, AutoCreate: true}),
		Logout:        usecase.NewLogout(r.Sessions),
		GetAuthConfig: usecase.NewGetAuthConfig(w.oidc, true, "Denar", "v0.0.0-test"),
		GetMe:         usecase.NewGetMe(d),

		CreateHousehold: usecase.NewCreateHousehold(d),
		ListHouseholds:  usecase.NewListHouseholds(d),
		GetHousehold:    usecase.NewGetHousehold(d),
		UpdateHousehold: usecase.NewUpdateHousehold(d),
		DeleteHousehold: usecase.NewDeleteHousehold(d),

		ListMembers:      usecase.NewListMembers(d),
		UpdateMemberRole: usecase.NewUpdateMemberRole(d),
		RemoveMember:     usecase.NewRemoveMember(d),

		CreateInvitation:       usecase.NewCreateInvitation(d, inv),
		ListInvitations:        usecase.NewListInvitations(d),
		RevokeInvitation:       usecase.NewRevokeInvitation(d),
		PreviewInvitation:      usecase.NewPreviewInvitation(d),
		AcceptInvitation:       usecase.NewAcceptInvitation(d),
		RegisterWithInvitation: usecase.NewRegisterWithInvitation(d, inv),

		CreateBot:      usecase.NewCreateBot(d),
		ListBots:       usecase.NewListBots(d),
		GetBot:         usecase.NewGetBot(d),
		UpdateBot:      usecase.NewUpdateBot(d),
		RotateBotToken: usecase.NewRotateBotToken(d),
		DeleteBot:      usecase.NewDeleteBot(d),

		ListAudit: usecase.NewListAudit(d),

		ListWallets:  usecase.NewListWallets(d),
		CreateWallet: usecase.NewCreateWallet(d),
		GetWallet:    usecase.NewGetWallet(d),
		UpdateWallet: usecase.NewUpdateWallet(d),

		SearchTransactions: usecase.NewSearchTransactions(d),
		RecordTransaction:  usecase.NewRecordTransaction(d),
		GetTransaction:     usecase.NewGetTransaction(d),
		UpdateTransaction:  usecase.NewUpdateTransaction(d),
		DeleteTransaction:  usecase.NewDeleteTransaction(d),
		CreateTransfer:     usecase.NewCreateTransfer(d),

		ListPayees:  usecase.NewListPayees(d),
		CreatePayee: usecase.NewCreatePayee(d),
		UpdatePayee: usecase.NewUpdatePayee(d),
		DeletePayee: usecase.NewDeletePayee(d),

		QuoteFxRate:     usecase.NewQuoteFxRate(d),
		ListFxOverrides: usecase.NewListFxOverrides(d),
		SetFxOverride:   usecase.NewSetFxOverride(d),

		GetBudgetMonth:       usecase.NewGetBudgetMonth(d),
		ListEnvelopeActivity: usecase.NewListEnvelopeActivity(d),
		SetAssignment:        usecase.NewSetAssignment(d),
		MoveBetweenEnvelopes: usecase.NewMoveBetweenEnvelopes(d),
		AutoAssign:           usecase.NewAutoAssign(d),
		ListEnvelopeGroups:   usecase.NewListEnvelopeGroups(d),
		CreateEnvelopeGroup:  usecase.NewCreateEnvelopeGroup(d),
		UpdateEnvelopeGroup:  usecase.NewUpdateEnvelopeGroup(d),
		DeleteEnvelopeGroup:  usecase.NewDeleteEnvelopeGroup(d),
		ListEnvelopes:        usecase.NewListEnvelopes(d),
		CreateEnvelope:       usecase.NewCreateEnvelope(d),
		GetEnvelope:          usecase.NewGetEnvelope(d),
		UpdateEnvelope:       usecase.NewUpdateEnvelope(d),
		DeleteEnvelope:       usecase.NewDeleteEnvelope(d),
	}
	owner := w.addUser("owner")
	w.addHousehold("h1", owner)
	for _, m := range []struct {
		id   domain.ID
		role membership.Role
	}{{"admin", membership.RoleAdmin}, {"member", membership.RoleMember}, {"viewer", membership.RoleViewer}} {
		w.addMember("h1", w.addUser(m.id), m.role)
	}
	w.addHousehold("h2", w.addUser("outsider"))
	w.addBot("b1", "h1", "member", bot.ScopeWalletsRead)
	w.addFinances()
	return w
}

// addFinances gives h1 a shared PLN wallet w1 "Konto" and a private USD
// wallet w2 "Dolary" of owner, payees p1 "Lidl" and p2 "Aldi", and an
// expense t1 of 12.34 PLN in w1 at Lidl. NBP knows USD on 2026-10-20.
// The budget has a group g1 "Codzienne" with envelopes e1 "Jedzenie" (a
// monthly goal of 500 PLN, used by t1), groceries "Zakupy" and e2
// "Wolna", and an empty group g2 "Cele".
func (w *world) addFinances() {
	w.t.Helper()
	ctx := context.Background()
	now := w.Clock.Now()
	w.must(w.EnvelopeGroups.Create(ctx, budget.Group{ID: "g1", HouseholdID: "h1", Name: "Codzienne", CreatedAt: now, UpdatedAt: now}))
	w.must(w.EnvelopeGroups.Create(ctx, budget.Group{ID: "g2", HouseholdID: "h1", Name: "Cele", Position: 1, CreatedAt: now, UpdatedAt: now}))
	for _, e := range []budget.Envelope{
		{ID: "e1", Name: "Jedzenie", Goal: budget.Goal{Type: budget.GoalMonthly, Amount: 50000}},
		{ID: "groceries", Name: "Zakupy", Position: 1},
		{ID: "e2", Name: "Wolna", Position: 2, Hidden: true},
	} {
		e.HouseholdID, e.GroupID, e.CreatedAt, e.UpdatedAt = "h1", "g1", now, now
		w.must(w.Envelopes.Create(ctx, e))
	}
	w.must(w.Wallets.Create(ctx, wallet.Wallet{ID: "w1", HouseholdID: "h1", Name: "Konto", Type: wallet.TypeChecking, Currency: "PLN",
		OnBudget: true, Visibility: wallet.Shared, MemberIDs: []domain.ID{}, CreatedBy: "owner", CreatedAt: now}))
	w.must(w.Wallets.Create(ctx, wallet.Wallet{ID: "w2", HouseholdID: "h1", Name: "Dolary", Type: wallet.TypeCash, Currency: "USD",
		OnBudget: true, Visibility: wallet.Private, MemberIDs: []domain.ID{"owner"}, CreatedBy: "owner", CreatedAt: now}))
	for id, name := range map[domain.ID]string{"p1": "Lidl", "p2": "Aldi"} {
		w.must(w.Payees.Create(ctx, payee.Payee{ID: id, HouseholdID: "h1", Name: name, Patterns: []string{"LIDL SP"}, CreatedAt: now, UpdatedAt: now}))
	}
	d, _ := domain.ParseDate("2026-10-20")
	w.must(w.Transactions.Create(ctx, transaction.Transaction{ID: "t1", HouseholdID: "h1", WalletID: "w1", Date: d, Amount: -1234,
		Currency: "PLN", BaseAmount: -1234, PayeeID: "p1", EnvelopeID: "e1", Kind: transaction.Expense, Status: transaction.Cleared,
		Source: transaction.SourceManual, Tags: []string{"food"}, Note: "zakupy", ExternalID: "ext-1",
		Splits: []transaction.Split{}, CreatedBy: domain.Actor{Kind: domain.ActorUser, ID: "member"}, CreatedAt: now, UpdatedAt: now}))
	w.Rates.Set("USD", "2026-10-20", "4.0000")
}

func (w *world) must(err error) {
	w.t.Helper()
	if err != nil {
		w.t.Fatal(err)
	}
}

func passwordOf(id domain.ID) string { return "password-of-" + string(id) }

func (w *world) addUser(id domain.ID) user.User {
	w.t.Helper()
	hash, _ := w.Hasher.Hash(passwordOf(id))
	u := user.User{ID: id, Email: string(id) + "@example.com", Name: "User " + string(id), PasswordHash: hash, Active: true, CreatedAt: w.Clock.Now()}
	w.must(w.Users.Create(context.Background(), u))
	return u
}

func (w *world) addHousehold(id domain.ID, owner user.User) {
	w.t.Helper()
	w.must(w.Households.Create(context.Background(), household.Household{ID: id, Name: "Dom " + string(id), BaseCurrency: "PLN",
		Timezone: "Europe/Warsaw", MonthStartDay: 1, CreatedBy: owner.ID, CreatedAt: w.Clock.Now()}))
	w.addMember(id, owner, membership.RoleOwner)
}

func (w *world) addMember(householdID domain.ID, u user.User, role membership.Role) {
	w.t.Helper()
	w.must(w.Memberships.Create(context.Background(), membership.Membership{HouseholdID: householdID, UserID: u.ID, Role: role, CreatedAt: w.Clock.Now()}))
	w.Clock.Advance(time.Second)
}

func botToken(id domain.ID) string { return "dnr_token-of-" + string(id) }

func (w *world) addBot(id, householdID, creator domain.ID, scopes ...bot.Scope) {
	w.t.Helper()
	w.must(w.Bots.Create(context.Background(), bot.Bot{ID: id, HouseholdID: householdID, Name: "Bot " + string(id), Scopes: scopes,
		WalletIDs: []domain.ID{}, CreatedBy: creator, CreatedAt: w.Clock.Now(), TokenHash: usecase.HashToken(botToken(id)), TokenPrefix: "dnr_toke"}))
}

// session opens a session for the user and returns its cookie.
func (w *world) session(id domain.ID) *http.Cookie {
	w.t.Helper()
	token := "session-of-" + string(id)
	now := w.Clock.Now()
	if _, err := w.Sessions.ByTokenHash(context.Background(), usecase.HashToken(token)); err == nil {
		return &http.Cookie{Name: sessionCookieName, Value: token}
	}
	w.must(w.Sessions.Create(context.Background(), usecase.Session{TokenHash: usecase.HashToken(token), UserID: id,
		CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(sessionTTL)}))
	return &http.Cookie{Name: sessionCookieName, Value: token}
}

// client sends requests to a handler, with its cookies or bot token.
type client struct {
	handler http.Handler
	cookies []*http.Cookie
	bearer  string
}

func (w *world) handler() http.Handler { return NewServer(w.deps).Handler() }

// as returns a client of h acting as who: a user id, "bot" for b1, or ""
// for nobody.
func (w *world) as(h http.Handler, who string) *client {
	c := &client{handler: h}
	switch who {
	case "":
	case "bot":
		c.bearer = botToken("b1")
	default:
		c.cookies = []*http.Cookie{w.session(domain.ID(who))}
	}
	return c
}

func (c *client) do(method, target, body string, header ...string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	return rec
}

// cookie returns the named cookie the response sets, or nil.
func cookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

var contract = sync.OnceValue(func() validator.Validator {
	spec, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		panic(err)
	}
	doc, err := libopenapi.NewDocument(spec)
	if err != nil {
		panic(err)
	}
	v, errs := validator.NewValidator(doc)
	if len(errs) > 0 {
		panic(errs)
	}
	return v
})

// wantContract checks that the response is one api/openapi.yaml declares
// for the operation, in status, media type and body.
func wantContract(t *testing.T, method, target string, rec *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	if ok, errs := contract().ValidateHttpResponse(req, rec.Result()); !ok {
		for _, e := range errs {
			t.Errorf("%s %s -> %d breaks the contract: %s: %s", method, target, rec.Code, e.Message, e.Reason)
		}
	}
}
