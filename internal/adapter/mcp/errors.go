package mcp

import (
	"errors"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
)

// The error codes of api/mcp-tools.md.
const (
	codeNotFound  = "not_found"
	codeForbidden = "forbidden"
	codeConflict  = "conflict"
	codeInvalid   = "invalid"
)

// errorOut is the structured content of a tool call that ended in an error.
type errorOut struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// The words for a refusal or conflict with a known reason.
var (
	forbiddenMessages = map[domain.ForbiddenReason]string{
		domain.ForbiddenHumanOnly: "only a person may do this, in the web panel",
		domain.ForbiddenWallet:    "the operation reaches a wallet the bot cannot see",
	}
	conflictMessages = map[domain.ConflictReason]string{
		domain.ConflictDuplicateName:       "the name is already used in this household",
		domain.ConflictDuplicateExternalID: "the wallet already has a transaction with this external id",
		domain.ConflictWalletArchived:      "the wallet is archived and takes no new entries",
		domain.ConflictDuplicateCharge:     "the subscription already has a charge on that day",
		domain.ConflictChargeState:         "the charge's status does not allow this (it is already matched, posted or skipped)",
		domain.ConflictTransactionLinked:   "the transaction already pays another charge",
		domain.ConflictEnvelopeInUse:       "the envelope is in use",
		domain.ConflictGroupNotEmpty:       "the envelope group still has envelopes",
	}
)

// toolError maps an error of a use case to the code and the message a tool
// answers with. known is false for an error the contract has no code for;
// that one is a failure of the server, not of the call.
func toolError(err error) (code, message string, known bool) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		var ref usecase.RefNotFound
		if errors.As(err, &ref) {
			return codeNotFound, ref.Field + ": no " + ref.Kind + " you can see has this id or name", true
		}
		// One message, so that what is hidden and what is missing look alike.
		return codeNotFound, "not found", true
	case errors.Is(err, domain.ErrForbidden):
		var reason domain.ForbiddenError
		if errors.As(err, &reason) {
			if reason.Scope != "" && (reason.Reason == domain.ForbiddenScope || reason.Reason == domain.ForbiddenRole) {
				return codeForbidden, "the bot lacks the scope " + reason.Scope +
					" (or its creator's role does not allow it); ask the owner of the household to grant it", true
			}
			if m := forbiddenMessages[reason.Reason]; m != "" {
				return codeForbidden, m, true
			}
		}
		return codeForbidden, "not allowed", true
	case errors.Is(err, domain.ErrConflict):
		var reason domain.ConflictError
		if errors.As(err, &reason) && conflictMessages[reason.Reason] != "" {
			return codeConflict, conflictMessages[reason.Reason], true
		}
		return codeConflict, "the change conflicts with the current state", true
	case errors.Is(err, domain.ErrInvalid), errors.Is(err, domain.ErrBadRequest):
		fields := validationErrors(err)
		if len(fields) == 0 {
			return codeInvalid, "the input is not valid", true
		}
		parts := make([]string, len(fields))
		for i, field := range fields {
			parts[i] = field.Error()
		}
		return codeInvalid, strings.Join(parts, "; "), true
	}
	return "", "", false
}

// validationErrors returns every ValidationError in err, however the use
// case joined and wrapped them, in order.
func validationErrors(err error) []domain.ValidationError {
	var found []domain.ValidationError
	var walk func(error)
	walk = func(err error) {
		if err == nil {
			return
		}
		if field, ok := err.(domain.ValidationError); ok {
			found = append(found, field)
			return
		}
		switch wrapped := err.(type) {
		case interface{ Unwrap() []error }:
			for _, inner := range wrapped.Unwrap() {
				walk(inner)
			}
		case interface{ Unwrap() error }:
			walk(wrapped.Unwrap())
		}
	}
	walk(err)
	return found
}
