package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/paramah/denar/internal/adapter/csvimport"
	"github.com/paramah/denar/internal/adapter/fakeconnector"
	"github.com/paramah/denar/internal/adapter/mcp"
	"github.com/paramah/denar/internal/adapter/nbp"
	"github.com/paramah/denar/internal/adapter/notify/matrix"
	"github.com/paramah/denar/internal/adapter/notify/notifyhttp"
	"github.com/paramah/denar/internal/adapter/notify/ntfy"
	"github.com/paramah/denar/internal/adapter/notify/smtp"
	"github.com/paramah/denar/internal/adapter/notify/telegram"
	"github.com/paramah/denar/internal/adapter/notify/webhook"
	"github.com/paramah/denar/internal/adapter/oidc"
	"github.com/paramah/denar/internal/adapter/rest"
	"github.com/paramah/denar/internal/adapter/spa"
	"github.com/paramah/denar/internal/adapter/sqldb"
	"github.com/paramah/denar/internal/domain/notification"
	"github.com/paramah/denar/internal/platform/clock"
	"github.com/paramah/denar/internal/platform/config"
	"github.com/paramah/denar/internal/platform/crypto"
	"github.com/paramah/denar/internal/platform/id"
	"github.com/paramah/denar/internal/platform/password"
	"github.com/paramah/denar/internal/platform/token"
	"github.com/paramah/denar/internal/usecase"
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
	// runSubscriptions runs the subscription jobs once.
	runSubscriptions func(ctx context.Context) (usecase.SubscriptionJobResult, error)
	// runNotifications dispatches events and sends deliveries once;
	// checkBudgets appends the envelope events once.
	runNotifications func(ctx context.Context) (usecase.NotificationJobResult, error)
	checkBudgets     func(ctx context.Context) (int, error)

	db             *sqldb.DB
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
	// noSubscriptionJobs leaves the subscription jobs to the test, which
	// runs them with app.runSubscriptions.
	noSubscriptionJobs bool
	// noNotificationJobs leaves dispatch, deliveries and budget checks to
	// the test (app.runNotifications, app.checkBudgets).
	noNotificationJobs bool
}

func newApp(ctx context.Context, cfg config.Config, log *slog.Logger, panel fs.FS) (*app, error) {
	return newAppWith(ctx, cfg, log, panel, appOptions{})
}

func newAppWith(ctx context.Context, cfg config.Config, log *slog.Logger, panel fs.FS, opts appOptions) (*app, error) {
	db, err := sqldb.Open(cfg.Database)
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
func useCaseDeps(db *sqldb.DB, log *slog.Logger, c usecase.Clock, rates usecase.RateProvider) usecase.Deps {
	if c == nil {
		c = clock.System{}
	}
	return usecase.Deps{
		Users:        sqldb.NewUserRepository(db),
		Households:   sqldb.NewHouseholdRepository(db),
		Memberships:  sqldb.NewMembershipRepository(db),
		Invitations:  sqldb.NewInvitationRepository(db),
		Bots:         sqldb.NewBotRepository(db),
		Audit:        sqldb.NewAuditRepository(db),
		Sessions:     sqldb.NewSessionRepository(db),
		Wallets:      sqldb.NewWalletRepository(db),
		Transactions: sqldb.NewTransactionRepository(db),
		Payees:       sqldb.NewPayeeRepository(db),
		FxRates:      sqldb.NewFxRateRepository(db),
		FxOverrides:  sqldb.NewFxOverrideRepository(db),

		EnvelopeGroups: sqldb.NewEnvelopeGroupRepository(db),
		Envelopes:      sqldb.NewEnvelopeRepository(db),
		Assignments:    sqldb.NewAssignmentRepository(db),

		Subscriptions: sqldb.NewSubscriptionRepository(db),
		Charges:       sqldb.NewChargeRepository(db),
		Events:        sqldb.NewEventOutbox(db),

		Notifications: sqldb.NewNotificationRepository(db),
		Channels:      sqldb.NewChannelRepository(db),
		Preferences:   sqldb.NewPreferenceRepository(db),
		Deliveries:    sqldb.NewDeliveryRepository(db),

		ImportBatches: sqldb.NewImportBatchRepository(db),
		Connections:   sqldb.NewConnectionRepository(db),
		CSV:           csvimport.New(),

		Rates:  rates,
		IDs:    id.New(),
		Tokens: token.Generator{},
		Clock:  c,
		Hasher: password.Hasher{},
		Tx:     db,
		Log:    log,
	}
}

func wire(ctx context.Context, cfg config.Config, log *slog.Logger, db *sqldb.DB, panel fs.FS, opts appOptions) (*app, error) {
	if cfg.AutoMigrate {
		if err := sqldb.Migrate(ctx, db, db.Migrations()); err != nil {
			return nil, err
		}
	} else if err := sqldb.CheckSchemaCurrent(ctx, db, db.Migrations()); err != nil {
		return nil, err
	}

	// The use cases log who did what; the request it was is added from the
	// context.
	rates := opts.rates
	if rates == nil {
		rates = nbp.New(nbp.Config{BaseURL: cfg.NBPURL})
	}
	d := useCaseDeps(db, rest.WithRequestID(log), opts.clock, rates)
	d.ChargeHorizonDays = cfg.ChargeHorizonDays
	if err := wireNotifications(cfg, log, &d); err != nil {
		return nil, err
	}
	d.Connectors = connectors(cfg, d.Clock)
	states := sqldb.NewOIDCStateRepository(db)
	provider := oidc.New(oidc.Config{
		Issuer:       cfg.OIDCIssuer,
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret.Reveal(),
		RedirectURL:  cfg.BaseURL + "/api/auth/callback",
	})

	if err := bootstrapAdmin(ctx, cfg, log, usecase.NewBootstrapAdmin(d.Users, d.Hasher, d.IDs, d.Clock, d.Tx)); err != nil {
		return nil, err
	}

	inv := usecase.InvitationConfig{TTL: cfg.InviteTTL, SessionTTL: cfg.SessionTTL, BaseURL: cfg.BaseURL}
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
		UpdateMe:      usecase.NewUpdateMe(d),

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

		ListVendors:             usecase.NewListVendors(),
		ListSubscriptions:       usecase.NewListSubscriptions(d),
		CreateSubscription:      usecase.NewCreateSubscription(d),
		GetSubscription:         usecase.NewGetSubscription(d),
		UpdateSubscription:      usecase.NewUpdateSubscription(d),
		DeleteSubscription:      usecase.NewDeleteSubscription(d),
		ListSubscriptionPrices:  usecase.NewListSubscriptionPrices(d),
		ChangeSubscriptionPrice: usecase.NewChangeSubscriptionPrice(d),
		GetSubscriptionSummary:  usecase.NewGetSubscriptionSummary(d),
		ListUpcomingRenewals:    usecase.NewListUpcomingRenewals(d),
		ListCharges:             usecase.NewListCharges(d),
		GetCharge:               usecase.NewGetCharge(d),
		SuggestTransactions:     usecase.NewSuggestTransactions(d),
		SuggestCharges:          usecase.NewSuggestCharges(d),
		ConfirmChargeMatch:      usecase.NewConfirmChargeMatch(d),
		UnmatchCharge:           usecase.NewUnmatchCharge(d),
		PostCharge:              usecase.NewPostCharge(d),
		SkipCharge:              usecase.NewSkipCharge(d),

		ListNotifications:          usecase.NewListNotifications(d),
		CountUnreadNotifications:   usecase.NewCountUnreadNotifications(d),
		MarkNotificationRead:       usecase.NewMarkNotificationRead(d),
		MarkAllNotificationsRead:   usecase.NewMarkAllNotificationsRead(d),
		ListChannelTypes:           usecase.NewListChannelTypes(d),
		ListChannels:               usecase.NewListChannels(d),
		CreateChannel:              usecase.NewCreateChannel(d),
		GetChannel:                 usecase.NewGetChannel(d),
		UpdateChannel:              usecase.NewUpdateChannel(d),
		DeleteChannel:              usecase.NewDeleteChannel(d),
		TestChannel:                usecase.NewTestChannel(d),
		GetNotificationPreferences: usecase.NewGetNotificationPreferences(d),
		SetNotificationPreferences: usecase.NewSetNotificationPreferences(d),

		ListImportPresets:       usecase.NewListImportPresets(d),
		PreviewCSVImport:        usecase.NewPreviewCSVImport(d),
		ImportCSV:               usecase.NewImportCSV(d),
		ListImports:             usecase.NewListImports(d),
		GetImport:               usecase.NewGetImport(d),
		RevertImport:            usecase.NewRevertImport(d),
		ListConnectionProviders: usecase.NewListConnectionProviders(d),
		ListConnections:         usecase.NewListConnections(d),
	})

	assistant := mcp.NewHandler(mcp.Deps{
		Logger:         log,
		Clock:          d.Clock,
		TrustedProxies: cfg.TrustedProxies,
		RatePerMinute:  cfg.BotRate,
		Version:        version,
		Authenticate:   usecase.NewAuthenticateBot(d.Bots, d.Clock, log),
		UseCases:       d,
	})

	backgroundCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	a := &app{
		handler:        rootHandler(server.Handler(), assistant, spa.Handler(panel, log)),
		deps:           d,
		db:             db,
		stopBackground: stop,
		backgroundDone: make(chan struct{}),
	}
	fill := usecase.NewFillPendingRates(d)
	a.fillRates = func(ctx context.Context) (int, error) { return fill.Do(ctx) }
	subscriptionJobs := usecase.NewRunSubscriptionJobs(d)
	a.runSubscriptions = subscriptionJobs.Do
	notificationJobs := usecase.NewRunNotificationJobs(d)
	a.runNotifications = notificationJobs.Do
	budgetChecks := usecase.NewCheckBudgets(d)
	a.checkBudgets = budgetChecks.Do
	prune := usecase.NewPruneNotifications(d)
	go func() {
		defer close(a.backgroundDone)
		var wg sync.WaitGroup
		wg.Go(func() { cleanExpired(backgroundCtx, log, d.Clock, d.Sessions, states) })
		wg.Go(func() { fillPendingRates(backgroundCtx, log, cfg.FxInterval, fill) })
		wg.Go(func() { every(backgroundCtx, log, cleanupInterval, "pruning notifications", logPrune(log, prune)) })
		if !opts.noSubscriptionJobs {
			wg.Go(func() { runSubscriptionJobs(backgroundCtx, log, cfg.SubscriptionInterval, subscriptionJobs) })
		}
		if !opts.noNotificationJobs {
			wg.Go(func() {
				every(backgroundCtx, log, cfg.NotifyInterval, "running the notification jobs", logNotifications(log, notificationJobs))
			})
			wg.Go(func() {
				every(backgroundCtx, log, cfg.BudgetCheckInterval, "checking the budgets", logBudgetChecks(log, budgetChecks))
			})
		}
		wg.Wait()
	}()
	return a, nil
}

// rootHandler routes between the adapters. The web UI is mounted next to
// the REST handler, not inside it: that handler asks every route it knows
// for credentials, and the UI has to load without them. /mcp is the MCP
// adapter (stage 006); nothing lives below it.
func rootHandler(api, assistant, panel http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/", api)
	mux.Handle("/api", api) // or the mux would redirect it to /api/
	mux.Handle("/healthz", api)
	mux.Handle("/mcp", assistant)
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

// runSubscriptionJobs keeps the charges of subscriptions up to the
// horizon, marks missed ones and puts reminders in the outbox (stage 004)
// now and once per interval, until ctx is done.
func runSubscriptionJobs(ctx context.Context, log *slog.Logger, interval time.Duration, jobs *usecase.RunSubscriptionJobs) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		r, err := jobs.Do(ctx)
		switch {
		case err != nil && ctx.Err() == nil:
			log.Error("running the subscription jobs", "error", err.Error())
		case r != (usecase.SubscriptionJobResult{}):
			log.Info("subscription jobs", "charges_created", r.Created, "charges_missed", r.Missed, "reminders", r.Reminders)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// wireNotifications gives the use cases the channels the server can send
// over: webhook, ntfy, Telegram and Matrix always, e-mail with SMTP; and
// the secret box with a master key.
func wireNotifications(cfg config.Config, log *slog.Logger, d *usecase.Deps) error {
	d.BaseURL, d.GoalDueDays = cfg.BaseURL, cfg.GoalDueDays
	// Channels send to public addresses only unless the administrator
	// opened private networks (009, SSRF): checked when a channel is saved
	// and again on every connection.
	d.NotifyTargets = notification.TargetPolicy{AllowPrivate: cfg.NotifyAllowPrivate, Allowed: cfg.NotifyAllowedCIDRs}
	client := notifyhttp.NewClient(d.NotifyTargets)
	d.Notifiers = map[notification.ChannelType]usecase.Notifier{
		notification.Webhook:  &webhook.Notifier{Client: client, Log: log},
		notification.Ntfy:     &ntfy.Notifier{Client: client, Log: log},
		notification.Telegram: &telegram.Notifier{APIURL: cfg.TelegramAPIURL, Client: client, Log: log},
		notification.Matrix:   &matrix.Notifier{Client: client, Log: log},
	}
	if cfg.NotifyAllowPrivate {
		log.Warn("DENAR_NOTIFY_ALLOW_PRIVATE is on: notification channels may reach private networks of the server")
	}
	if cfg.SMTPEnabled() {
		mailer, err := smtp.New(smtp.Config{Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword.Reveal(), From: cfg.SMTPFrom, TLS: cfg.SMTPTLS})
		if err != nil {
			return err
		}
		d.Mailer = mailer
		d.Notifiers[notification.Email] = mailer
	}
	if cfg.MasterKey != "" {
		key, err := crypto.ParseKey(cfg.MasterKey.Reveal())
		if err != nil {
			return fmt.Errorf("DENAR_MASTER_KEY: %w", err)
		}
		box, err := crypto.NewBox(key)
		if err != nil {
			return err
		}
		d.Secrets = box
	} else {
		log.Info("DENAR_MASTER_KEY is not set: channels with secrets (Telegram, Matrix, signed webhooks, ntfy tokens) are unavailable")
	}
	return nil
}

// connectors is the registry of connection providers (stage 008): none
// yet but the pretend bank of development. Stages 010-012 add the real
// ones here.
func connectors(cfg config.Config, c usecase.Clock) map[string]usecase.ConnectorProvider {
	out := map[string]usecase.ConnectorProvider{}
	if cfg.DevFakeConnector {
		out[fakeconnector.Key] = fakeconnector.Provider(c.Now)
	}
	return out
}

// every runs job now and once per interval, until ctx is done; an error
// is logged as what failed.
func every(ctx context.Context, log *slog.Logger, interval time.Duration, what string, job func(ctx context.Context) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := job(ctx); err != nil && ctx.Err() == nil {
			log.Error(what, "error", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func logNotifications(log *slog.Logger, jobs *usecase.RunNotificationJobs) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		r, err := jobs.Do(ctx)
		if r != (usecase.NotificationJobResult{}) {
			log.Info("notification jobs", "events", r.Dispatch.Events, "notifications", r.Dispatch.Notifications,
				"deliveries", r.Dispatch.Deliveries, "sent", r.Delivery.Sent, "retried", r.Delivery.Retried, "failed", r.Delivery.Failed)
		}
		return err
	}
}

func logBudgetChecks(log *slog.Logger, checks *usecase.CheckBudgets) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		n, err := checks.Do(ctx)
		if n > 0 {
			log.Info("budget events", "events", n)
		}
		return err
	}
}

func logPrune(log *slog.Logger, prune *usecase.PruneNotifications) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		r, err := prune.Do(ctx)
		if r != (usecase.PruneResult{}) {
			log.Info("notifications pruned", "notifications", r.Notifications, "deliveries", r.Deliveries, "events", r.Events)
		}
		return err
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
