package fakes

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/transaction"
	"github.com/paramah/denar/internal/usecase"
)

// TransactionRepository keeps transactions in memory. Set CreateErr to
// make Create fail.
type TransactionRepository struct {
	CreateErr error

	store  *Store
	txs    map[domain.ID]transaction.Transaction
	payees *PayeeRepository
}

// NewTransactionRepository links the transactions to the payees, whose
// names the text search looks at and whose deletion clears them.
func NewTransactionRepository(s *Store, payees *PayeeRepository) *TransactionRepository {
	r := &TransactionRepository{store: s, txs: map[domain.ID]transaction.Transaction{}, payees: payees}
	payees.transactions = r
	s.Register(r)
	return r
}

func (r *TransactionRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.txs)
	return func() { r.txs = saved }
}

func (r *TransactionRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.txs, func(_ domain.ID, t transaction.Transaction) bool { return t.HouseholdID == id })
}

func (r *TransactionRepository) clearPayee(id domain.ID) {
	for k, t := range r.txs {
		if t.PayeeID == id {
			t.PayeeID = ""
			r.txs[k] = t
		}
	}
}

func cloneTx(t transaction.Transaction) transaction.Transaction {
	t.Tags = slices.Clone(t.Tags)
	if t.Tags == nil {
		t.Tags = []string{}
	}
	t.Splits = slices.Clone(t.Splits)
	if t.Splits == nil {
		t.Splits = []transaction.Split{}
	}
	if t.RatePending {
		t.BaseAmount, t.FxRate = 0, ""
		for i := range t.Splits {
			t.Splits[i].BaseAmount = 0
		}
	}
	return t
}

func (r *TransactionRepository) checkExternal(t transaction.Transaction) error {
	if t.ExternalID == "" {
		return nil
	}
	for _, o := range r.txs {
		if o.ID != t.ID && o.WalletID == t.WalletID && o.ExternalID == t.ExternalID {
			return domain.ConflictError{Reason: domain.ConflictDuplicateExternalID}
		}
	}
	return nil
}

func (r *TransactionRepository) Create(ctx context.Context, t transaction.Transaction) (err error) {
	if r.CreateErr != nil {
		return r.CreateErr
	}
	r.store.Do(ctx, func() {
		if _, ok := r.txs[t.ID]; ok {
			err = domain.ErrConflict
			return
		}
		if err = r.checkExternal(t); err == nil {
			r.txs[t.ID] = cloneTx(t)
		}
	})
	return err
}

func (r *TransactionRepository) Update(ctx context.Context, t transaction.Transaction) (err error) {
	r.store.Do(ctx, func() {
		old, ok := r.txs[t.ID]
		if !ok || old.HouseholdID != t.HouseholdID {
			err = domain.ErrNotFound
			return
		}
		t.HouseholdID, t.WalletID, t.Source, t.CreatedBy, t.CreatedAt = old.HouseholdID, old.WalletID, old.Source, old.CreatedBy, old.CreatedAt
		t.Currency = old.Currency
		if err = r.checkExternal(t); err == nil {
			r.txs[t.ID] = cloneTx(t)
		}
	})
	return err
}

func (r *TransactionRepository) Delete(ctx context.Context, householdID, id domain.ID) (err error) {
	r.store.Do(ctx, func() {
		if t, ok := r.txs[id]; !ok || t.HouseholdID != householdID {
			err = domain.ErrNotFound
			return
		}
		delete(r.txs, id)
	})
	return err
}

func (r *TransactionRepository) find(ctx context.Context, match func(transaction.Transaction) bool) []transaction.Transaction {
	out := []transaction.Transaction{}
	r.store.Do(ctx, func() {
		for _, t := range r.txs {
			if match(t) {
				out = append(out, cloneTx(t))
			}
		}
	})
	return out
}

func (r *TransactionRepository) ByID(ctx context.Context, householdID, id domain.ID) (transaction.Transaction, error) {
	found := r.find(ctx, func(t transaction.Transaction) bool { return t.ID == id && t.HouseholdID == householdID })
	if len(found) == 0 {
		return transaction.Transaction{}, domain.ErrNotFound
	}
	return found[0], nil
}

func (r *TransactionRepository) ByPair(ctx context.Context, householdID, pairID domain.ID) ([]transaction.Transaction, error) {
	found := r.find(ctx, func(t transaction.Transaction) bool {
		return pairID != "" && t.TransferPairID == pairID && t.HouseholdID == householdID
	})
	slices.SortFunc(found, func(a, b transaction.Transaction) int { return cmp.Compare(a.ID, b.ID) })
	return found, nil
}

type txCursor struct {
	F  string    `json:"f"`
	D  string    `json:"d"`
	ID domain.ID `json:"id"`
}

func filterFingerprint(f usecase.TransactionFilter) string {
	ids := slices.Clone(f.WalletIDs)
	slices.Sort(ids)
	canon, _ := json.Marshal([]any{f.HouseholdID, ids, f.PayeeID, f.EnvelopeID, f.From.String(), f.To.String(), f.Text,
		f.Kind, f.Status, f.Tag})
	sum := sha256.Sum256(canon)
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}

func (r *TransactionRepository) matches(f usecase.TransactionFilter, t transaction.Transaction) bool {
	if t.HouseholdID != f.HouseholdID || !slices.Contains(f.WalletIDs, t.WalletID) ||
		(f.PayeeID != "" && t.PayeeID != f.PayeeID) ||
		(!f.From.IsZero() && t.Date.Before(f.From)) || (!f.To.IsZero() && t.Date.After(f.To)) ||
		(f.Kind != "" && t.Kind != f.Kind) || (f.Status != "" && t.Status != f.Status) ||
		(f.Tag != "" && !slices.Contains(t.Tags, f.Tag)) {
		return false
	}
	if f.EnvelopeID != "" && t.EnvelopeID != f.EnvelopeID &&
		!slices.ContainsFunc(t.Splits, func(s transaction.Split) bool { return s.EnvelopeID == f.EnvelopeID }) {
		return false
	}
	if f.Text != "" {
		q := strings.ToLower(f.Text)
		name := ""
		if t.PayeeID != "" {
			name = r.payees.name(t.PayeeID)
		}
		if !strings.Contains(strings.ToLower(t.Note), q) && !strings.Contains(strings.ToLower(name), q) {
			return false
		}
	}
	return true
}

func (r *TransactionRepository) List(ctx context.Context, f usecase.TransactionFilter, p usecase.Page) ([]transaction.Transaction, string, error) {
	fp := filterFingerprint(f)
	var after *txCursor
	if p.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(p.Cursor)
		var c txCursor
		if err != nil || json.Unmarshal(raw, &c) != nil || c.F != fp || c.ID == "" {
			return nil, "", domain.ValidationError{Field: "cursor", Message: "is not a valid cursor of this query"}
		}
		after = &c
	}
	found := r.find(ctx, func(t transaction.Transaction) bool {
		if !r.matches(f, t) {
			return false
		}
		if after != nil {
			d := t.Date.String()
			return d < after.D || (d == after.D && t.ID < after.ID)
		}
		return true
	})
	slices.SortFunc(found, func(a, b transaction.Transaction) int {
		return cmp.Or(b.Date.Compare(a.Date), cmp.Compare(b.ID, a.ID))
	})
	next := ""
	if len(found) > p.Limit {
		found = found[:p.Limit]
		last := found[len(found)-1]
		raw, _ := json.Marshal(txCursor{F: fp, D: last.Date.String(), ID: last.ID})
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return found, next, nil
}

func (r *TransactionRepository) Balances(ctx context.Context, householdID domain.ID, walletIDs []domain.ID) (map[domain.ID]usecase.WalletBalance, error) {
	out := make(map[domain.ID]usecase.WalletBalance, len(walletIDs))
	for _, id := range walletIDs {
		out[id] = usecase.WalletBalance{}
	}
	for _, t := range r.find(ctx, func(t transaction.Transaction) bool {
		return t.HouseholdID == householdID && slices.Contains(walletIDs, t.WalletID)
	}) {
		b := out[t.WalletID]
		b.Total += t.Amount
		if t.Status != transaction.Pending {
			b.Cleared += t.Amount
		}
		if t.RatePending {
			b.Pending++
		} else {
			b.Base += t.BaseAmount
		}
		out[t.WalletID] = b
	}
	return out, nil
}

func (r *TransactionRepository) ListRatePending(ctx context.Context, householdID domain.ID, limit int) ([]transaction.Transaction, error) {
	found := r.find(ctx, func(t transaction.Transaction) bool {
		return t.RatePending && (householdID == "" || t.HouseholdID == householdID)
	})
	slices.SortFunc(found, func(a, b transaction.Transaction) int { return cmp.Compare(a.ID, b.ID) })
	if len(found) > limit {
		found = found[:limit]
	}
	return found, nil
}

func (r *TransactionRepository) BudgetFlows(ctx context.Context, householdID domain.ID, walletIDs []domain.ID, to domain.Date) (usecase.BudgetFlows, error) {
	type key struct {
		date     string
		envelope domain.ID
		kind     transaction.Kind
	}
	sums := map[key]int64{}
	out := usecase.BudgetFlows{Flows: []usecase.BudgetFlow{}, Pending: map[domain.Date]int{}}
	for _, t := range r.find(ctx, func(t transaction.Transaction) bool {
		return t.HouseholdID == householdID && slices.Contains(walletIDs, t.WalletID) && (to.IsZero() || !t.Date.After(to))
	}) {
		if t.RatePending {
			out.Pending[t.Date]++
			continue
		}
		if len(t.Splits) == 0 {
			sums[key{t.Date.String(), t.EnvelopeID, t.Kind}] += t.BaseAmount
			continue
		}
		for _, s := range t.Splits {
			sums[key{t.Date.String(), s.EnvelopeID, t.Kind}] += s.BaseAmount
		}
	}
	for k, sum := range sums {
		d, _ := domain.ParseDate(k.date)
		out.Flows = append(out.Flows, usecase.BudgetFlow{Date: d, EnvelopeID: k.envelope, Kind: k.kind, Base: sum})
	}
	slices.SortFunc(out.Flows, func(a, b usecase.BudgetFlow) int {
		return cmp.Or(a.Date.Compare(b.Date), cmp.Compare(a.EnvelopeID, b.EnvelopeID), cmp.Compare(a.Kind, b.Kind))
	})
	return out, nil
}

func (r *TransactionRepository) CountByEnvelope(ctx context.Context, householdID, envelopeID domain.ID) (int, error) {
	return len(r.find(ctx, func(t transaction.Transaction) bool {
		return t.HouseholdID == householdID && envelopeID != "" && (t.EnvelopeID == envelopeID ||
			slices.ContainsFunc(t.Splits, func(s transaction.Split) bool { return s.EnvelopeID == envelopeID }))
	})), nil
}

func (r *TransactionRepository) ExternalIDs(ctx context.Context, householdID, walletID domain.ID, externalIDs []string) (map[string]domain.ID, error) {
	want := map[string]bool{}
	for _, id := range externalIDs {
		want[id] = true
	}
	out := map[string]domain.ID{}
	r.store.Do(ctx, func() {
		for _, t := range r.txs {
			if t.HouseholdID == householdID && t.WalletID == walletID && t.ExternalID != "" && want[t.ExternalID] {
				out[t.ExternalID] = t.ID
			}
		}
	})
	return out, nil
}

// exists reports whether the transaction is stored; call it while the
// store is held.
func (r *TransactionRepository) exists(id domain.ID) bool {
	_, ok := r.txs[id]
	return ok
}
