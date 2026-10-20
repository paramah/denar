package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestValidationErrorIsInvalid(t *testing.T) {
	err := fmt.Errorf("create: %w", ValidationError{Field: "name", Message: "must not be empty"})
	if !errors.Is(err, ErrInvalid) {
		t.Error("errors.Is(err, ErrInvalid) = false")
	}
	var ve ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" || ve.Error() != "name: must not be empty" {
		t.Errorf("errors.As gave %+v", ve)
	}
	joined := errors.Join(ValidationError{Field: "a"}, ValidationError{Field: "b"})
	if !errors.Is(joined, ErrInvalid) {
		t.Error("joined validation errors are not ErrInvalid")
	}
}

func TestTypedErrors(t *testing.T) {
	c := fmt.Errorf("x: %w", ConflictError{Reason: ConflictLastOwner})
	if !errors.Is(c, ErrConflict) || errors.Is(c, ErrInvalid) {
		t.Error("ConflictError does not match ErrConflict only")
	}
	f := fmt.Errorf("x: %w", ForbiddenError{Reason: ForbiddenHumanOnly})
	if !errors.Is(f, ErrForbidden) || errors.Is(f, ErrConflict) {
		t.Error("ForbiddenError does not match ErrForbidden only")
	}
	var fe ForbiddenError
	if !errors.As(f, &fe) || fe.Reason != ForbiddenHumanOnly {
		t.Errorf("errors.As gave %+v", fe)
	}
}

func TestActor(t *testing.T) {
	if !(Actor{}).IsZero() || (Actor{Kind: ActorBot, ID: "b"}).IsZero() {
		t.Error("IsZero wrong")
	}
	for _, k := range []ActorKind{ActorUser, ActorBot, ActorSystem} {
		if !k.Valid() {
			t.Errorf("%q not valid", k)
		}
	}
	if ActorKind("agent").Valid() {
		t.Error("unknown kind valid")
	}
}

func TestSome(t *testing.T) {
	var unset Optional[string]
	if unset.Set {
		t.Error("zero Optional is set")
	}
	if got := Some("x"); !got.Set || got.Value != "x" {
		t.Errorf("Some = %+v", got)
	}
}
