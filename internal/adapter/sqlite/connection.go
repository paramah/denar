package sqlite

import (
	"context"
	"database/sql"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/connection"
	"github.com/paramah/denar/internal/usecase"
)

var _ usecase.ConnectionRepository = (*ConnectionRepository)(nil)

// ConnectionRepository stores connections, their sealed credentials as a
// BLOB, and their accounts.
type ConnectionRepository struct{ db *DB }

func NewConnectionRepository(db *DB) *ConnectionRepository { return &ConnectionRepository{db: db} }

const connectionColumns = `id, household_id, provider, kind, status, display_name, credentials, last_sync_at, last_error,
	created_by, created_at, updated_at`

func scanConnection(row interface{ Scan(...any) error }) (connection.Connection, error) {
	var (
		c                connection.Connection
		lastSync         sql.NullInt64
		created, updated int64
	)
	if err := row.Scan(&c.ID, &c.HouseholdID, &c.Provider, &c.Kind, &c.Status, &c.DisplayName, &c.SealedCredentials, &lastSync,
		&c.LastError, &c.CreatedBy, &created, &updated); err != nil {
		return connection.Connection{}, err
	}
	c.LastSyncAt, c.CreatedAt, c.UpdatedAt = fromNullMicros(lastSync), fromMicros(created), fromMicros(updated)
	return c, nil
}

func (r *ConnectionRepository) Create(ctx context.Context, c connection.Connection) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx, `INSERT INTO connections (`+connectionColumns+`) VALUES (`+placeholders(12)+`)`,
		c.ID, c.HouseholdID, c.Provider, c.Kind, c.Status, c.DisplayName, c.SealedCredentials, nullMicros(c.LastSyncAt), c.LastError,
		c.CreatedBy, toMicros(c.CreatedAt), toMicros(c.UpdatedAt))
	switch {
	case isUnique(err, "connections."):
		return domain.ErrConflict
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

func (r *ConnectionRepository) Update(ctx context.Context, c connection.Connection) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx,
		`UPDATE connections SET status = ?, display_name = ?, credentials = ?, last_sync_at = ?, last_error = ?, updated_at = ?
		 WHERE id = ? AND household_id = ?`,
		c.Status, c.DisplayName, c.SealedCredentials, nullMicros(c.LastSyncAt), c.LastError, toMicros(c.UpdatedAt), c.ID, c.HouseholdID)
	if err != nil {
		return err
	}
	return affected(res)
}

func (r *ConnectionRepository) ByID(ctx context.Context, householdID, id domain.ID) (connection.Connection, error) {
	c, err := scanConnection(r.db.Query(ctx).QueryRowContext(ctx,
		`SELECT `+connectionColumns+` FROM connections WHERE id = ? AND household_id = ?`, id, householdID))
	if err != nil {
		return connection.Connection{}, notFound(err)
	}
	return c, nil
}

func (r *ConnectionRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]connection.Connection, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx,
		`SELECT `+connectionColumns+` FROM connections WHERE household_id = ? ORDER BY display_name COLLATE NOCASE, id`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []connection.Connection{}
	for rows.Next() {
		c, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *ConnectionRepository) PutAccount(ctx context.Context, householdID domain.ID, a connection.Account) error {
	if _, err := r.ByID(ctx, householdID, a.ConnectionID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx).ExecContext(ctx,
		`INSERT INTO connection_accounts (connection_id, external_id, name, currency, wallet_id) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (connection_id, external_id) DO UPDATE SET name = excluded.name, currency = excluded.currency,
		   wallet_id = excluded.wallet_id`,
		a.ConnectionID, a.ExternalID, a.Name, a.Currency, nullString(string(a.WalletID)))
	if isForeignKey(err) {
		return domain.ErrNotFound
	}
	return err
}

func (r *ConnectionRepository) ListAccounts(ctx context.Context, householdID, connectionID domain.ID) ([]connection.Account, error) {
	if _, err := r.ByID(ctx, householdID, connectionID); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx).QueryContext(ctx,
		`SELECT connection_id, external_id, name, currency, wallet_id FROM connection_accounts WHERE connection_id = ? ORDER BY external_id`,
		connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []connection.Account{}
	for rows.Next() {
		var (
			a      connection.Account
			wallet sql.NullString
		)
		if err := rows.Scan(&a.ConnectionID, &a.ExternalID, &a.Name, &a.Currency, &wallet); err != nil {
			return nil, err
		}
		a.WalletID = domain.ID(wallet.String)
		out = append(out, a)
	}
	return out, rows.Err()
}
