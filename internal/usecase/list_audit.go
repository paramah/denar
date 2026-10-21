package usecase

import (
	"context"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/audit"
	"github.com/paramah/denar/internal/domain/membership"
)

type ListAuditInput struct {
	ActorKind    string
	ActorID      string
	ResourceType string
	Cursor       string
	Limit        domain.Optional[int]
}

// AuditPage is one page of the audit log, newest first.
type AuditPage struct {
	Items      []audit.Entry
	NextCursor string
}

// ListAudit returns the audit log of a household to its owners and admins
// (FR-041).
type ListAudit struct{ d Deps }

func NewListAudit(d Deps) *ListAudit { return &ListAudit{d: d} }

func (uc *ListAudit) Do(ctx context.Context, p Principal, householdID domain.ID, in ListAuditInput) (AuditPage, error) {
	if err := requireHuman(p); err != nil {
		return AuditPage{}, err
	}
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return AuditPage{}, err
	}
	if err := requireRole(s, membership.RoleAdmin); err != nil {
		return AuditPage{}, err
	}
	page, err := NewPage(in.Cursor, in.Limit)
	if err != nil {
		return AuditPage{}, err
	}
	f := AuditFilter{
		HouseholdID:  householdID,
		ActorKind:    domain.ActorKind(strings.TrimSpace(in.ActorKind)),
		ActorID:      domain.ID(strings.TrimSpace(in.ActorID)),
		ResourceType: strings.TrimSpace(in.ResourceType),
	}
	if f.ActorKind != "" && !f.ActorKind.Valid() {
		return AuditPage{}, domain.ValidationError{Field: "actor_kind", Message: "must be user, bot or system"}
	}
	items, next, err := uc.d.Audit.List(ctx, f, page)
	if err != nil {
		return AuditPage{}, cursorError(err, in.Cursor)
	}
	return AuditPage{Items: items, NextCursor: next}, nil
}
