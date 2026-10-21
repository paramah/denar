package mcp

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
)

func TestToolError(t *testing.T) {
	for _, tc := range []struct {
		err           error
		code, message string
	}{
		{domain.ErrNotFound, codeNotFound, "not found"},
		{fmt.Errorf("wrapped: %w", domain.ErrNotFound), codeNotFound, "not found"},
		{usecase.RefNotFound{Kind: "wallet", Field: "from_wallet"}, codeNotFound, "from_wallet: no wallet you can see has this id or name"},
		{domain.ForbiddenError{Reason: domain.ForbiddenScope, Scope: "budget:write"}, codeForbidden, "the bot lacks the scope budget:write"},
		{domain.ForbiddenError{Reason: domain.ForbiddenRole, Scope: "budget:write"}, codeForbidden, "the bot lacks the scope budget:write"},
		{domain.ForbiddenError{Reason: domain.ForbiddenWallet}, codeForbidden, "the operation reaches a wallet the bot cannot see"},
		{domain.ForbiddenError{Reason: domain.ForbiddenHumanOnly}, codeForbidden, "only a person may do this"},
		{domain.ErrForbidden, codeForbidden, "not allowed"},
		{domain.ConflictError{Reason: domain.ConflictWalletArchived}, codeConflict, "the wallet is archived"},
		{domain.ConflictError{Reason: domain.ConflictChargeState}, codeConflict, "the charge's status does not allow this"},
		{domain.ErrConflict, codeConflict, "the change conflicts with the current state"},
		{errors.Join(domain.ValidationError{Field: "amount", Message: "must be greater than zero"},
			fmt.Errorf("x: %w", domain.ValidationError{Field: "date", Message: "must be a date"})), codeInvalid,
			"amount: must be greater than zero; date: must be a date"},
		{domain.ErrInvalid, codeInvalid, "the input is not valid"},
	} {
		code, message, known := toolError(tc.err)
		if !known || code != tc.code || !strings.HasPrefix(message, tc.message) {
			t.Errorf("%v: %q %q %v, want %q %q", tc.err, code, message, known, tc.code, tc.message)
		}
	}
	for _, err := range []error{errors.New("disk on fire"), domain.ErrUnauthenticated} {
		if _, _, known := toolError(err); known {
			t.Errorf("%v is known", err)
		}
	}
}
