package membership

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain"
)

func TestRoleOrder(t *testing.T) {
	tests := []struct {
		r, min Role
		want   bool
	}{
		{RoleOwner, RoleAdmin, true}, {RoleAdmin, RoleAdmin, true}, {RoleMember, RoleAdmin, false},
		{RoleViewer, RoleMember, false}, {RoleViewer, RoleViewer, true}, {Role("x"), RoleViewer, false},
		{Role(""), RoleViewer, false},
	}
	for _, tt := range tests {
		if got := tt.r.AtLeast(tt.min); got != tt.want {
			t.Errorf("%q.AtLeast(%q) = %v", tt.r, tt.min, got)
		}
	}
	if !RoleMember.CanWrite() || RoleViewer.CanWrite() || !RoleAdmin.CanManage() || RoleMember.CanManage() {
		t.Error("CanWrite/CanManage wrong")
	}
	if ValidateRole("role", "boss") == nil || ValidateRole("role", RoleViewer) != nil {
		t.Error("ValidateRole wrong")
	}
}

func TestCheckRoleChange(t *testing.T) {
	for _, actor := range Roles {
		for _, old := range Roles {
			for _, new := range Roles {
				want := actor == RoleOwner || (actor == RoleAdmin && old != RoleOwner && new != RoleOwner)
				err := CheckRoleChange(actor, old, new)
				if (err == nil) != want {
					t.Errorf("CheckRoleChange(%s, %s, %s) = %v, want allowed %v", actor, old, new, err, want)
				}
				if err != nil && !errors.Is(err, domain.ErrForbidden) {
					t.Errorf("error %v is not forbidden", err)
				}
			}
		}
	}
}

func TestCheckRemoval(t *testing.T) {
	for _, actor := range Roles {
		for _, target := range Roles {
			want := actor == RoleOwner || (actor == RoleAdmin && target != RoleOwner)
			if err := CheckRemoval("a", actor, "b", target); (err == nil) != want {
				t.Errorf("CheckRemoval(%s → %s) = %v, want allowed %v", actor, target, err, want)
			}
			if err := CheckRemoval("a", actor, "a", actor); err != nil {
				t.Errorf("leaving as %s = %v", actor, err)
			}
		}
	}
}

func TestCheckInvite(t *testing.T) {
	for _, actor := range Roles {
		for _, invited := range Roles {
			want := actor == RoleOwner || (actor == RoleAdmin && invited != RoleOwner)
			if err := CheckInvite(actor, invited); (err == nil) != want {
				t.Errorf("CheckInvite(%s, %s) = %v", actor, invited, err)
			}
		}
	}
}

func TestCheckLastOwner(t *testing.T) {
	m := func(id domain.ID, r Role) Membership { return Membership{UserID: id, Role: r} }
	one := []Membership{m("o", RoleOwner), m("a", RoleAdmin)}
	two := []Membership{m("o", RoleOwner), m("p", RoleOwner)}
	tests := []struct {
		members  []Membership
		target   domain.ID
		role     Role
		removed  bool
		conflict bool
	}{
		{one, "o", RoleAdmin, false, true},
		{one, "o", "", true, true},
		{one, "a", RoleViewer, false, false},
		{one, "a", "", true, false},
		{one, "a", RoleOwner, false, false},
		{two, "o", RoleMember, false, false},
		{two, "o", "", true, false},
		{one, "o", RoleOwner, false, false},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			err := CheckLastOwner(tt.members, tt.target, tt.role, tt.removed)
			var ce domain.ConflictError
			if tt.conflict != (err != nil) || (err != nil && (!errors.As(err, &ce) || ce.Reason != domain.ConflictLastOwner)) {
				t.Errorf("err = %v, want conflict %v", err, tt.conflict)
			}
		})
	}
}

func TestInvitationState(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	base := Invitation{ExpiresAt: now.Add(time.Hour)}
	accepted, revoked, expired := base, base, base
	accepted.AcceptedAt = now.Add(-time.Minute)
	revoked.RevokedAt = now.Add(-time.Minute)
	expired.ExpiresAt = now
	tests := []struct {
		inv   Invitation
		state InvitationState
		err   error
	}{
		{base, InvitationPending, nil},
		{accepted, InvitationAccepted, domain.ConflictError{Reason: domain.ConflictInvitationUsed}},
		{revoked, InvitationRevoked, domain.ErrNotFound},
		{expired, InvitationExpired, domain.ConflictError{Reason: domain.ConflictInvitationExpired}},
	}
	for _, tt := range tests {
		if got := tt.inv.State(now); got != tt.state {
			t.Errorf("State = %s, want %s", got, tt.state)
		}
		if err := tt.inv.CheckUsable(now); err != tt.err {
			t.Errorf("CheckUsable = %v, want %v", err, tt.err)
		}
	}
}
