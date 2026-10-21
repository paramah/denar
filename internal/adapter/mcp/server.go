package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"runtime/debug"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/usecase"
)

const (
	// maxRequestBody is the largest request body accepted.
	maxRequestBody = 1 << 20
	// callTimeout is how long one tool call may take.
	callTimeout = 20 * time.Second
	// DefaultRatePerMinute is how many requests a bot may make per minute
	// when Deps names no rate (DENAR_BOT_RATE's default).
	DefaultRatePerMinute = 120
	// methodToolsCall is the JSON-RPC method of a tool call.
	methodToolsCall = "tools/call"
)

// Deps is what the MCP adapter needs from the composition root.
type Deps struct {
	// Logger receives the request log and unexpected errors; nil means
	// slog.Default.
	Logger *slog.Logger
	// Clock times the limits; nil means UseCases.Clock, else the system
	// clock.
	Clock usecase.Clock
	// TrustedProxies are the networks of the reverse proxies in front of
	// the server; only they are believed about X-Forwarded-For.
	TrustedProxies []netip.Prefix
	// RatePerMinute is how many requests one bot may make per minute;
	// zero means DefaultRatePerMinute.
	RatePerMinute int
	// Version is reported to clients as the version of the server.
	Version string

	// Authenticate resolves the bearer token of every request.
	Authenticate *usecase.AuthenticateBot
	// UseCases are the dependencies the tools' use cases are built from
	// (AD-611).
	UseCases usecase.Deps
}

// useCases are the interactors the tools call.
type useCases struct {
	resolve  *usecase.Resolve
	whoami   *usecase.Whoami
	spending *usecase.SpendingReporter

	listWallets        *usecase.ListWallets
	searchTransactions *usecase.SearchTransactions
	recordTransaction  *usecase.RecordTransaction
	createTransfer     *usecase.CreateTransfer
	listPayees         *usecase.ListPayees

	listEnvelopeGroups   *usecase.ListEnvelopeGroups
	listEnvelopes        *usecase.ListEnvelopes
	getBudgetMonth       *usecase.GetBudgetMonth
	setAssignment        *usecase.SetAssignment
	moveBetweenEnvelopes *usecase.MoveBetweenEnvelopes

	listVendors             *usecase.ListVendors
	listSubscriptions       *usecase.ListSubscriptions
	getSubscription         *usecase.GetSubscription
	listSubscriptionPrices  *usecase.ListSubscriptionPrices
	createSubscription      *usecase.CreateSubscription
	updateSubscription      *usecase.UpdateSubscription
	changeSubscriptionPrice *usecase.ChangeSubscriptionPrice
	subscriptionSummary     *usecase.GetSubscriptionSummary
	upcomingRenewals        *usecase.ListUpcomingRenewals
	listCharges             *usecase.ListCharges
	getCharge               *usecase.GetCharge
	suggestCharges          *usecase.SuggestCharges
	confirmChargeMatch      *usecase.ConfirmChargeMatch
	postCharge              *usecase.PostCharge
	skipCharge              *usecase.SkipCharge

	listNotifications *usecase.ListNotifications
	countUnread       *usecase.CountUnreadNotifications
}

func newUseCases(d usecase.Deps) useCases {
	return useCases{
		resolve:  usecase.NewResolve(d),
		whoami:   usecase.NewWhoami(d),
		spending: usecase.NewSpendingReport(d),

		listWallets:        usecase.NewListWallets(d),
		searchTransactions: usecase.NewSearchTransactions(d),
		recordTransaction:  usecase.NewRecordTransaction(d),
		createTransfer:     usecase.NewCreateTransfer(d),
		listPayees:         usecase.NewListPayees(d),

		listEnvelopeGroups:   usecase.NewListEnvelopeGroups(d),
		listEnvelopes:        usecase.NewListEnvelopes(d),
		getBudgetMonth:       usecase.NewGetBudgetMonth(d),
		setAssignment:        usecase.NewSetAssignment(d),
		moveBetweenEnvelopes: usecase.NewMoveBetweenEnvelopes(d),

		listVendors:             usecase.NewListVendors(),
		listSubscriptions:       usecase.NewListSubscriptions(d),
		getSubscription:         usecase.NewGetSubscription(d),
		listSubscriptionPrices:  usecase.NewListSubscriptionPrices(d),
		createSubscription:      usecase.NewCreateSubscription(d),
		updateSubscription:      usecase.NewUpdateSubscription(d),
		changeSubscriptionPrice: usecase.NewChangeSubscriptionPrice(d),
		subscriptionSummary:     usecase.NewGetSubscriptionSummary(d),
		upcomingRenewals:        usecase.NewListUpcomingRenewals(d),
		listCharges:             usecase.NewListCharges(d),
		getCharge:               usecase.NewGetCharge(d),
		suggestCharges:          usecase.NewSuggestCharges(d),
		confirmChargeMatch:      usecase.NewConfirmChargeMatch(d),
		postCharge:              usecase.NewPostCharge(d),
		skipCharge:              usecase.NewSkipCharge(d),

		listNotifications: usecase.NewListNotifications(d),
		countUnread:       usecase.NewCountUnreadNotifications(d),
	}
}

// server holds what the tools share.
type server struct {
	deps  Deps
	log   *slog.Logger
	clock usecase.Clock
	uc    useCases
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// NewHandler returns the handler of the MCP endpoint; the composition root
// mounts it at /mcp. Every request is a POST with the token of a bot.
func NewHandler(deps Deps) http.Handler {
	s := &server{deps: deps, log: deps.Logger, clock: deps.Clock, uc: newUseCases(deps.UseCases)}
	if s.log == nil {
		s.log = slog.Default()
	}
	if s.clock == nil {
		s.clock = deps.UseCases.Clock
	}
	if s.clock == nil {
		s.clock = systemClock{}
	}
	if s.deps.RatePerMinute <= 0 {
		s.deps.RatePerMinute = DefaultRatePerMinute
	}

	version := deps.Version
	if version == "" {
		version = "dev"
	}
	mcpServer := sdk.NewServer(&sdk.Implementation{Name: "denar", Title: "Denar", Version: version}, &sdk.ServerOptions{
		Instructions: serverInstructions,
	})
	mcpServer.AddReceivingMiddleware(invalidArguments)
	s.addReadTools(mcpServer)
	s.addBudgetTools(mcpServer)
	s.addSubscriptionTools(mcpServer)
	s.addWriteTools(mcpServer)
	s.addNotificationTools(mcpServer)

	var h http.Handler = sdk.NewStreamableHTTPHandler(
		func(*http.Request) *sdk.Server { return mcpServer },
		&sdk.StreamableHTTPOptions{
			Stateless:           true,
			JSONResponse:        true,
			MaxRequestBodyBytes: maxRequestBody,
			// See point 7 of the package comment.
			DisableLocalhostProtection: true,
		},
	)
	// The SDK middleware puts the principal where the SDK hands it to the
	// tools; by then the request is authenticated.
	h = auth.RequireBearerToken(verifiedPrincipal, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(h)
	h = singleMessage(h)
	h = s.limitRequests(h)
	h = s.authenticate(h)
	h = http.NewCrossOriginProtection().Handler(h)
	h = postOnly(h)
	h = noSniff(h)
	h = s.logRequests(h)
	return s.recoverPanics(h)
}

// serverInstructions tell the model how the tools fit together.
const serverInstructions = `Denar is a household budget: wallets, transactions, envelope budgeting (YNAB style) and subscriptions.
You act as a bot of one household; call whoami first to learn the household, its base currency, today's date there, your scopes and your wallets.
Amounts are decimal strings such as "12.34" (dot as the separator); results give money as {amount, currency, minor}.
Dates are YYYY-MM-DD, months YYYY-MM. Wallets, envelopes, payees and subscriptions may be named by id or by name (case-insensitive).
An error is {code, message} with code not_found, forbidden (a scope is missing), invalid or conflict.`

// invalidArguments gives the tool errors of the SDK itself, raised for
// arguments that do not match the input schema, the shape of the contract.
func invalidArguments(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
		result, err := next(ctx, method, req)
		if res, ok := result.(*sdk.CallToolResult); ok && res != nil && method == methodToolsCall && res.IsError && res.StructuredContent == nil {
			message := "the arguments do not match the input schema"
			if len(res.Content) > 0 {
				if text, ok := res.Content[0].(*sdk.TextContent); ok && text.Text != "" {
					message = text.Text
				}
			}
			return errorResult(codeInvalid, message), err
		}
		return result, err
	}
}

// errorResult builds the answer of a tool call that ended in an error: the
// object {code, message}, structured and as the same JSON in text.
func errorResult(code, message string) *sdk.CallToolResult {
	out := errorOut{Code: code, Message: message}
	text, _ := json.Marshal(out)
	return &sdk.CallToolResult{
		IsError:           true,
		StructuredContent: out,
		Content:           []sdk.Content{&sdk.TextContent{Text: string(text)}},
	}
}

// errInternal is what a client hears of a failure the contract has no code
// for; the detail goes to the log.
var errInternal = &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}

// errUnauthorized is what a call hears when its bot stopped being
// authorized while the call was under way.
var errUnauthorized = &jsonrpc.Error{Code: -32001, Message: "unauthorized"}

// addTool registers a tool. fn gets the calling bot, its household and the
// arguments, already checked against the schema derived from In, and
// returns what the tool answers with. An error the contract knows becomes
// the answer {code, message}; any other, a panic included, is logged and
// answered as an internal error of the protocol.
func addTool[In any](s *server, m *sdk.Server, name, description string, readOnly bool,
	fn func(ctx context.Context, p usecase.Principal, hid domain.ID, in In) (any, error)) {
	annotations := &sdk.ToolAnnotations{ReadOnlyHint: readOnly, OpenWorldHint: new(false)}
	if !readOnly {
		annotations.DestructiveHint = new(false)
	}
	sdk.AddTool(m, &sdk.Tool{Name: name, Description: description, Annotations: annotations},
		func(ctx context.Context, req *sdk.CallToolRequest, in In) (result *sdk.CallToolResult, out any, err error) {
			p, ok := principalOf(req)
			if !ok {
				s.logFailure(ctx, name, "", fmt.Errorf("no principal in the request"))
				return nil, nil, errInternal
			}
			ctx, cancel := context.WithTimeout(ctx, callTimeout)
			defer cancel()
			defer func() {
				if v := recover(); v != nil {
					s.logFailure(ctx, name, p.Bot.ID, fmt.Errorf("panic: %v\n%s", v, debug.Stack()))
					result, out, err = nil, nil, errInternal
				}
			}()

			out, err = fn(ctx, p, p.Bot.HouseholdID, in)
			if err == nil {
				return nil, out, nil
			}
			if errors.Is(err, domain.ErrUnauthenticated) {
				// The bot was disabled or its token replaced after this
				// request was authenticated. Its next request gets the 401.
				s.log.LogAttrs(ctx, slog.LevelInfo, "bot lost its token during a call",
					slog.String("tool", name), slog.String("bot_id", string(p.Bot.ID)))
				return nil, nil, errUnauthorized
			}
			code, message, known := toolError(err)
			if !known {
				s.logFailure(ctx, name, p.Bot.ID, err)
				return nil, nil, errInternal
			}
			return errorResult(code, message), nil, nil
		})
}

func (s *server) logFailure(ctx context.Context, tool string, botID domain.ID, err error) {
	s.log.LogAttrs(ctx, slog.LevelError, "tool failed",
		slog.String("tool", tool), slog.String("bot_id", string(botID)), slog.String("error", err.Error()))
}
