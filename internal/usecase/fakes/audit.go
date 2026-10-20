package fakes

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"maps"
	"slices"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// AuditRepository keeps audit entries in memory. Set AppendErr to make
// Append fail, to test that the change it describes is rolled back.
type AuditRepository struct {
	AppendErr error

	store   *Store
	entries []audit.Entry
}

func NewAuditRepository(s *Store) *AuditRepository {
	r := &AuditRepository{store: s}
	s.Register(r)
	return r
}

func (r *AuditRepository) Snapshot() (restore func()) {
	saved := slices.Clone(r.entries)
	return func() { r.entries = saved }
}

func (r *AuditRepository) dropHousehold(id domain.ID) {
	r.entries = slices.DeleteFunc(r.entries, func(e audit.Entry) bool { return e.HouseholdID == id })
}

func (r *AuditRepository) Append(ctx context.Context, e audit.Entry) (err error) {
	if r.AppendErr != nil {
		return r.AppendErr
	}
	r.store.Do(ctx, func() {
		for _, other := range r.entries {
			if other.ID == e.ID {
				err = domain.ErrConflict
				return
			}
		}
		e.Changes = maps.Clone(e.Changes)
		r.entries = append(r.entries, e)
	})
	return err
}

// All returns every entry, oldest first, for assertions.
func (r *AuditRepository) All() []audit.Entry {
	var out []audit.Entry
	r.store.Do(context.Background(), func() { out = slices.Clone(r.entries) })
	return out
}

type auditCursor struct {
	F  usecase.AuditFilter `json:"f"`
	ID domain.ID           `json:"id"`
}

func (r *AuditRepository) List(ctx context.Context, f usecase.AuditFilter, p usecase.Page) ([]audit.Entry, string, error) {
	var after domain.ID
	if p.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(p.Cursor)
		var c auditCursor
		if err != nil || json.Unmarshal(raw, &c) != nil || c.F != f || c.ID == "" {
			return nil, "", domain.ValidationError{Field: "cursor", Message: "is not a valid cursor of this query"}
		}
		after = c.ID
	}
	var matching []audit.Entry
	r.store.Do(ctx, func() {
		for _, e := range r.entries {
			if e.HouseholdID != f.HouseholdID ||
				(f.ActorKind != "" && e.Actor.Kind != f.ActorKind) ||
				(f.ActorID != "" && e.Actor.ID != f.ActorID) ||
				(f.ResourceType != "" && e.ResourceType != f.ResourceType) ||
				(after != "" && e.ID >= after) {
				continue
			}
			matching = append(matching, e)
		}
	})
	slices.SortFunc(matching, func(a, b audit.Entry) int { return cmp.Compare(b.ID, a.ID) })
	items := []audit.Entry{}
	next := ""
	for i, e := range matching {
		if i == p.Limit {
			raw, _ := json.Marshal(auditCursor{F: f, ID: items[len(items)-1].ID})
			next = base64.RawURLEncoding.EncodeToString(raw)
			break
		}
		items = append(items, e)
	}
	return items, next, nil
}
