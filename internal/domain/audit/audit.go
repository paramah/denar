// Package audit models the trail of changes in a household: who, human or
// bot, did what to which resource.
package audit

import (
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
)

// Resource types of stage 001.
const (
	ResourceHousehold  = "household"
	ResourceMembership = "membership"
	ResourceInvitation = "invitation"
	ResourceBot        = "bot"
)

// Resource types of stage 002.
const (
	ResourceWallet      = "wallet"
	ResourceTransaction = "transaction"
	ResourcePayee       = "payee"
	ResourceFxRate      = "fx_rate"
)

// Resource types of stage 003.
const (
	ResourceEnvelopeGroup = "envelope_group"
	ResourceEnvelope      = "envelope"
	// ResourceAssignment entries name the envelope; the month is a change.
	ResourceAssignment = "assignment"
)

// Change is the value of one field before and after. Either side may be
// nil (created or removed). Values are plain JSON values; secrets never go
// here.
type Change struct {
	From any `json:"from"`
	To   any `json:"to"`
}

// Changes are the changed fields of an entry, by field name.
type Changes map[string]Change

// Entry is one change of a household.
type Entry struct {
	ID           domain.ID
	HouseholdID  domain.ID
	At           time.Time
	Actor        domain.Actor
	Action       string // for example "bot.created"
	ResourceType string
	ResourceID   domain.ID
	Changes      Changes
}

// Set records a field that got a value.
func (c Changes) Set(field string, to any) Changes {
	c[field] = Change{To: to}
	return c
}

// Diff records a field when its value changed.
func (c Changes) Diff(field string, from, to any) Changes {
	if from != to {
		c[field] = Change{From: from, To: to}
	}
	return c
}
