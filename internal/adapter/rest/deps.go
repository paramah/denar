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
}
