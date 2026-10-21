package rest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
)

// maxBodyBytes caps a request body; no request of the contract comes near.
const maxBodyBytes = 1 << 20

// requestInfo is what is known about one request. logRequests puts it into
// the context; authentication, which runs further in, fills in the caller,
// for the operations and for the line written when the request is done.
type requestInfo struct {
	id     string
	caller caller
}

type requestInfoKey struct{}

// requestID returns the ID logRequests gave the request.
func requestID(ctx context.Context) string {
	if info, ok := ctx.Value(requestInfoKey{}).(*requestInfo); ok {
		return info.id
	}
	return ""
}

// statusRecorder remembers the status code a handler wrote.
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

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// logRequests gives each request an ID, returned in X-Request-Id, and logs
// one line when it is done, with the user or bot when there is one. The line
// carries the route pattern rather than the URL, and no headers, so codes,
// tokens and cookies stay out of the log.
func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var b [8]byte
		rand.Read(b[:])
		info := &requestInfo{id: hex.EncodeToString(b[:])}
		w.Header().Set("X-Request-Id", info.id)

		rec := &statusRecorder{ResponseWriter: w}
		// The mux records the matched pattern on the request it is given.
		r = r.WithContext(context.WithValue(r.Context(), requestInfoKey{}, info))
		next.ServeHTTP(rec, r)

		path := r.Pattern
		if path == "" {
			path = r.URL.Path
		}
		attrs := []slog.Attr{
			slog.String("request_id", info.id),
			slog.String("method", r.Method),
			slog.String("path", path),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
		}
		if id := info.caller.principal.User.ID; id != "" {
			attrs = append(attrs, slog.String("user_id", string(id)))
		}
		if id := info.caller.principal.Bot.ID; id != "" {
			attrs = append(attrs, slog.String("bot_id", string(id)))
		}
		log.LogAttrs(r.Context(), slog.LevelInfo, "request", attrs...)
	})
}

// noSniff tells browsers to take every answer for the type it declares.
func noSniff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// recoverPanics turns a panic in a handler into a 500 problem.
func recoverPanics(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				panic(v)
			}
			log.LogAttrs(r.Context(), slog.LevelError, "panic in handler",
				slog.String("request_id", requestID(r.Context())),
				slog.String("panic", fmt.Sprint(v)),
				slog.String("stack", string(debug.Stack())),
			)
			writeProblem(w, problemOf(fmt.Errorf("panic: %v", v)))
		}()
		next.ServeHTTP(w, r)
	})
}

// protectCrossOrigin rejects state-changing requests a browser sends from
// another origin (FR-014). The session cookie is SameSite=Lax and the
// contract has no CSRF token, so this check is the protection. Requests
// without Sec-Fetch-Site and Origin are not from a browser, as bots are,
// and pass.
func protectCrossOrigin(baseURL string, next http.Handler) http.Handler {
	protection := http.NewCrossOriginProtection()
	if baseURL != "" {
		// Behind a proxy the Host header may differ from the public origin.
		if err := protection.AddTrustedOrigin(baseURL); err != nil {
			panic(fmt.Sprintf("rest: base URL is not an origin: %v", err))
		}
	}
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeProblem(w, problemOf(domain.ErrForbidden))
	}))
	return protection.Handler(next)
}

// maxUploadBytes caps the multipart body of a CSV import: the 5 MiB file
// and its options (NFR-082).
const maxUploadBytes = 6 << 20

// acceptsUpload reports whether the request is one of the two operations
// that take multipart/form-data: the CSV import and its preview.
func acceptsUpload(r *http.Request) bool {
	if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/api/households/") {
		return false
	}
	return strings.HasSuffix(r.URL.Path, "/imports/csv") || strings.HasSuffix(r.URL.Path, "/imports/csv/preview")
}

// requireJSON lets a request body through only as application/json, which a
// cross-site form cannot send without a preflight, and caps its size. The
// CSV uploads take multipart/form-data instead; cross-site requests are
// stopped for them, as for every state-changing request, by
// protectCrossOrigin.
func requireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength != 0 {
			mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err == nil && mediaType == "multipart/form-data" && acceptsUpload(r) {
				r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
				next.ServeHTTP(w, r)
				return
			}
			if err != nil || mediaType != "application/json" {
				writeProblem(w, problemOf(domain.ValidationError{Field: "Content-Type", Message: "must be application/json"}))
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}
