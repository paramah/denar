package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
)

// requestInfo is what the request log learns while a request is handled.
type requestInfo struct {
	botID domain.ID
}

type requestInfoKey struct{}

// noteBot tells the request log which bot a request belongs to.
func noteBot(ctx context.Context, id domain.ID) {
	if info, ok := ctx.Value(requestInfoKey{}).(*requestInfo); ok {
		info.botID = id
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(p)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// routePattern is what the request log names as the path: the route the
// composition root mounts the handler at.
const routePattern = "/mcp"

// logRequests logs one line per request, with the bot once it is known.
// The line names the route, not the address the client asked for, and no
// headers: a token never reaches the log, not even one put into the path
// or the query by mistake.
func (s *server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		info := &requestInfo{}
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), requestInfoKey{}, info)))

		attrs := []slog.Attr{
			slog.String("method", r.Method),
			slog.String("path", routePattern),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
		}
		if info.botID != "" {
			attrs = append(attrs, slog.String("bot_id", string(info.botID)))
		}
		s.log.LogAttrs(r.Context(), slog.LevelInfo, "mcp request", attrs...)
	})
}

// recoverPanics answers a panic of the HTTP layer with a bare 500 and logs
// it. A panic inside a tool is caught where the tool runs (addTool).
func (s *server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				panic(v)
			}
			s.log.LogAttrs(r.Context(), slog.LevelError, "panic in handler",
				slog.String("panic", fmt.Sprint(v)), slog.String("stack", string(debug.Stack())))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}

// noSniff tells browsers to take every answer for the type it declares.
func noSniff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// postOnly refuses every other method before anything else looks at the
// request. The server keeps no sessions, so there is no stream to open with
// GET and nothing to end with DELETE; and with OPTIONS refused and no CORS
// header anywhere, a preflight from a browser fails.
func postOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// singleMessage refuses a JSON-RPC batch. The limits of a bot count
// requests, and a batch would run any number of tool calls as one. Only the
// first byte that is not white space is looked at; the body goes on unread.
func singleMessage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// One byte over the limit of the SDK, so that a body of nothing but
		// white space still ends in its 413.
		body := bufio.NewReader(http.MaxBytesReader(w, r.Body, maxRequestBody+1))
		for {
			c, err := body.ReadByte()
			if err != nil {
				break
			}
			if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
				continue
			}
			if c == '[' {
				http.Error(w, "a request carries one JSON-RPC message, not a batch", http.StatusBadRequest)
				return
			}
			_ = body.UnreadByte()
			break
		}
		// The white space read so far is not part of the message.
		r.Body = struct {
			io.Reader
			io.Closer
		}{body, r.Body}
		next.ServeHTTP(w, r)
	})
}
