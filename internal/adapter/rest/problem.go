package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"git.eiac.dev/tools/denar/internal/adapter/rest/api"
	"git.eiac.dev/tools/denar/internal/domain"
)

const problemTypePrefix = "urn:denar:problem:"

// errRateLimited is answered with 429; whoever returns it has set
// Retry-After.
var errRateLimited = errors.New("rate limited")

// problemOf maps an error to the problem the client sees. This is the only
// place that does so (FR-051). Apart from validation messages nothing of
// the error reaches the client, so a missing resource and a hidden one give
// the same body.
func problemOf(err error) api.Problem {
	switch {
	case errors.Is(err, errRateLimited):
		return problem(http.StatusTooManyRequests, api.ProblemCodeRateLimited, "rate-limited",
			"Too many requests", "Try again after the time in Retry-After.")
	case errors.Is(err, domain.ErrBadRequest):
		return problem(http.StatusBadRequest, api.ProblemCodeBadRequest, "bad-request",
			"Bad request", "The request is malformed.")
	case errors.Is(err, domain.ErrUnauthenticated):
		return problem(http.StatusUnauthorized, api.ProblemCodeUnauthenticated, "unauthenticated",
			"Unauthenticated", "Sign in or use a valid bot token.")
	case errors.Is(err, domain.ErrNotFound):
		return problem(http.StatusNotFound, api.ProblemCodeNotFound, "not-found",
			"Not found", "The resource does not exist.")
	case errors.Is(err, domain.ErrForbidden):
		slug := "forbidden"
		var refusal domain.ForbiddenError
		if errors.As(err, &refusal) {
			slug = strings.ReplaceAll(string(refusal.Reason), "_", "-")
		}
		return problem(http.StatusForbidden, api.ProblemCodeForbidden, slug,
			"Forbidden", "You are not allowed to do this.")
	case errors.Is(err, domain.ErrInvalid):
		p := problem(http.StatusUnprocessableEntity, api.ProblemCodeValidation, "validation",
			"Validation failed", "The request is not valid.")
		if fields := validationErrors(err); len(fields) > 0 {
			p.Errors = &fields
		}
		return p
	case errors.Is(err, domain.ErrConflict):
		slug := "conflict"
		var conflict domain.ConflictError
		if errors.As(err, &conflict) {
			slug = strings.ReplaceAll(string(conflict.Reason), "_", "-")
		}
		return problem(http.StatusConflict, api.ProblemCodeConflict, slug,
			"Conflict", "The request conflicts with the current state.")
	default:
		return problem(http.StatusInternalServerError, api.ProblemCodeInternal, "internal",
			"Internal error", "Something went wrong on the server.")
	}
}

func problem(status int, code api.ProblemCode, slug, title, detail string) api.Problem {
	return api.Problem{Type: problemTypePrefix + slug, Code: code, Status: status, Title: title, Detail: &detail}
}

// problemErrors is the type of api.Problem's Errors member.
type problemErrors = []struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// validationErrors collects every domain.ValidationError in err's tree.
func validationErrors(err error) problemErrors {
	switch e := err.(type) {
	case domain.ValidationError:
		return problemErrors{{Field: e.Field, Message: e.Message}}
	case interface{ Unwrap() []error }:
		var all problemErrors
		for _, child := range e.Unwrap() {
			all = append(all, validationErrors(child)...)
		}
		return all
	case interface{ Unwrap() error }:
		return validationErrors(e.Unwrap())
	}
	return nil
}

func writeProblem(w http.ResponseWriter, p api.Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(p.Status)
	json.NewEncoder(w).Encode(p)
}
