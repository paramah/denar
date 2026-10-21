package sqlite

import (
	"context"
	"database/sql"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
	"git.eiac.dev/tools/denar/internal/usecase"
)

var _ usecase.WalletRepository = (*WalletRepository)(nil)

// WalletRepository stores wallets and the members of private ones in
// wallet_members.
type WalletRepository struct{ db *DB }

func NewWalletRepository(db *DB) *WalletRepository { return &WalletRepository{db: db} }

const walletColumns = `id, household_id, name, type, currency, on_budget, visibility, created_by, created_at, archived_at`

func scanWallet(row interface{ Scan(...any) error }) (wallet.Wallet, error) {
	var (
		w        wallet.Wallet
		created  int64
		archived sql.NullInt64
	)
	if err := row.Scan(&w.ID, &w.HouseholdID, &w.Name, &w.Type, &w.Currency, &w.OnBudget, &w.Visibility, &w.CreatedBy,
		&created, &archived); err != nil {
		return wallet.Wallet{}, err
	}
	w.CreatedAt, w.ArchivedAt, w.MemberIDs = fromMicros(created), fromNullMicros(archived), []domain.ID{}
	return w, nil
}

func mapWalletWriteError(err error) error {
	switch {
	case isUnique(err, "wallets.household_id"), isUnique(err, "wallets.name"):
		return domain.ConflictError{Reason: domain.ConflictDuplicateName}
	case isUnique(err, "wallets."):
		return domain.ErrConflict
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

func (r *WalletRepository) putMembers(ctx context.Context, id domain.ID, members []domain.ID) error {
	if _, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM wallet_members WHERE wallet_id = ?`, id); err != nil {
		return err
	}
	for _, m := range members {
		if _, err := r.db.Exec(ctx).ExecContext(ctx, `INSERT OR IGNORE INTO wallet_members (wallet_id, user_id) VALUES (?, ?)`, id, m); err != nil {
			return mapWalletWriteError(err)
		}
	}
	return nil
}

func (r *WalletRepository) Create(ctx context.Context, w wallet.Wallet) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		_, err := r.db.Exec(ctx).ExecContext(ctx, `INSERT INTO wallets (`+walletColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			w.ID, w.HouseholdID, w.Name, w.Type, w.Currency, w.OnBudget, w.Visibility, w.CreatedBy, toMicros(w.CreatedAt),
			nullMicros(w.ArchivedAt))
		if err != nil {
			return mapWalletWriteError(err)
		}
		return r.putMembers(ctx, w.ID, w.MemberIDs)
	})
}

func (r *WalletRepository) Update(ctx context.Context, w wallet.Wallet) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		res, err := r.db.Exec(ctx).ExecContext(ctx,
			`UPDATE wallets SET name = ?, type = ?, on_budget = ?, visibility = ?, archived_at = ? WHERE id = ? AND household_id = ?`,
			w.Name, w.Type, w.OnBudget, w.Visibility, nullMicros(w.ArchivedAt), w.ID, w.HouseholdID)
		if err != nil {
			return mapWalletWriteError(err)
		}
		if err := affected(res); err != nil {
			return err
		}
		return r.putMembers(ctx, w.ID, w.MemberIDs)
	})
}

func (r *WalletRepository) query(ctx context.Context, where string, args ...any) ([]wallet.Wallet, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx, `SELECT `+walletColumns+` FROM wallets WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	out := []wallet.Wallet{}
	index := map[domain.ID]int{}
	for rows.Next() {
		w, err := scanWallet(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		index[w.ID] = len(out)
		out = append(out, w)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	householdID := out[0].HouseholdID
	members, err := r.db.Query(ctx).QueryContext(ctx,
		`SELECT m.wallet_id, m.user_id FROM wallet_members m JOIN wallets w ON w.id = m.wallet_id
		 WHERE w.household_id = ? ORDER BY m.wallet_id, m.user_id`, householdID)
	if err != nil {
		return nil, err
	}
	defer members.Close()
	for members.Next() {
		var walletID, userID domain.ID
		if err := members.Scan(&walletID, &userID); err != nil {
			return nil, err
		}
		if i, ok := index[walletID]; ok {
			out[i].MemberIDs = append(out[i].MemberIDs, userID)
		}
	}
	return out, members.Err()
}

func (r *WalletRepository) ByID(ctx context.Context, householdID, id domain.ID) (wallet.Wallet, error) {
	found, err := r.query(ctx, `id = ? AND household_id = ?`, id, householdID)
	if err != nil {
		return wallet.Wallet{}, err
	}
	if len(found) == 0 {
		return wallet.Wallet{}, domain.ErrNotFound
	}
	return found[0], nil
}

func (r *WalletRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]wallet.Wallet, error) {
	return r.query(ctx, `household_id = ? ORDER BY lower(name), id`, householdID)
}
