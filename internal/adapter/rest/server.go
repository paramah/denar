// Package rest is the HTTP adapter implementing api/openapi.yaml.
package rest

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"git.eiac.dev/tools/denar/internal/adapter/rest/api"
	"git.eiac.dev/tools/denar/internal/domain"
)

// Server implements the operations of api/openapi.yaml. An operation
// returns its typed response, or an error that problemOf maps.
type Server struct {
	deps Deps
}

var _ api.StrictServerInterface = (*Server)(nil)

func NewServer(deps Deps) *Server {
	return &Server{deps: deps}
}

// Handler returns the HTTP handler serving the API under /api and /healthz.
func (s *Server) Handler() http.Handler {
	return newHandler(s, s.deps)
}

// newHandler puts the middleware chain in front of the generated router:
// nosniff, request ID and log, panic recovery, cross-origin check,
// authentication (session or bot token), JSON bodies only, then per
// operation the rate limits of the login and the public operations.
func newHandler(ssi api.StrictServerInterface, deps Deps) http.Handler {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	fail := func(w http.ResponseWriter, r *http.Request, err error) {
		p := problemOf(err)
		if p.Status >= http.StatusInternalServerError {
			log.LogAttrs(r.Context(), slog.LevelError, "request failed",
				slog.String("request_id", requestID(r.Context())),
				slog.String("error", err.Error()),
			)
		}
		writeProblem(w, p)
	}

	clock := deps.Clock
	if clock == nil {
		clock = wallClock{}
	}
	lim := newLimits(clock, deps.TrustedProxies, deps.BotRate)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz(deps.Health))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		fail(w, r, domain.ErrNotFound)
	})
	strict := api.NewStrictHandlerWithOptions(ssi,
		[]api.StrictMiddlewareFunc{lim.middleware},
		api.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				fail(w, r, bodyError(err))
			},
			ResponseErrorHandlerFunc: fail,
		})
	api.HandlerWithOptions(strict, api.StdHTTPServerOptions{
		BaseURL:    "/api",
		BaseRouter: mux,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			fail(w, r, paramError(err))
		},
	})

	var h http.Handler = mux
	h = requireJSON(h)
	h = authenticate(deps, lim, mux, fail, h)
	h = protectCrossOrigin(deps.BaseURL, h)
	h = recoverPanics(log, h)
	return noSniff(logRequests(log, h))
}

// bodyError turns a failure to decode a JSON body into a validation error.
func bodyError(err error) error {
	var tooLarge *http.MaxBytesError
	var wrongType *json.UnmarshalTypeError
	switch {
	case errors.As(err, &tooLarge):
		return domain.ValidationError{Field: "body", Message: "is too large"}
	case strings.Contains(err.Error(), "multipart"):
		return domain.ValidationError{Field: "body", Message: "must be multipart/form-data with the parts file and options"}
	case errors.As(err, &wrongType) && wrongType.Field != "":
		return domain.ValidationError{Field: wrongType.Field, Message: "has the wrong type"}
	}
	return domain.ValidationError{Field: "body", Message: "must be valid JSON"}
}

// paramError turns a failure to read a path or query parameter into a
// validation error naming the parameter.
func paramError(err error) error {
	var (
		format    *api.InvalidParamFormatError
		required  *api.RequiredParamError
		unmarshal *api.UnmarshalingParamError
		tooMany   *api.TooManyValuesForParamError
	)
	switch {
	case errors.As(err, &format):
		return domain.ValidationError{Field: format.ParamName, Message: "has the wrong format"}
	case errors.As(err, &required):
		return domain.ValidationError{Field: required.ParamName, Message: "is required"}
	case errors.As(err, &unmarshal):
		return domain.ValidationError{Field: unmarshal.ParamName, Message: "has the wrong format"}
	case errors.As(err, &tooMany):
		return domain.ValidationError{Field: tooMany.ParamName, Message: "must be given once"}
	}
	return domain.ErrInvalid
}

// healthz answers 200 while the database answers, 503 otherwise (US-001).
func healthz(check func(ctx context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if check != nil {
			if err := check(r.Context()); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(`{"status":"unavailable"}`))
				return
			}
		}
		w.Write([]byte(`{"status":"ok"}`))
	}
}
