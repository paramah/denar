package rest

import (
	"context"
	"log/slog"
	"net/netip"
	"time"

	"git.eiac.dev/tools/denar/internal/usecase"
)

// Deps is what the REST adapter needs from the composition root.
type Deps struct {
	// BaseURL is the public origin of the server; requests from it pass the
	// cross-origin check, cookies are Secure when it is https, and
	// invitation links start with it.
	BaseURL string
	// Logger receives the request log and unexpected errors; nil means
	// slog.Default.
	Logger *slog.Logger
	// Clock times the rate limits; nil means the system clock.
	Clock usecase.Clock
	// TrustedProxies are the networks of the reverse proxies in front of
	// the server; only they are believed about X-Forwarded-For.
	TrustedProxies []netip.Prefix
	// SessionTTL is the Max-Age of the session cookie.
	SessionTTL time.Duration
	// BotRate is how many requests a bot may make per minute; 0 means 120.
	BotRate int
	// Health checks the database for /healthz; nil always answers ok.
	Health func(ctx context.Context) error

	Authenticate      *usecase.Authenticate
	AuthenticateBot   *usecase.AuthenticateBot
	LoginLocal        *usecase.LoginLocal
	LoginOIDCStart    *usecase.LoginOIDCStart
	LoginOIDCCallback *usecase.LoginOIDCCallback
	Logout            *usecase.Logout
	GetAuthConfig     *usecase.GetAuthConfig
	GetMe             *usecase.GetMe

	CreateHousehold *usecase.CreateHousehold
	ListHouseholds  *usecase.ListHouseholds
	GetHousehold    *usecase.GetHousehold
	UpdateHousehold *usecase.UpdateHousehold
	DeleteHousehold *usecase.DeleteHousehold

	ListMembers      *usecase.ListMembers
	UpdateMemberRole *usecase.UpdateMemberRole
	RemoveMember     *usecase.RemoveMember

	CreateInvitation       *usecase.CreateInvitation
	ListInvitations        *usecase.ListInvitations
	RevokeInvitation       *usecase.RevokeInvitation
	PreviewInvitation      *usecase.PreviewInvitation
	AcceptInvitation       *usecase.AcceptInvitation
	RegisterWithInvitation *usecase.RegisterWithInvitation

	CreateBot      *usecase.CreateBot
	ListBots       *usecase.ListBots
	GetBot         *usecase.GetBot
	UpdateBot      *usecase.UpdateBot
	RotateBotToken *usecase.RotateBotToken
	DeleteBot      *usecase.DeleteBot

	ListAudit *usecase.ListAudit

	ListWallets  *usecase.ListWallets
	CreateWallet *usecase.CreateWallet
	GetWallet    *usecase.GetWallet
	UpdateWallet *usecase.UpdateWallet

	SearchTransactions *usecase.SearchTransactions
	RecordTransaction  *usecase.RecordTransaction
	GetTransaction     *usecase.GetTransaction
	UpdateTransaction  *usecase.UpdateTransaction
	DeleteTransaction  *usecase.DeleteTransaction
	CreateTransfer     *usecase.CreateTransfer

	ListPayees  *usecase.ListPayees
	CreatePayee *usecase.CreatePayee
	UpdatePayee *usecase.UpdatePayee
	DeletePayee *usecase.DeletePayee

	QuoteFxRate     *usecase.QuoteFxRate
	ListFxOverrides *usecase.ListFxOverrides
	SetFxOverride   *usecase.SetFxOverride

	GetBudgetMonth       *usecase.GetBudgetMonth
	ListEnvelopeActivity *usecase.ListEnvelopeActivity
	SetAssignment        *usecase.SetAssignment
	MoveBetweenEnvelopes *usecase.MoveBetweenEnvelopes
	AutoAssign           *usecase.AutoAssign
	ListEnvelopeGroups   *usecase.ListEnvelopeGroups
	CreateEnvelopeGroup  *usecase.CreateEnvelopeGroup
	UpdateEnvelopeGroup  *usecase.UpdateEnvelopeGroup
	DeleteEnvelopeGroup  *usecase.DeleteEnvelopeGroup
	ListEnvelopes        *usecase.ListEnvelopes
	CreateEnvelope       *usecase.CreateEnvelope
	GetEnvelope          *usecase.GetEnvelope
	UpdateEnvelope       *usecase.UpdateEnvelope
	DeleteEnvelope       *usecase.DeleteEnvelope

	ListVendors             *usecase.ListVendors
	ListSubscriptions       *usecase.ListSubscriptions
	CreateSubscription      *usecase.CreateSubscription
	GetSubscription         *usecase.GetSubscription
	UpdateSubscription      *usecase.UpdateSubscription
	DeleteSubscription      *usecase.DeleteSubscription
	ListSubscriptionPrices  *usecase.ListSubscriptionPrices
	ChangeSubscriptionPrice *usecase.ChangeSubscriptionPrice
	GetSubscriptionSummary  *usecase.GetSubscriptionSummary
	ListUpcomingRenewals    *usecase.ListUpcomingRenewals
	ListCharges             *usecase.ListCharges
	GetCharge               *usecase.GetCharge
	SuggestTransactions     *usecase.SuggestTransactions
	SuggestCharges          *usecase.SuggestCharges
	ConfirmChargeMatch      *usecase.ConfirmChargeMatch
	UnmatchCharge           *usecase.UnmatchCharge
	PostCharge              *usecase.PostCharge
	SkipCharge              *usecase.SkipCharge

	ListNotifications          *usecase.ListNotifications
	CountUnreadNotifications   *usecase.CountUnreadNotifications
	MarkNotificationRead       *usecase.MarkNotificationRead
	MarkAllNotificationsRead   *usecase.MarkAllNotificationsRead
	ListChannelTypes           *usecase.ListChannelTypes
	ListChannels               *usecase.ListChannels
	CreateChannel              *usecase.CreateChannel
	GetChannel                 *usecase.GetChannel
	UpdateChannel              *usecase.UpdateChannel
	DeleteChannel              *usecase.DeleteChannel
	TestChannel                *usecase.TestChannel
	GetNotificationPreferences *usecase.GetNotificationPreferences
	SetNotificationPreferences *usecase.SetNotificationPreferences
}
