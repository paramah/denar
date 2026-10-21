package usecase

import (
	"context"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/audit"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/imports"
)

type ListImportsInput struct {
	WalletID string
	Cursor   string
	Limit    domain.Optional[int]
}

type ImportPage struct {
	Items      []imports.Batch
	NextCursor string
}

// ListImports lists the import batches of the wallets the principal sees,
// newest first (FR-820).
type ListImports struct{ d Deps }

func NewListImports(d Deps) *ListImports { return &ListImports{d: d} }

func (uc *ListImports) Do(ctx context.Context, p Principal, householdID domain.ID, in ListImportsInput) (ImportPage, error) {
	page, err := NewPage(in.Cursor, in.Limit)
	if err != nil {
		return ImportPage{}, err
	}
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return ImportPage{}, err
	}
	if err := requireScope(s, bot.ScopeTransactionsRead); err != nil {
		return ImportPage{}, err
	}
	f := ImportBatchFilter{HouseholdID: householdID}
	if id := strings.TrimSpace(in.WalletID); id != "" {
		w, err := uc.d.visibleWallet(ctx, s, p, domain.ID(id))
		if err != nil {
			return ImportPage{}, err
		}
		f.WalletIDs = []domain.ID{w.ID}
	} else {
		ws, err := uc.d.visibleWallets(ctx, s, p)
		if err != nil {
			return ImportPage{}, err
		}
		f.WalletIDs = walletIDs(ws)
	}
	items, next, err := uc.d.ImportBatches.List(ctx, f, page)
	if err != nil {
		return ImportPage{}, cursorError(err, in.Cursor)
	}
	return ImportPage{Items: items, NextCursor: next}, nil
}

// visibleBatch is a batch of a wallet the principal sees; others are
// domain.ErrNotFound.
func (d Deps) visibleBatch(ctx context.Context, s HouseholdScope, p Principal, id domain.ID) (imports.Batch, error) {
	b, err := d.ImportBatches.ByID(ctx, s.Household.ID, id)
	if err != nil {
		return imports.Batch{}, plainNotFound(err)
	}
	if _, err := d.visibleWallet(ctx, s, p, b.WalletID); err != nil {
		return imports.Batch{}, err
	}
	return b, nil
}

// GetImport returns one import batch (FR-820).
type GetImport struct{ d Deps }

func NewGetImport(d Deps) *GetImport { return &GetImport{d: d} }

func (uc *GetImport) Do(ctx context.Context, p Principal, householdID, id domain.ID) (imports.Batch, error) {
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return imports.Batch{}, err
	}
	if err := requireScope(s, bot.ScopeTransactionsRead); err != nil {
		return imports.Batch{}, err
	}
	return uc.d.visibleBatch(ctx, s, p, id)
}

// RevertImport deletes the transactions of a batch that still exist,
// unlinking their subscription charges, and marks it reverted (FR-821,
// AD-807).
type RevertImport struct{ d Deps }

func NewRevertImport(d Deps) *RevertImport { return &RevertImport{d: d} }

func (uc *RevertImport) Do(ctx context.Context, p Principal, householdID, id domain.ID) (imports.Batch, error) {
	var out imports.Batch
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireScope(s, bot.ScopeTransactionsWrite); err != nil {
			return err
		}
		b, err := uc.d.visibleBatch(ctx, s, p, id)
		if err != nil {
			return err
		}
		if b.Status == imports.Reverted {
			return domain.ConflictError{Reason: domain.ConflictImportReverted}
		}
		if _, err := uc.d.writableWallet(ctx, s, p, b.WalletID); err != nil {
			return err
		}
		ids, err := uc.d.ImportBatches.TransactionIDs(ctx, householdID, b.ID)
		if err != nil {
			return err
		}
		deleted := 0
		for _, txID := range ids {
			t, err := uc.d.Transactions.ByID(ctx, householdID, txID)
			if found, err := ignoreNotFound(err); err != nil {
				return err
			} else if !found || t.IsTransfer() {
				continue
			}
			if err := uc.d.unlinkCharge(ctx, s.Household, p.Actor(), t.ID); err != nil {
				return err
			}
			if err := uc.d.Transactions.Delete(ctx, householdID, t.ID); err != nil {
				return err
			}
			changes := audit.Changes{}
			for field, c := range txChanges(t, s.Household.BaseCurrency) {
				changes[field] = audit.Change{From: c.To}
			}
			changes.Set("import_id", string(b.ID))
			if err := uc.d.record(ctx, householdID, p.Actor(), "transaction.deleted", audit.ResourceTransaction, t.ID, changes); err != nil {
				return err
			}
			deleted++
		}
		b.Status, b.Reverted, b.RevertedBy, b.RevertedAt = imports.Reverted, deleted, p.Actor(), uc.d.Clock.Now()
		if err := uc.d.ImportBatches.Update(ctx, b); err != nil {
			return err
		}
		out = b
		return uc.d.record(ctx, householdID, p.Actor(), "import.reverted", audit.ResourceImport, b.ID,
			audit.Changes{}.Diff("status", string(imports.Completed), string(imports.Reverted)).Set("reverted", deleted))
	})
	return out, err
}
