package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
	"git.eiac.dev/tools/denar/internal/usecase"
)

var _ usecase.AuditRepository = (*AuditRepository)(nil)

// AuditRepository appends to and pages through the audit log. A trigger
// keeps the log append-only.
type AuditRepository struct{ db *DB }

func NewAuditRepository(db *DB) *AuditRepository { return &AuditRepository{db: db} }

func (r *AuditRepository) Append(ctx context.Context, e audit.Entry) error {
	changes := e.Changes
	if changes == nil {
		changes = audit.Changes{}
	}
	raw, err := json.Marshal(changes)
	if err != nil {
		return fmt.Errorf("sqlite: audit changes: %w", err)
	}
	_, err = r.db.Exec(ctx).ExecContext(ctx,
		`INSERT INTO audit_log (id, household_id, at, actor_kind, actor_id, action, resource_type, resource_id, changes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.HouseholdID, toMicros(e.At), string(e.Actor.Kind), e.Actor.ID, e.Action, e.ResourceType, e.ResourceID, string(raw))
	switch {
	case isUnique(err, "audit_log.id"):
		return domain.ErrConflict
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

var errBadAuditCursor = domain.ValidationError{Field: "cursor", Message: "is not a valid cursor of this query"}

type auditCursorJSON struct {
	F  string    `json:"f"`
	ID domain.ID `json:"id"`
}

// auditFingerprint ties a cursor to the filter it was made for.
func auditFingerprint(f usecase.AuditFilter) string {
	canon, _ := json.Marshal([]string{string(f.HouseholdID), string(f.ActorKind), string(f.ActorID), f.ResourceType})
	sum := sha256.Sum256(canon)
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}

func (r *AuditRepository) List(ctx context.Context, f usecase.AuditFilter, p usecase.Page) ([]audit.Entry, string, error) {
	where := []string{"household_id = ?"}
	args := []any{f.HouseholdID}
	if f.ActorKind != "" {
		where, args = append(where, "actor_kind = ?"), append(args, string(f.ActorKind))
	}
	if f.ActorID != "" {
		where, args = append(where, "actor_id = ?"), append(args, f.ActorID)
	}
	if f.ResourceType != "" {
		where, args = append(where, "resource_type = ?"), append(args, f.ResourceType)
	}
	fp := auditFingerprint(f)
	if p.Cursor != "" {
		raw, err := base64.RawURLEncoding.Strict().DecodeString(p.Cursor)
		var c auditCursorJSON
		if err != nil || json.Unmarshal(raw, &c) != nil || c.F != fp || c.ID == "" {
			return nil, "", errBadAuditCursor
		}
		where, args = append(where, "id < ?"), append(args, c.ID)
	}
	args = append(args, p.Limit+1)
	rows, err := r.db.Query(ctx).QueryContext(ctx,
		`SELECT id, household_id, at, actor_kind, actor_id, action, resource_type, resource_id, changes
		 FROM audit_log WHERE `+strings.Join(where, " AND ")+` ORDER BY id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := []audit.Entry{}
	for rows.Next() {
		var (
			e       audit.Entry
			at      int64
			kind    string
			changes string
		)
		if err := rows.Scan(&e.ID, &e.HouseholdID, &at, &kind, &e.Actor.ID, &e.Action, &e.ResourceType, &e.ResourceID, &changes); err != nil {
			return nil, "", err
		}
		e.At, e.Actor.Kind = fromMicros(at), domain.ActorKind(kind)
		if err := json.Unmarshal([]byte(changes), &e.Changes); err != nil {
			return nil, "", fmt.Errorf("sqlite: changes of audit entry %s: %w", e.ID, err)
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > p.Limit {
		items = items[:p.Limit]
		raw, _ := json.Marshal(auditCursorJSON{F: fp, ID: items[len(items)-1].ID})
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return items, next, nil
}
