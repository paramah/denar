// Package membership models who belongs to a household and with which
// role, and the invitations that bring new members in.
package membership

import (
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
)

// Role is a permission level within one household.
type Role string

const (
	// RoleOwner may do everything, including deleting the household and
	// managing other owners.
	RoleOwner Role = "owner"
	// RoleAdmin manages members other than owners, bots and settings.
	RoleAdmin Role = "admin"
	// RoleMember reads and writes financial data.
	RoleMember Role = "member"
	// RoleViewer only reads.
	RoleViewer Role = "viewer"
)

// Roles lists every role, highest first.
var Roles = []Role{RoleOwner, RoleAdmin, RoleMember, RoleViewer}

// Valid reports whether r is a known role.
func (r Role) Valid() bool { return r.rank() > 0 }

func (r Role) rank() int {
	switch r {
	case RoleOwner:
		return 4
	case RoleAdmin:
		return 3
	case RoleMember:
		return 2
	case RoleViewer:
		return 1
	}
	return 0
}

// AtLeast reports whether r is min or higher. An unknown role is below
// every role.
func (r Role) AtLeast(min Role) bool { return r.rank() > 0 && r.rank() >= min.rank() }

// CanWrite reports whether the role may change financial data.
func (r Role) CanWrite() bool { return r.AtLeast(RoleMember) }

// CanManage reports whether the role administers the household: members,
// invitations, all bots, settings and the audit log.
func (r Role) CanManage() bool { return r.AtLeast(RoleAdmin) }

// ValidateRole checks a role given as input, on field.
func ValidateRole(field string, r Role) error {
	if !r.Valid() {
		return domain.ValidationError{Field: field, Message: "must be owner, admin, member or viewer"}
	}
	return nil
}

// Membership is one user's role in one household.
type Membership struct {
	HouseholdID domain.ID
	UserID      domain.ID
	Role        Role
	CreatedAt   time.Time
}

var errRole = domain.ForbiddenError{Reason: domain.ForbiddenRole}

// CheckRoleChange reports whether an actor with the given role may change
// a member's role from old to new. Only owners and admins change roles,
// and only an owner touches the owner role, given or taken.
func CheckRoleChange(actor, old, new Role) error {
	if !actor.CanManage() {
		return errRole
	}
	if actor != RoleOwner && (old == RoleOwner || new == RoleOwner) {
		return errRole
	}
	return nil
}

// CheckRemoval reports whether the actor may remove the target member.
// Anyone may leave; removing someone else takes the right to change their
// role.
func CheckRemoval(actorID domain.ID, actor Role, targetID domain.ID, target Role) error {
	if actorID == targetID {
		return nil
	}
	if !actor.CanManage() || (actor != RoleOwner && target == RoleOwner) {
		return errRole
	}
	return nil
}

// CheckInvite reports whether an actor may invite someone with the role.
func CheckInvite(actor, invited Role) error {
	if !actor.CanManage() || (actor != RoleOwner && invited == RoleOwner) {
		return errRole
	}
	return nil
}

// CheckLastOwner reports whether giving the target newRole, or removing it
// when removed is set, leaves the household with an owner. members are the
// memberships as stored before the change.
func CheckLastOwner(members []Membership, targetID domain.ID, newRole Role, removed bool) error {
	owners := 0
	for _, m := range members {
		role := m.Role
		if m.UserID == targetID {
			if removed {
				continue
			}
			role = newRole
		}
		if role == RoleOwner {
			owners++
		}
	}
	if owners == 0 {
		return domain.ConflictError{Reason: domain.ConflictLastOwner}
	}
	return nil
}
