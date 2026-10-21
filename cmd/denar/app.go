package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"git.eiac.dev/tools/denar/internal/adapter/nbp"
	"git.eiac.dev/tools/denar/internal/adapter/oidc"
	"git.eiac.dev/tools/denar/internal/adapter/rest"
	"git.eiac.dev/tools/denar/internal/adapter/spa"
	"git.eiac.dev/tools/denar/internal/adapter/sqlite"
	"git.eiac.dev/tools/denar/internal/platform/clock"
	"git.eiac.dev/tools/denar/internal/platform/config"
	"git.eiac.dev/tools/denar/internal/platform/id"
	"git.eiac.dev/tools/denar/internal/platform/password"
	"git.eiac.dev/tools/denar/internal/platform/token"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// cleanupInterval is how often expired sessions and OIDC states are dropped
// (FR-013).
const cleanupInterval = time.Hour

// app is the wired application: the HTTP handler, serving the API and the
// web UI, over the database.
type app struct {
	handler http.Handler
	deps    usecase.Deps
	// fillRates runs the job that fills in missing base amounts once.
	fillRates func(ctx context.Context) (int, error)

	db             *sqlite.DB
	stopBackground context.CancelFunc
	backgroundDone chan struct{}
	closeOnce      sync.Once
	closeErr       error
}

// appOptions replace parts of the application in tests; the zero value is
// what the binary runs with.
type appOptions struct {
	clock usecase.Clock // nil means the system clock
	// rates replaces the NBP provider; nil means NBP at cfg.NBPURL.
	rates usecase.RateProvider
}

func newApp(ctx context.Context, cfg config.Config, log *slog.Logger, panel fs.FS) (*app, error) {
	return newAppWith(ctx, cfg, log, panel, appOptions{})
}

func newAppWith(ctx context.Context, cfg config.Config, log *slog.Logger, panel fs.FS, opts appOptions) (*app, error) {
	db, err := sqlite.Open(cfg.DatabasePath)
	if err != nil {
		return nil, err
	}
	a, err := wire(ctx, cfg, log, db, panel, opts)
	if err != nil {
		db.Close()
		return nil, err
	}
	return a, nil
}

// useCaseDeps builds the dependencies of the use cases over the database.
// Without a rate provider only the base currency, rates set by hand and
// cached rates are known (the command line needs none).
func useCaseDeps(db *sqlite.DB, log *slog.Logger, c usecase.Clock, rates usecase.RateProvider) usecase.Deps {
	if c == nil {
		c = clock.System{}
	}
	return usecase.Deps{
		Users:        sqlite.NewUserRepository(db),
		Households:   sqlite.NewHouseholdRepository(db),
		Memberships:  sqlite.NewMembershipRepository(db),
		Invitations:  sqlite.NewInvitationRepository(db),
		Bots:         sqlite.NewBotRepository(db),
		Audit:        sqlite.NewAuditRepository(db),
		Sessions:     sqlite.NewSessionRepository(db),
		Wallets:      sqlite.NewWalletRepository(db),
		Transactions: sqlite.NewTransactionRepository(db),
		Payees:       sqlite.NewPayeeRepository(db),
		FxRates:      sqlite.NewFxRateRepository(db),
		FxOverrides:  sqlite.NewFxOverrideRepository(db),

		EnvelopeGroups: sqlite.NewEnvelopeGroupRepository(db),
		Envelopes:      sqlite.NewEnvelopeRepository(db),
		Assignments:    sqlite.NewAssignmentRepository(db),

		Rates:  rates,
		IDs:    id.New(),
		Tokens: token.Generator{},
		Clock:  c,
		Hasher: password.Hasher{},
		Tx:     db,
		Log:    log,
	}
}

func wire(ctx context.Context, cfg config.Config, log *slog.Logger, db *sqlite.DB, panel fs.FS, opts appOptions) (*app, error) {
	if cfg.AutoMigrate {
		if err := sqlite.Migrate(ctx, db, sqlite.Migrations()); err != nil {
			return nil, err
		}
	} else if err := sqlite.CheckSchemaCurrent(ctx, db, sqlite.Migrations()); err != nil {
		return nil, err
	}

	// The use cases log who did what; the request it was is added from the
	// context.
	rates := opts.rates
	if rates == nil {
		rates = nbp.New(nbp.Config{BaseURL: cfg.NBPURL})
	}
	d := useCaseDeps(db, rest.WithRequestID(log), opts.clock, rates)
	states := sqlite.NewOIDCStateRepository(db)
	provider := oidc.New(oidc.Config{
		Issuer:       cfg.OIDCIssuer,
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret.Reveal(),
		RedirectURL:  cfg.BaseURL + "/api/auth/callback",
	})

	if err := bootstrapAdmin(ctx, cfg, log, usecase.NewBootstrapAdmin(d.Users, d.Hasher, d.IDs, d.Clock, d.Tx)); err != nil {
		return nil, err
	}

	inv := usecase.InvitationConfig{TTL: cfg.InviteTTL, SessionTTL: cfg.SessionTTL}
	server := rest.NewServer(rest.Deps{
		BaseURL:        cfg.BaseURL,
		Logger:         log,
		Clock:          d.Clock,
		TrustedProxies: cfg.TrustedProxies,
		SessionTTL:     cfg.SessionTTL,
		BotRate:        cfg.BotRate,
		Health:         db.Ping,

		Authenticate:    usecase.NewAuthenticate(d.Sessions, d.Users, d.Clock, cfg.SessionTTL),
		AuthenticateBot: usecase.NewAuthenticateBot(d.Bots, d.Clock, log),
		LoginLocal: usecase.NewLoginLocal(d.Users, d.Hasher, d.Sessions, d.Tokens, d.Clock, d.Tx,
			usecase.LoginLocalConfig{Enabled: cfg.LocalLogin, SessionTTL: cfg.SessionTTL}),
		LoginOIDCStart: usecase.NewLoginOIDCStart(provider, states, d.Tokens, d.Clock),
		LoginOIDCCallback: usecase.NewLoginOIDCCallback(provider, states, d.Users, d.Invitations, d.Sessions, d.Tokens, d.IDs,
			d.Clock, d.Tx, usecase.LoginOIDCCallbackConfig{SessionTTL: cfg.SessionTTL, AutoCreate: cfg.OIDCAutoCreate}),
		Logout:        usecase.NewLogout(d.Sessions),
		GetAuthConfig: usecase.NewGetAuthConfig(provider, cfg.LocalLogin, cfg.InstanceName, version),
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
	})

	backgroundCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	a := &app{
		handler:        rootHandler(server.Handler(), spa.Handler(panel, log)),
		deps:           d,
		db:             db,
		stopBackground: stop,
		backgroundDone: make(chan struct{}),
	}
	fill := usecase.NewFillPendingRates(d)
	a.fillRates = func(ctx context.Context) (int, error) { return fill.Do(ctx) }
	go func() {
		defer close(a.backgroundDone)
		var wg sync.WaitGroup
		wg.Go(func() { cleanExpired(backgroundCtx, log, d.Clock, d.Sessions, states) })
		wg.Go(func() { fillPendingRates(backgroundCtx, log, cfg.FxInterval, fill) })
		wg.Wait()
	}()
	return a, nil
}

// rootHandler routes between the adapters. The web UI is mounted next to
// the REST handler, not inside it: that handler asks every route it knows
// for credentials, and the UI has to load without them. /mcp is kept for
// the MCP adapter of stage 006 and is not found until then.
func rootHandler(api, panel http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/", api)
	mux.Handle("/api", api) // or the mux would redirect it to /api/
	mux.Handle("/healthz", api)
	mux.Handle("/mcp", http.NotFoundHandler())
	mux.Handle("/mcp/", http.NotFoundHandler())
	mux.Handle("/", panel)
	return mux
}

// bootstrapAdmin creates the configured instance admin if it is not there
// yet (FR-003).
func bootstrapAdmin(ctx context.Context, cfg config.Config, log *slog.Logger, bootstrap *usecase.BootstrapAdmin) error {
	if !cfg.HasAdmin() {
		if cfg.LocalLogin && !cfg.OIDCEnabled() {
			log.Warn("no bootstrap account is configured; set DENAR_ADMIN_EMAIL and DENAR_ADMIN_PASSWORD to create one")
		}
		return nil
	}
	created, err := bootstrap.Do(ctx, usecase.BootstrapAdminInput{
		Email:    cfg.AdminEmail,
		Password: cfg.AdminPassword.Reveal(),
		Name:     cfg.AdminName,
	})
	if err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	if created {
		log.Info("bootstrap admin created")
	}
	return nil
}

// cleanExpired drops expired sessions and OIDC login states now and once
// per cleanupInterval, until ctx is done.
func cleanExpired(ctx context.Context, log *slog.Logger, clock usecase.Clock, sessions usecase.SessionRepository, states usecase.OIDCStateRepository) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		now := clock.Now()
		if err := sessions.DeleteExpired(ctx, now); err != nil && ctx.Err() == nil {
			log.Error("dropping expired sessions", "error", err.Error())
		}
		if err := states.DeleteExpired(ctx, now); err != nil && ctx.Err() == nil {
			log.Error("dropping expired OIDC states", "error", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// fillPendingRates fills in the base amounts of transactions saved without
// a rate (FR-111) now and once per interval, until ctx is done.
func fillPendingRates(ctx context.Context, log *slog.Logger, interval time.Duration, fill *usecase.FillPendingRates) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		n, err := fill.Do(ctx)
		switch {
		case err != nil && ctx.Err() == nil:
			log.Error("filling in exchange rates", "error", err.Error())
		case n > 0:
			log.Info("exchange rates filled in", "transactions", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Close stops the background tasks and closes the database. It may be
// called more than once.
func (a *app) Close() error {
	a.closeOnce.Do(func() {
		a.stopBackground()
		<-a.backgroundDone
		a.closeErr = a.db.Close()
	})
	return a.closeErr
}
