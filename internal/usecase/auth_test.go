package usecase_test

import (
	"errors"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/membership"
	"github.com/paramah/denar/internal/usecase"
)

const ttl = 168 * time.Hour

func (w *world) loginLocal(enabled bool) *usecase.LoginLocal {
	return usecase.NewLoginLocal(w.Users, w.Hasher, w.Sessions, w.Tokens, w.Clock, w.Tx,
		usecase.LoginLocalConfig{Enabled: enabled, SessionTTL: ttl})
}

func TestLoginLocal(t *testing.T) {
	w := newWorld(t)
	ada := w.addUser("ada")
	inactive := w.addUser("gone")
	inactive.Active = false
	w.must(w.Users.Update(ctx, inactive))
	login := w.loginLocal(true)

	got, err := login.Do(ctx, usecase.LoginLocalInput{Email: " ADA@example.com ", Password: passwordOf("ada")})
	noErr(t, err)
	if got.User.ID != ada.ID || got.Token == "" || !got.ExpiresAt.Equal(w.Clock.Now().Add(ttl)) {
		t.Errorf("login = %+v", got)
	}
	s, err := w.Sessions.ByTokenHash(ctx, usecase.HashToken(got.Token))
	noErr(t, err)
	if s.UserID != ada.ID {
		t.Errorf("session = %+v", s)
	}
	if _, err := w.Sessions.ByTokenHash(ctx, got.Token); !errors.Is(err, domain.ErrNotFound) {
		t.Error("the token itself is stored")
	}

	for _, in := range []usecase.LoginLocalInput{
		{Email: "ada@example.com", Password: "wrong password!"},
		{Email: "nobody@example.com", Password: passwordOf("ada")},
		{Email: "gone@example.com", Password: passwordOf("gone")},
	} {
		_, err := login.Do(ctx, in)
		if err != domain.ErrUnauthenticated {
			t.Errorf("%s: err = %v, want bare ErrUnauthenticated", in.Email, err)
		}
	}
	_, err = w.loginLocal(false).Do(ctx, usecase.LoginLocalInput{Email: "ada@example.com", Password: passwordOf("ada")})
	wantErr(t, err, domain.ErrUnauthenticated)

	// A new login replaces the session the browser had.
	again, err := login.Do(ctx, usecase.LoginLocalInput{Email: "ada@example.com", Password: passwordOf("ada"), PreviousToken: got.Token})
	noErr(t, err)
	if _, err := w.Sessions.ByTokenHash(ctx, usecase.HashToken(got.Token)); !errors.Is(err, domain.ErrNotFound) {
		t.Error("previous session survives")
	}
	if again.Token == got.Token {
		t.Error("token reused")
	}
}

func TestAuthenticateSession(t *testing.T) {
	w := newWorld(t)
	w.addUser("ada")
	login, err := w.loginLocal(true).Do(ctx, usecase.LoginLocalInput{Email: "ada@example.com", Password: passwordOf("ada")})
	noErr(t, err)
	auth := usecase.NewAuthenticate(w.Sessions, w.Users, w.Clock, ttl)

	got, err := auth.Do(ctx, login.Token)
	noErr(t, err)
	if got.Principal.User.ID != "ada" || got.Renewed {
		t.Errorf("auth = %+v", got)
	}
	// Used after more than an hour, the session slides.
	w.Clock.Advance(2 * time.Hour)
	got, err = auth.Do(ctx, login.Token)
	noErr(t, err)
	if !got.Renewed || !got.ExpiresAt.Equal(w.Clock.Now().Add(ttl)) {
		t.Errorf("not renewed: %+v", got)
	}
	// Idle for the whole TTL, it ends.
	w.Clock.Advance(ttl)
	_, err = auth.Do(ctx, login.Token)
	wantErr(t, err, domain.ErrUnauthenticated)
	_, err = auth.Do(ctx, "")
	wantErr(t, err, domain.ErrUnauthenticated)

	logout := usecase.NewLogout(w.Sessions)
	noErr(t, logout.Do(ctx, "unknown"))
	noErr(t, logout.Do(ctx, ""))
}

func TestLogoutEndsSession(t *testing.T) {
	w := newWorld(t)
	w.addUser("ada")
	login, err := w.loginLocal(true).Do(ctx, usecase.LoginLocalInput{Email: "ada@example.com", Password: passwordOf("ada")})
	noErr(t, err)
	noErr(t, usecase.NewLogout(w.Sessions).Do(ctx, login.Token))
	_, err = usecase.NewAuthenticate(w.Sessions, w.Users, w.Clock, ttl).Do(ctx, login.Token)
	wantErr(t, err, domain.ErrUnauthenticated)
}

func TestAuthenticateBot(t *testing.T) {
	f := newFamily(t)
	b := f.addBot("b1", "h1", f.member, bot.ScopeWalletsRead)
	auth := usecase.NewAuthenticateBot(f.Bots, f.Clock, nil)

	p, err := auth.Do(ctx, botToken("b1"))
	noErr(t, err)
	if !p.IsBot() || p.Bot.ID != "b1" || p.Actor() != (domain.Actor{Kind: domain.ActorBot, ID: "b1"}) {
		t.Errorf("principal = %+v", p)
	}
	stored, _ := f.Bots.ByID(ctx, "b1")
	if !stored.LastUsedAt.Equal(f.Clock.Now()) {
		t.Errorf("LastUsedAt = %v", stored.LastUsedAt)
	}
	// Within a minute the time is not written again.
	first := f.Clock.Now()
	f.Clock.Advance(30 * time.Second)
	_, err = auth.Do(ctx, botToken("b1"))
	noErr(t, err)
	if stored, _ := f.Bots.ByID(ctx, "b1"); !stored.LastUsedAt.Equal(first) {
		t.Errorf("touched within a minute: %v", stored.LastUsedAt)
	}
	f.Clock.Advance(31 * time.Second)
	_, err = auth.Do(ctx, botToken("b1"))
	noErr(t, err)
	if stored, _ := f.Bots.ByID(ctx, "b1"); !stored.LastUsedAt.Equal(f.Clock.Now()) {
		t.Errorf("not touched after a minute: %v", stored.LastUsedAt)
	}
	// A failed write does not lock the bot out.
	f.Bots.TouchUsedErr = errors.New("disk full")
	f.Clock.Advance(time.Hour)
	_, err = auth.Do(ctx, botToken("b1"))
	noErr(t, err)
	f.Bots.TouchUsedErr = nil

	for _, tok := range []string{"", "slg_whatever", "dnr_unknown", "token-of-b1"} {
		_, err := auth.Do(ctx, tok)
		if err != domain.ErrUnauthenticated {
			t.Errorf("token %q: err = %v", tok, err)
		}
	}

	expiring := b
	expiring.ExpiresAt = f.Clock.Now().Add(time.Minute)
	f.must(f.Bots.Update(ctx, expiring))
	_, err = auth.Do(ctx, botToken("b1"))
	noErr(t, err)
	f.Clock.Advance(time.Minute)
	_, err = auth.Do(ctx, botToken("b1"))
	wantErr(t, err, domain.ErrUnauthenticated)

	disabled := f.addBot("b2", "h1", f.member, bot.ScopeWalletsRead)
	disabled.Disabled = true
	f.must(f.Bots.Update(ctx, disabled))
	_, err = auth.Do(ctx, botToken("b2"))
	wantErr(t, err, domain.ErrUnauthenticated)
}

func TestBootstrapAdmin(t *testing.T) {
	w := newWorld(t)
	uc := usecase.NewBootstrapAdmin(w.Users, w.Hasher, w.IDs, w.Clock, w.Tx)
	created, err := uc.Do(ctx, usecase.BootstrapAdminInput{Email: "Admin@Example.com", Password: "a long admin password", Name: ""})
	noErr(t, err)
	if !created {
		t.Fatal("not created")
	}
	u, err := w.Users.ByEmail(ctx, "admin@example.com")
	noErr(t, err)
	if !u.InstanceAdmin || !u.Active || u.Name != "Admin" || !w.Hasher.Verify("a long admin password", u.PasswordHash) {
		t.Errorf("admin = %+v", u)
	}
	created, err = uc.Do(ctx, usecase.BootstrapAdminInput{Email: "admin@example.com", Password: "another long password"})
	noErr(t, err)
	if created {
		t.Error("created twice")
	}
	if u2, _ := w.Users.ByEmail(ctx, "admin@example.com"); u2.PasswordHash != u.PasswordHash {
		t.Error("password overwritten")
	}
	_, err = uc.Do(ctx, usecase.BootstrapAdminInput{Email: "x@example.com", Password: "short"})
	wantErr(t, err, domain.ErrInvalid)
}

func (w *world) oidcCallback(autoCreate bool) (*usecase.LoginOIDCStart, *usecase.LoginOIDCCallback, *fakeOIDC) {
	p := newFakeOIDC()
	start := usecase.NewLoginOIDCStart(p, w.OIDCStates, w.Tokens, w.Clock)
	cb := usecase.NewLoginOIDCCallback(p, w.OIDCStates, w.Users, w.Invitations, w.Sessions, w.Tokens, w.IDs, w.Clock, w.Tx,
		usecase.LoginOIDCCallbackConfig{SessionTTL: ttl, AutoCreate: autoCreate})
	return start, cb, p
}

func TestLoginOIDC(t *testing.T) {
	verified, unverified := true, false
	identity := usecase.OIDCIdentity{Issuer: "https://idp.test", Subject: "sub-1", Email: "Grace@example.com", EmailVerified: &verified, Name: "Grace"}

	run := func(t *testing.T, w *world, autoCreate bool, id usecase.OIDCIdentity) (usecase.OIDCLogin, error) {
		t.Helper()
		start, cb, p := w.oidcCallback(autoCreate)
		p.On, p.Identity = true, id
		s, err := start.Do(ctx, usecase.LoginOIDCStartInput{ReturnTo: "/households/h1"})
		noErr(t, err)
		return cb.Do(ctx, usecase.LoginOIDCCallbackInput{Code: "code", State: s.State, CookieState: s.State})
	}

	t.Run("no account and no invitation is refused", func(t *testing.T) {
		w := newWorld(t)
		_, err := run(t, w, false, identity)
		var le usecase.OIDCLoginError
		if !errors.As(err, &le) || le.Reason != usecase.OIDCNotInvited || !errors.Is(err, domain.ErrUnauthenticated) {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("a pending invitation lets the account be created", func(t *testing.T) {
		f := newFamily(t)
		_, err := usecase.NewCreateInvitation(f.deps, usecase.InvitationConfig{TTL: ttl}).
			Do(ctx, human(f.owner), "h1", usecase.CreateInvitationInput{Email: "grace@example.com", Role: membership.RoleMember})
		noErr(t, err)
		got, err := run(t, f.world, false, identity)
		noErr(t, err)
		if got.User.Email != "grace@example.com" || got.User.OIDCSubject != "sub-1" || got.ReturnTo != "/households/h1" || got.Token == "" {
			t.Errorf("login = %+v", got)
		}
		// The second login finds the account by subject.
		again, err := run(t, f.world, false, identity)
		noErr(t, err)
		if again.User.ID != got.User.ID {
			t.Error("second login made another account")
		}
	})

	t.Run("auto-create gives anyone an account", func(t *testing.T) {
		w := newWorld(t)
		got, err := run(t, w, true, identity)
		noErr(t, err)
		if got.User.Name != "Grace" {
			t.Errorf("user = %+v", got.User)
		}
	})

	t.Run("unverified or missing email and a taken email are refused", func(t *testing.T) {
		w := newWorld(t)
		w.addUser("grace")
		for reason, id := range map[usecase.OIDCLoginFailure]usecase.OIDCIdentity{
			usecase.OIDCEmailUnverified: {Issuer: "https://idp.test", Subject: "s2", Email: "x@example.com", EmailVerified: &unverified},
			usecase.OIDCEmailMissing:    {Issuer: "https://idp.test", Subject: "s3", EmailVerified: &verified},
			usecase.OIDCEmailTaken:      {Issuer: "https://idp.test", Subject: "s4", Email: "grace@example.com", EmailVerified: &verified},
		} {
			_, err := run(t, w, true, id)
			var le usecase.OIDCLoginError
			if !errors.As(err, &le) || le.Reason != reason {
				t.Errorf("%s: err = %v", reason, err)
			}
		}
	})

	t.Run("a state that does not match the cookie is refused once used", func(t *testing.T) {
		w := newWorld(t)
		start, cb, p := w.oidcCallback(true)
		p.On, p.Identity = true, identity
		s, err := start.Do(ctx, usecase.LoginOIDCStartInput{})
		noErr(t, err)
		_, err = cb.Do(ctx, usecase.LoginOIDCCallbackInput{Code: "c", State: s.State, CookieState: "other"})
		wantErr(t, err, domain.ErrUnauthenticated)
		_, err = cb.Do(ctx, usecase.LoginOIDCCallbackInput{Code: "c", State: s.State, CookieState: s.State})
		noErr(t, err)
		_, err = cb.Do(ctx, usecase.LoginOIDCCallbackInput{Code: "c", State: s.State, CookieState: s.State})
		wantErr(t, err, domain.ErrUnauthenticated)
	})

	t.Run("OIDC off", func(t *testing.T) {
		w := newWorld(t)
		start, _, _ := w.oidcCallback(false)
		_, err := start.Do(ctx, usecase.LoginOIDCStartInput{})
		wantErr(t, err, domain.ErrNotFound)
		cfg := usecase.NewGetAuthConfig(newFakeOIDC(), true, "Denar", "dev").Do()
		if cfg.OIDCEnabled || !cfg.LocalLogin || cfg.InstanceName != "Denar" {
			t.Errorf("config = %+v", cfg)
		}
	})
}
