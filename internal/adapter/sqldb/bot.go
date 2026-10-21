package sqldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/usecase"
)

var _ usecase.BotRepository = (*BotRepository)(nil)

// BotRepository stores bots, their scopes as a JSON array and their wallet
// list in bot_wallets.
type BotRepository struct{ db *DB }

func NewBotRepository(db *DB) *BotRepository { return &BotRepository{db: db} }

const botColumns = `id, household_id, name, scopes, created_by, created_at, expires_at, disabled, token_hash, token_prefix, last_used_at`

func scanBot(row interface{ Scan(...any) error }) (bot.Bot, error) {
	var (
		b               bot.Bot
		scopes          string
		created         int64
		expires, usedAt sql.NullInt64
	)
	if err := row.Scan(&b.ID, &b.HouseholdID, &b.Name, &scopes, &b.CreatedBy, &created, &expires, &b.Disabled,
		&b.TokenHash, &b.TokenPrefix, &usedAt); err != nil {
		return bot.Bot{}, err
	}
	if err := json.Unmarshal([]byte(scopes), &b.Scopes); err != nil {
		return bot.Bot{}, fmt.Errorf("sqldb: scopes of bot %s: %w", b.ID, err)
	}
	b.CreatedAt, b.ExpiresAt, b.LastUsedAt = fromMicros(created), fromNullMicros(expires), fromNullMicros(usedAt)
	b.WalletIDs = []domain.ID{}
	return b, nil
}

func scopesJSON(scopes []bot.Scope) string {
	if scopes == nil {
		scopes = []bot.Scope{}
	}
	raw, _ := json.Marshal(scopes) // a slice of strings always marshals
	return string(raw)
}

func mapBotWriteError(err error) error {
	switch {
	case isUnique(err, "bots.household_id"), isUnique(err, "bots.name"):
		return domain.ConflictError{Reason: domain.ConflictDuplicateName}
	case isUnique(err, "bots."):
		return domain.ErrConflict
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

func (r *BotRepository) Create(ctx context.Context, b bot.Bot) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		_, err := r.db.Exec(ctx).ExecContext(ctx,
			`INSERT INTO bots (`+botColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			b.ID, b.HouseholdID, b.Name, scopesJSON(b.Scopes), b.CreatedBy, toMicros(b.CreatedAt), nullMicros(b.ExpiresAt),
			b.Disabled, b.TokenHash, b.TokenPrefix, nullMicros(b.LastUsedAt))
		if err != nil {
			return mapBotWriteError(err)
		}
		return r.putWallets(ctx, b.ID, b.WalletIDs)
	})
}

func (r *BotRepository) putWallets(ctx context.Context, botID domain.ID, ids []domain.ID) error {
	if _, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM bot_wallets WHERE bot_id = ?`, botID); err != nil {
		return err
	}
	for i, id := range ids {
		if _, err := r.db.Exec(ctx).ExecContext(ctx,
			`INSERT INTO bot_wallets (bot_id, wallet_id, position) VALUES (?, ?, ?)`, botID, id, i); err != nil {
			return err
		}
	}
	return nil
}

func (r *BotRepository) Update(ctx context.Context, b bot.Bot) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		res, err := r.db.Exec(ctx).ExecContext(ctx,
			`UPDATE bots SET name = ?, scopes = ?, expires_at = ?, disabled = ?, token_hash = ?, token_prefix = ? WHERE id = ?`,
			b.Name, scopesJSON(b.Scopes), nullMicros(b.ExpiresAt), b.Disabled, b.TokenHash, b.TokenPrefix, b.ID)
		if err != nil {
			return mapBotWriteError(err)
		}
		if err := affected(res); err != nil {
			return err
		}
		return r.putWallets(ctx, b.ID, b.WalletIDs)
	})
}

func (r *BotRepository) Delete(ctx context.Context, id domain.ID) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM bots WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return affected(res)
}

// withWallets adds the wallet lists, with one query for all bots.
func (r *BotRepository) withWallets(ctx context.Context, bots []bot.Bot) ([]bot.Bot, error) {
	if len(bots) == 0 {
		return bots, nil
	}
	index := make(map[domain.ID]int, len(bots))
	ids := make([]domain.ID, len(bots))
	for i, b := range bots {
		index[b.ID], ids[i] = i, b.ID
	}
	for start := 0; start < len(ids); start += idsChunk {
		chunk := ids[start:min(start+idsChunk, len(ids))]
		rows, err := r.db.Query(ctx).QueryContext(ctx,
			`SELECT bot_id, wallet_id FROM bot_wallets WHERE bot_id IN (`+placeholders(len(chunk))+`) ORDER BY bot_id, position`,
			idArgs(chunk)...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var botID, walletID domain.ID
			if err := rows.Scan(&botID, &walletID); err != nil {
				rows.Close()
				return nil, err
			}
			i := index[botID]
			bots[i].WalletIDs = append(bots[i].WalletIDs, walletID)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return bots, nil
}

func (r *BotRepository) query(ctx context.Context, where string, args ...any) ([]bot.Bot, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx, `SELECT `+botColumns+` FROM bots WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	out := []bot.Bot{}
	for rows.Next() {
		b, err := scanBot(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, b)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return r.withWallets(ctx, out)
}

func (r *BotRepository) one(ctx context.Context, where string, arg any) (bot.Bot, error) {
	found, err := r.query(ctx, where, arg)
	if err != nil {
		return bot.Bot{}, err
	}
	if len(found) == 0 {
		return bot.Bot{}, domain.ErrNotFound
	}
	return found[0], nil
}

func (r *BotRepository) ByID(ctx context.Context, id domain.ID) (bot.Bot, error) {
	return r.one(ctx, `id = ?`, id)
}

func (r *BotRepository) ByTokenHash(ctx context.Context, hash string) (bot.Bot, error) {
	if hash == "" {
		return bot.Bot{}, domain.ErrNotFound
	}
	return r.one(ctx, `token_hash = ?`, hash)
}

func (r *BotRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]bot.Bot, error) {
	return r.query(ctx, `household_id = ? ORDER BY lower(name), id`, householdID)
}

func (r *BotRepository) DisableByCreator(ctx context.Context, householdID, creatorID domain.ID) ([]bot.Bot, error) {
	var out []bot.Bot
	err := r.db.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		out, err = r.query(ctx, `household_id = ? AND created_by = ? AND NOT disabled ORDER BY id`, householdID, creatorID)
		if err != nil {
			return err
		}
		_, err = r.db.Exec(ctx).ExecContext(ctx,
			`UPDATE bots SET disabled = TRUE WHERE household_id = ? AND created_by = ? AND NOT disabled`, householdID, creatorID)
		return err
	})
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Disabled = true
	}
	return out, nil
}

func (r *BotRepository) TouchUsed(ctx context.Context, id domain.ID, at time.Time) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `UPDATE bots SET last_used_at = ? WHERE id = ?`, toMicros(at), id)
	if err != nil {
		return err
	}
	return affected(res)
}
