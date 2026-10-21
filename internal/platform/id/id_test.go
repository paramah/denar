package id

import (
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
)

var _ usecase.IDGenerator = (*Generator)(nil)

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func TestNewIDFormat(t *testing.T) {
	got := string(New().NewID())

	if len(got) != 26 {
		t.Fatalf("NewID() = %q, want 26 characters", got)
	}
	if i := strings.IndexFunc(got, func(r rune) bool { return !strings.ContainsRune(crockford, r) }); i >= 0 {
		t.Errorf("NewID() = %q, character %d is not Crockford base32", got, i)
	}
	if got[0] > '7' {
		t.Errorf("NewID() = %q, first character overflows 128 bits", got)
	}
}

func TestNewIDEncodesTimestamp(t *testing.T) {
	g := New()
	g.now = func() time.Time { return time.UnixMilli(1469918176385) }

	// Timestamp prefix of the example in the ULID specification.
	if got := string(g.NewID())[:10]; got != "01ARYZ6S41" {
		t.Errorf("timestamp prefix = %q, want 01ARYZ6S41", got)
	}
}

func TestNewIDSortsByTime(t *testing.T) {
	g := New()
	now := time.UnixMilli(1_700_000_000_000)
	g.now = func() time.Time { return now }

	var prev domain.ID
	for i := range 100 {
		now = now.Add(time.Millisecond)
		got := g.NewID()
		if got <= prev {
			t.Fatalf("id %d = %q, not after %q", i, got, prev)
		}
		prev = got
	}
}

func TestNewIDMonotonicWithinMillisecond(t *testing.T) {
	g := New()
	g.now = func() time.Time { return time.UnixMilli(1_700_000_000_000) }

	prev := g.NewID()
	for i := range 1000 {
		got := g.NewID()
		if got <= prev {
			t.Fatalf("id %d = %q, not after %q", i, got, prev)
		}
		if got[:10] != prev[:10] {
			t.Fatalf("id %d = %q, timestamp changed from %q", i, got, prev)
		}
		prev = got
	}
}

func TestNewIDClockGoingBackwards(t *testing.T) {
	g := New()
	now := time.UnixMilli(1_700_000_000_000)
	g.now = func() time.Time { return now }

	first := g.NewID()
	now = now.Add(-time.Second)
	if second := g.NewID(); second <= first {
		t.Errorf("id after clock step back = %q, not after %q", second, first)
	}
}

func TestNewIDRandomOverflowAdvancesTimestamp(t *testing.T) {
	g := New()
	g.now = func() time.Time { return time.UnixMilli(1_700_000_000_000) }
	first := g.NewID()
	for i := range g.entropy {
		g.entropy[i] = 0xff
	}

	second := g.NewID()
	if second <= first {
		t.Errorf("id after overflow = %q, not after %q", second, first)
	}
	if second[:10] == first[:10] {
		t.Errorf("id after overflow = %q, want a later timestamp than %q", second, first)
	}
}

func TestNewIDConcurrentUnique(t *testing.T) {
	const n = 10_000
	g := New()
	ids := make([]string, n)

	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { ids[i] = string(g.NewID()) })
	}
	wg.Wait()

	sort.Strings(ids)
	for i := 1; i < n; i++ {
		if ids[i] == ids[i-1] {
			t.Fatalf("duplicate id %q", ids[i])
		}
	}
}
