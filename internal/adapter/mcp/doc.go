// Package mcp is the MCP adapter implementing api/mcp-tools.md: the tools
// of an assistant acting with a bot token, over Streamable HTTP, on the
// official github.com/modelcontextprotocol/go-sdk. Each tool checks
// nothing itself: it calls a use case and maps the answer. The rules
// (scopes, the creator's role, the wallet list, names, audit) stay in
// internal/usecase.
//
// What the adapter relies on in go-sdk v1.8.0 is confirmed against the
// SDK itself in sdk_probe_test.go (taken over from Straylight):
//
//  1. With Stateless and JSONResponse a tools/call needs no initialize; the
//     answer is application/json and carries no Mcp-Session-Id. GET and
//     DELETE answer 405. The Accept header must name both application/json
//     and text/event-stream (or be */*); application/json alone is a 400.
//  2. What the token verifier puts into TokenInfo.Extra reaches the tool
//     handler of a stateless server as req.Extra.TokenInfo.
//  3. auth.RequireBearerToken refuses a TokenInfo without an expiration
//     unless AllowMissingExpiration is set; a bot's expiry is checked by
//     the use case, so it is set. Its own 401 differs by cause and names
//     no scheme, which the contract forbids, so the adapter authenticates
//     first and answers a failure itself (auth.go); the SDK middleware
//     then only hands the principal on.
//  4. An error returned by a typed handler becomes isError with a text and
//     no structuredContent. A result built by the handler keeps its
//     structuredContent only when the tool has no output type, so the tools
//     are registered with the output type any, without an output schema.
//  5. Arguments that do not match the input schema (wrong type, unknown or
//     missing property) are answered by the SDK as isError with a text,
//     not as a protocol error; a receiving middleware gives them the code
//     "invalid" (server.go). An unknown tool is a JSON-RPC error.
//  6. MaxRequestBodyBytes answers a larger body with 413.
//  7. By default the SDK refuses a request that arrives over loopback with
//     another Host, which is exactly what a reverse proxy on the same
//     machine sends. That guard is against DNS rebinding of servers without
//     authentication; here every request carries a bearer token no browser
//     has, and cross-origin requests are refused, so the guard is turned
//     off (DisableLocalhostProtection).
package mcp
