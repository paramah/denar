package clock

import (
	"testing"
	"time"

	"github.com/paramah/denar/internal/usecase"
)

var _ usecase.Clock = System{}

func TestSystemNow(t *testing.T) {
	before := time.Now().Add(-time.Millisecond)
	got := System{}.Now()
	after := time.Now().Add(time.Millisecond)

	if got.Before(before) || got.After(after) {
		t.Errorf("Now() = %v, want between %v and %v", got, before, after)
	}
	if got.Location() != time.UTC {
		t.Errorf("Now() location = %v, want UTC", got.Location())
	}
	if !got.Equal(got.Truncate(time.Microsecond)) {
		t.Errorf("Now() = %v, want microsecond precision", got)
	}
}
