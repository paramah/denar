package fakes

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"maps"
	"slices"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/imports"
	"github.com/paramah/denar/internal/usecase"
)

// ImportBatchRepository keeps import batches in memory. A transaction the
// linked TransactionRepository no longer has is no longer part of its
// batch, as the database's foreign key has it.
type ImportBatchRepository struct {
	store        *Store
	batches      map[domain.ID]imports.Batch
	links        map[domain.ID][]domain.ID
	transactions *TransactionRepository
}

func NewImportBatchRepository(s *Store, transactions *TransactionRepository) *ImportBatchRepository {
	r := &ImportBatchRepository{store: s, batches: map[domain.ID]imports.Batch{}, links: map[domain.ID][]domain.ID{}, transactions: transactions}
	s.Register(r)
	return r
}

func (r *ImportBatchRepository) Snapshot() (restore func()) {
	saved, links := maps.Clone(r.batches), map[domain.ID][]domain.ID{}
	for k, v := range r.links {
		links[k] = slices.Clone(v)
	}
	return func() { r.batches, r.links = saved, links }
}

func (r *ImportBatchRepository) dropHousehold(id domain.ID) {
	for k, b := range r.batches {
		if b.HouseholdID == id {
			delete(r.batches, k)
			delete(r.links, k)
		}
	}
}

func cloneBatch(b imports.Batch) imports.Batch {
	b.Errors = slices.Clone(b.Errors)
	if b.Errors == nil {
		b.Errors = []imports.RowError{}
	}
	return b
}

func (r *ImportBatchRepository) Create(ctx context.Context, b imports.Batch) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.batches[b.ID]; ok {
			err = domain.ErrConflict
			return
		}
		r.batches[b.ID] = cloneBatch(b)
	})
	return err
}

func (r *ImportBatchRepository) Update(ctx context.Context, b imports.Batch) (err error) {
	r.store.Do(ctx, func() {
		old, ok := r.batches[b.ID]
		if !ok || old.HouseholdID != b.HouseholdID {
			err = domain.ErrNotFound
			return
		}
		old.Status, old.Matched, old.Reverted, old.RevertedBy, old.RevertedAt = b.Status, b.Matched, b.Reverted, b.RevertedBy, b.RevertedAt
		r.batches[b.ID] = old
	})
	return err
}

func (r *ImportBatchRepository) ByID(ctx context.Context, householdID, id domain.ID) (b imports.Batch, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if b, ok = r.batches[id]; !ok || b.HouseholdID != householdID {
			b, err = imports.Batch{}, domain.ErrNotFound
		}
	})
	return cloneBatch(b), err
}

type importCursor struct {
	F  string    `json:"f"`
	ID domain.ID `json:"id"`
}

func importFingerprint(f usecase.ImportBatchFilter) string {
	ids := slices.Clone(f.WalletIDs)
	slices.Sort(ids)
	raw, _ := json.Marshal([]any{f.HouseholdID, ids})
	return string(raw)
}

func (r *ImportBatchRepository) List(ctx context.Context, f usecase.ImportBatchFilter, p usecase.Page) ([]imports.Batch, string, error) {
	fp := importFingerprint(f)
	var after domain.ID
	if p.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(p.Cursor)
		var c importCursor
		if err != nil || json.Unmarshal(raw, &c) != nil || c.F != fp || c.ID == "" {
			return nil, "", domain.ValidationError{Field: "cursor", Message: "is not a valid cursor of this query"}
		}
		after = c.ID
	}
	var matching []imports.Batch
	r.store.Do(ctx, func() {
		for _, b := range r.batches {
			if b.HouseholdID == f.HouseholdID && slices.Contains(f.WalletIDs, b.WalletID) && (after == "" || b.ID < after) {
				matching = append(matching, cloneBatch(b))
			}
		}
	})
	slices.SortFunc(matching, func(a, b imports.Batch) int { return cmp.Compare(b.ID, a.ID) })
	items, next := []imports.Batch{}, ""
	for i, b := range matching {
		if i == p.Limit {
			raw, _ := json.Marshal(importCursor{F: fp, ID: items[len(items)-1].ID})
			next = base64.RawURLEncoding.EncodeToString(raw)
			break
		}
		items = append(items, b)
	}
	return items, next, nil
}

func (r *ImportBatchRepository) AddTransactions(ctx context.Context, batchID domain.ID, ids []domain.ID) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.batches[batchID]; !ok {
			err = domain.ErrNotFound
			return
		}
		for _, id := range ids {
			if !r.transactions.exists(id) {
				err = domain.ErrNotFound
				return
			}
			if !slices.Contains(r.links[batchID], id) {
				r.links[batchID] = append(r.links[batchID], id)
			}
		}
	})
	return err
}

func (r *ImportBatchRepository) TransactionIDs(ctx context.Context, householdID, batchID domain.ID) (out []domain.ID, err error) {
	out = []domain.ID{}
	r.store.Do(ctx, func() {
		if b, ok := r.batches[batchID]; !ok || b.HouseholdID != householdID {
			err = domain.ErrNotFound
			return
		}
		for _, id := range r.links[batchID] {
			if r.transactions.exists(id) {
				out = append(out, id)
			}
		}
	})
	slices.Sort(out)
	return out, err
}
