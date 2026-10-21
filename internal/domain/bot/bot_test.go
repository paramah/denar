package bot

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/membership"
)

func TestParseScopes(t *testing.T) {
	got, err := ParseScopes("scopes", []string{"transactions:write", "wallets:read", "transactions:write"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []Scope{ScopeWalletsRead, ScopeTransactionsWrite}; !reflect.DeepEqual(got, want) {
		t.Errorf("ParseScopes = %v, want %v", got, want)
	}
	for _, bad := range [][]string{nil, {}, {"wallets:delete"}, {"*"}, {"wallets:read", ""}} {
		_, err := ParseScopes("scopes", bad)
		var ve domain.ValidationError
		if !errors.As(err, &ve) || ve.Field != "scopes" {
			t.Errorf("ParseScopes(%q) = %v", bad, err)
		}
	}
	if len(Scopes) != 10 {
		t.Errorf("FR-031 lists 10 scopes, have %d", len(Scopes))
	}
}

func TestWriteDoesNotImplyRead(t *testing.T) {
	b := Bot{Scopes: []Scope{ScopeTransactionsWrite}}
	for _, s := range b.EffectiveScopes(membership.RoleOwner) {
		if s == ScopeTransactionsRead {
			t.Error("write implied read")
		}
	}
}

func TestRolePermits(t *testing.T) {
	for _, s := range Scopes {
		for _, r := range membership.Roles {
			want := s.IsRead() || r != membership.RoleViewer
			if got := RolePermits(r, s); got != want {
				t.Errorf("RolePermits(%s, %s) = %v", r, s, got)
			}
		}
		if RolePermits("", s) {
			t.Errorf("no role permits %s", s)
		}
	}
	if RolePermits(membership.RoleOwner, "admin:all") {
		t.Error("unknown scope permitted")
	}
}

func TestEffectiveScopes(t *testing.T) {
	b := Bot{Scopes: []Scope{ScopeWalletsRead, ScopeTransactionsWrite, ScopeReportsRead}}
	tests := []struct {
		role membership.Role
		want []Scope
	}{
		{membership.RoleOwner, []Scope{ScopeWalletsRead, ScopeTransactionsWrite, ScopeReportsRead}},
		{membership.RoleMember, []Scope{ScopeWalletsRead, ScopeTransactionsWrite, ScopeReportsRead}},
		{membership.RoleViewer, []Scope{ScopeWalletsRead, ScopeReportsRead}},
		{"", []Scope{}},
	}
	for _, tt := range tests {
		if got := b.EffectiveScopes(tt.role); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("EffectiveScopes(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestUsable(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		b    Bot
		want bool
	}{
		{Bot{}, true},
		{Bot{Disabled: true}, false},
		{Bot{ExpiresAt: now.Add(time.Second)}, true},
		{Bot{ExpiresAt: now}, false},
		{Bot{ExpiresAt: now.Add(-time.Hour)}, false},
	}
	for i, tt := range tests {
		if got := tt.b.Usable(now); got != tt.want {
			t.Errorf("%d: Usable = %v", i, got)
		}
	}
}

func TestParseWalletIDs(t *testing.T) {
	got, err := ParseWalletIDs("wallet_ids", []string{"w1", " w2 ", "w1"})
	if err != nil || !reflect.DeepEqual(got, []domain.ID{"w1", "w2"}) {
		t.Errorf("got %v, %v", got, err)
	}
	if got, err := ParseWalletIDs("wallet_ids", nil); err != nil || len(got) != 0 {
		t.Errorf("empty: %v %v", got, err)
	}
	many := make([]string, 101)
	for i := range many {
		many[i] = strings.Repeat("w", 1) + string(rune('A'+i%26)) + string(rune('a'+i/26))
	}
	for _, bad := range [][]string{{""}, {"\x00"}, {strings.Repeat("x", 65)}, many} {
		if _, err := ParseWalletIDs("wallet_ids", bad); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("ParseWalletIDs(%d items) = %v", len(bad), err)
		}
	}
}

func TestValidate(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	if err := (Bot{Name: "Claude"}).Validate(now); err != nil {
		t.Error(err)
	}
	if err := (Bot{Name: "", ExpiresAt: now}).Validate(now); !errors.Is(err, domain.ErrInvalid) ||
		!strings.Contains(err.Error(), "name") || !strings.Contains(err.Error(), "expires_at") {
		t.Errorf("err = %v", err)
	}
}
