package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/auth"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// These tests pin down what the adapter relies on in go-sdk v1.8.0
// (design §2 of 006-mcp, as found in Straylight). They use the SDK alone, so a change of its
// behaviour in an upgrade shows here first. The findings are summed up in
// the package comment.

type probeIn struct {
	Name string `json:"name" jsonschema:"who to greet"`
}

type probeOut struct {
	Greeting string `json:"greeting"`
}

// probeServer has one typed tool per way of answering.
func probeServer() *sdk.Server {
	s := sdk.NewServer(&sdk.Implementation{Name: "probe", Version: "0"}, nil)
	sdk.AddTool(s, &sdk.Tool{Name: "typed"}, func(_ context.Context, _ *sdk.CallToolRequest, in probeIn) (*sdk.CallToolResult, probeOut, error) {
		if in.Name == "fail" {
			return nil, probeOut{}, errors.New("plain failure")
		}
		return nil, probeOut{Greeting: "hi " + in.Name}, nil
	})
	sdk.AddTool(s, &sdk.Tool{Name: "untyped"}, func(_ context.Context, req *sdk.CallToolRequest, in probeIn) (*sdk.CallToolResult, any, error) {
		switch in.Name {
		case "fail":
			return &sdk.CallToolResult{
				IsError:           true,
				StructuredContent: map[string]string{"code": "forbidden", "message": "no"},
				Content:           []sdk.Content{&sdk.TextContent{Text: `{"code":"forbidden","message":"no"}`}},
			}, nil, nil
		case "whoami":
			who, _ := req.Extra.TokenInfo.Extra["who"].(string)
			return nil, probeOut{Greeting: who}, nil
		}
		return nil, probeOut{Greeting: "hi " + in.Name}, nil
	})
	return s
}

func probeHandler(opts *sdk.StreamableHTTPOptions) http.Handler {
	s := probeServer()
	return sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return s }, opts)
}

var statelessJSON = sdk.StreamableHTTPOptions{Stateless: true, JSONResponse: true}

type probeReply struct {
	status int
	header http.Header
	body   string
	// The JSON-RPC answer, when the body is one.
	rpc struct {
		Result struct {
			IsError           bool            `json:"isError"`
			StructuredContent json.RawMessage `json:"structuredContent"`
			Content           []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
}

func callBody(tool, arguments string) string {
	return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `","arguments":` + arguments + `}}`
}

// post sends one JSON-RPC message the way a client without the SDK would.
func post(t *testing.T, h http.Handler, body string, edit func(*http.Request)) probeReply {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "http://denar.test/mcp", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	if edit != nil {
		edit(r)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	reply := probeReply{status: w.Code, header: w.Header(), body: w.Body.String()}
	_ = json.Unmarshal(w.Body.Bytes(), &reply.rpc)
	return reply
}

// Point 1: a stateless server answers tools/call without initialize, with
// plain JSON and without a session.
func TestProbeStatelessCallWithoutInitialize(t *testing.T) {
	h := probeHandler(&statelessJSON)

	reply := post(t, h, callBody("typed", `{"name":"ada"}`), nil)

	if reply.status != http.StatusOK {
		t.Fatalf("status = %d, body %q", reply.status, reply.body)
	}
	if got := reply.header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if got := reply.header.Get("Mcp-Session-Id"); got != "" {
		t.Errorf("Mcp-Session-Id = %q, want none", got)
	}
	if got := string(reply.rpc.Result.StructuredContent); got != `{"greeting":"hi ada"}` {
		t.Errorf("structuredContent = %s", got)
	}
	// The same JSON as text.
	if len(reply.rpc.Result.Content) != 1 || reply.rpc.Result.Content[0].Text != `{"greeting":"hi ada"}` {
		t.Errorf("content = %+v", reply.rpc.Result.Content)
	}
}

// Point 1: which Accept headers a client must send.
func TestProbeAcceptHeader(t *testing.T) {
	h := probeHandler(&statelessJSON)
	for accept, want := range map[string]int{
		"application/json, text/event-stream": http.StatusOK,
		"application/json":                    http.StatusBadRequest,
		"*/*":                                 http.StatusOK,
		"text/event-stream":                   http.StatusBadRequest,
		"text/html":                           http.StatusBadRequest,
	} {
		reply := post(t, h, callBody("typed", `{"name":"ada"}`), func(r *http.Request) { r.Header.Set("Accept", accept) })
		if reply.status != want {
			t.Errorf("Accept %q: status = %d, want %d (%s)", accept, reply.status, want, reply.body)
		}
	}
}

// Point 1: a stateless server has no stream to open and no session to end.
func TestProbeStatelessRefusesGetAndDelete(t *testing.T) {
	h := probeHandler(&statelessJSON)
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		r := httptest.NewRequest(method, "http://denar.test/mcp", nil)
		r.Header.Set("Accept", "application/json, text/event-stream")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, want 405", method, w.Code)
		}
	}
}

// Points 2 and 3: what the verifier puts into TokenInfo.Extra reaches the
// tool handler of a stateless server, and a token without an expiration
// passes only with AllowMissingExpiration.
func TestProbeTokenInfoReachesTheHandler(t *testing.T) {
	verifier := func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		if token != "good" {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{UserID: "agent-1", Extra: map[string]any{"who": "planner"}}, nil
	}
	bearer := func(token string) func(*http.Request) {
		return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
	}

	strict := auth.RequireBearerToken(verifier, nil)(probeHandler(&statelessJSON))
	if reply := post(t, strict, callBody("untyped", `{"name":"whoami"}`), bearer("good")); reply.status != http.StatusUnauthorized {
		t.Errorf("without AllowMissingExpiration: status = %d, want 401", reply.status)
	}

	lenient := auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(probeHandler(&statelessJSON))
	reply := post(t, lenient, callBody("untyped", `{"name":"whoami"}`), bearer("good"))
	if reply.status != http.StatusOK || string(reply.rpc.Result.StructuredContent) != `{"greeting":"planner"}` {
		t.Errorf("with AllowMissingExpiration: status = %d, body %s", reply.status, reply.body)
	}
}

// The 401 of the SDK's middleware differs by cause and names no scheme, so
// the adapter answers failed authentication itself.
func TestProbeSDKUnauthorizedAnswersDiffer(t *testing.T) {
	verifier := func(context.Context, string, *http.Request) (*auth.TokenInfo, error) {
		return nil, auth.ErrInvalidToken
	}
	h := auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(probeHandler(&statelessJSON))

	missing := post(t, h, callBody("typed", `{}`), nil)
	wrong := post(t, h, callBody("typed", `{}`), func(r *http.Request) { r.Header.Set("Authorization", "Bearer nope") })

	if missing.status != http.StatusUnauthorized || wrong.status != http.StatusUnauthorized {
		t.Fatalf("statuses = %d, %d, want 401 twice", missing.status, wrong.status)
	}
	if missing.body == wrong.body {
		t.Errorf("the bodies are the same now (%q); the adapter's own 401 may no longer be needed", missing.body)
	}
	if got := wrong.header.Get("WWW-Authenticate"); got != "" {
		t.Errorf("WWW-Authenticate = %q; the SDK sets it now", got)
	}
}

// Point 4: an error returned by a typed handler becomes isError with the
// text only. A result built by the handler keeps its structuredContent, as
// long as the tool has no output type to validate it against.
func TestProbeToolErrors(t *testing.T) {
	h := probeHandler(&statelessJSON)

	plain := post(t, h, callBody("typed", `{"name":"fail"}`), nil)
	if !plain.rpc.Result.IsError || plain.rpc.Result.StructuredContent != nil || plain.rpc.Result.Content[0].Text != "plain failure" {
		t.Errorf("returned error: %s", plain.body)
	}

	shaped := post(t, h, callBody("untyped", `{"name":"fail"}`), nil)
	if !shaped.rpc.Result.IsError || string(shaped.rpc.Result.StructuredContent) != `{"code":"forbidden","message":"no"}` {
		t.Errorf("built result: %s", shaped.body)
	}
	if shaped.rpc.Result.Content[0].Text != `{"code":"forbidden","message":"no"}` {
		t.Errorf("text of the built result: %s", shaped.body)
	}

	// Without an output type the success still carries both forms.
	ok := post(t, h, callBody("untyped", `{"name":"ada"}`), nil)
	if string(ok.rpc.Result.StructuredContent) != `{"greeting":"hi ada"}` || ok.rpc.Result.Content[0].Text != `{"greeting":"hi ada"}` {
		t.Errorf("success of an untyped tool: %s", ok.body)
	}
}

// Point 5: arguments that do not match the input schema are a tool error
// with a text, not a protocol error; an unknown tool is a protocol error.
func TestProbeBadArgumentsAndUnknownTool(t *testing.T) {
	h := probeHandler(&statelessJSON)

	for name, arguments := range map[string]string{
		"wrong type":       `{"name":7}`,
		"unknown property": `{"name":"ada","extra":true}`,
		"missing required": `{}`,
	} {
		reply := post(t, h, callBody("typed", arguments), nil)
		if reply.rpc.Error != nil || !reply.rpc.Result.IsError || reply.rpc.Result.StructuredContent != nil {
			t.Errorf("%s: want isError without structuredContent, got %s", name, reply.body)
		}
	}

	unknown := post(t, h, callBody("nope", `{}`), nil)
	if unknown.rpc.Error == nil {
		t.Errorf("unknown tool: want a JSON-RPC error, got %s", unknown.body)
	}
}

// Point 6: a body over the limit is refused with 413.
func TestProbeBodyLimit(t *testing.T) {
	opts := statelessJSON
	opts.MaxRequestBodyBytes = 1 << 20
	h := probeHandler(&opts)

	fits := post(t, h, callBody("typed", `{"name":"`+strings.Repeat("a", 1<<19)+`"}`), nil)
	if fits.status != http.StatusOK {
		t.Errorf("half the limit: status = %d", fits.status)
	}
	tooBig := post(t, h, callBody("typed", `{"name":"`+strings.Repeat("a", 1<<20)+`"}`), nil)
	if tooBig.status != http.StatusRequestEntityTooLarge {
		t.Errorf("over the limit: status = %d, want 413", tooBig.status)
	}
}

// Point 7: by default a request that arrives over loopback with another
// Host is refused. A reverse proxy on the same machine sends exactly that.
func TestProbeLocalhostProtection(t *testing.T) {
	viaProxy := func(r *http.Request) {
		r.RemoteAddr = "127.0.0.1:40000"
		r.Host = "denar.example.com"
	}
	local := httptest.NewServer(probeHandler(&statelessJSON))
	defer local.Close()

	send := func(h http.Handler) int {
		srv := httptest.NewServer(h)
		defer srv.Close()
		req, err := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader([]byte(callBody("typed", `{"name":"ada"}`))))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		viaProxy(req)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		_, _ = io.Copy(io.Discard, res.Body)
		return res.StatusCode
	}

	if got := send(probeHandler(&statelessJSON)); got != http.StatusForbidden {
		t.Errorf("default: status = %d, want 403", got)
	}
	open := statelessJSON
	open.DisableLocalhostProtection = true
	if got := send(probeHandler(&open)); got != http.StatusOK {
		t.Errorf("DisableLocalhostProtection: status = %d, want 200", got)
	}
}

// The SDK's own client works against a stateless JSON server, with the
// token added by the HTTP client it is given.
func TestProbeSDKClient(t *testing.T) {
	verifier := func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		if token != "good" {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{Extra: map[string]any{"who": "planner"}}, nil
	}
	opts := statelessJSON
	opts.DisableLocalhostProtection = true
	srv := httptest.NewServer(auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(probeHandler(&opts)))
	defer srv.Close()

	client := sdk.NewClient(&sdk.Implementation{Name: "probe-client", Version: "0"}, nil)
	session, err := client.Connect(context.Background(), &sdk.StreamableClientTransport{
		Endpoint:   srv.URL,
		HTTPClient: &http.Client{Transport: bearerTransport{token: "good"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != 2 {
		t.Fatalf("ListTools = %v, %v", tools, err)
	}
	res, err := session.CallTool(context.Background(), &sdk.CallToolParams{Name: "untyped", Arguments: map[string]any{"name": "whoami"}})
	if err != nil {
		t.Fatal(err)
	}
	if structured, _ := json.Marshal(res.StructuredContent); string(structured) != `{"greeting":"planner"}` {
		t.Errorf("structuredContent = %s", structured)
	}
}

// bearerTransport adds the agent token to every request of a test client.
type bearerTransport struct {
	token string
}

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if b.token != "" {
		r.Header.Set("Authorization", "Bearer "+b.token)
	}
	return http.DefaultTransport.RoundTrip(r)
}
