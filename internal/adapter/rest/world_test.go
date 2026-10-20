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
	"git.eiac.dev/tools/denar/internal/domain/household"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/domain/user"
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
	return w
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
