package sqldb

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/subscription"
	"github.com/paramah/denar/internal/usecase"
)

var _ usecase.SubscriptionRepository = (*SubscriptionRepository)(nil)

// SubscriptionRepository stores subscriptions in subscriptions and their
// price points in subscription_prices.
type SubscriptionRepository struct{ db *DB }

func NewSubscriptionRepository(db *DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

const subColumns = `id, household_id, name, vendor, external_ref, cycle_unit, cycle_every, cycle_anchor, trial_ends_at, status,
	cancelled_at, ends_at, wallet_id, envelope_id, reminder_offsets, payee_pattern, amount_tolerance_pct, date_window_days,
	note, manage_url, created_by_kind, created_by_id, created_at, updated_at`

func nullDate(d domain.Date) sql.NullString { return nullString(d.String()) }

func parseNullDate(field string, id domain.ID, v sql.NullString) (domain.Date, error) {
	if !v.Valid {
		return domain.Date{}, nil
	}
	d, err := domain.ParseDate(v.String)
	if err != nil {
		return domain.Date{}, fmt.Errorf("sqldb: %s of %s: %w", field, id, err)
	}
	return d, nil
}

func scanSub(row interface{ Scan(...any) error }) (subscription.Subscription, error) {
	var (
		s                                                    subscription.Subscription
		anchor, offsets                                      string
		externalRef, trialEnds, endsAt, walletID, envelopeID sql.NullString
		manageURL                                            sql.NullString
		cancelled                                            sql.NullInt64
		created, updated                                     int64
	)
	if err := row.Scan(&s.ID, &s.HouseholdID, &s.Name, &s.Vendor, &externalRef, &s.Cycle.Unit, &s.Cycle.Every, &anchor, &trialEnds,
		&s.Status, &cancelled, &endsAt, &walletID, &envelopeID, &offsets, &s.Match.PayeePattern, &s.Match.AmountTolerancePct,
		&s.Match.DateWindowDays, &s.Note, &manageURL, &s.CreatedBy.Kind, &s.CreatedBy.ID, &created, &updated); err != nil {
		return subscription.Subscription{}, err
	}
	var err error
	if s.Cycle.Anchor, err = domain.ParseDate(anchor); err != nil {
		return subscription.Subscription{}, fmt.Errorf("sqldb: anchor of %s: %w", s.ID, err)
	}
	if s.TrialEndsAt, err = parseNullDate("trial end", s.ID, trialEnds); err != nil {
		return subscription.Subscription{}, err
	}
	if s.EndsAt, err = parseNullDate("end", s.ID, endsAt); err != nil {
		return subscription.Subscription{}, err
	}
	if err := json.Unmarshal([]byte(offsets), &s.ReminderOffsets); err != nil {
		return subscription.Subscription{}, fmt.Errorf("sqldb: reminders of %s: %w", s.ID, err)
	}
	if s.ReminderOffsets == nil {
		s.ReminderOffsets = []int{}
	}
	s.ExternalRef, s.ManageURL = externalRef.String, manageURL.String
	s.WalletID, s.EnvelopeID = domain.ID(walletID.String), domain.ID(envelopeID.String)
	s.CancelledAt, s.CreatedAt, s.UpdatedAt = fromNullMicros(cancelled), fromMicros(created), fromMicros(updated)
	return s, nil
}

func offsetsJSON(o []int) string {
	if o == nil {
		o = []int{}
	}
	b, _ := json.Marshal(o)
	return string(b)
}

func mapSubWriteError(err error) error {
	switch {
	case isUnique(err, "subscriptions.household_id"), isUnique(err, "subscriptions.name"):
		return domain.ConflictError{Reason: domain.ConflictDuplicateName}
	case isUnique(err, "subscriptions."):
		return domain.ErrConflict
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

func (r *SubscriptionRepository) Create(ctx context.Context, s subscription.Subscription) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		_, err := r.db.Exec(ctx).ExecContext(ctx, `INSERT INTO subscriptions (`+subColumns+`) VALUES (`+placeholders(24)+`)`,
			s.ID, s.HouseholdID, s.Name, s.Vendor, nullString(s.ExternalRef), s.Cycle.Unit, s.Cycle.Every, s.Cycle.Anchor.String(),
			nullDate(s.TrialEndsAt), s.Status, nullMicros(s.CancelledAt), nullDate(s.EndsAt), nullString(string(s.WalletID)),
			nullString(string(s.EnvelopeID)), offsetsJSON(s.ReminderOffsets), s.Match.PayeePattern, s.Match.AmountTolerancePct,
			s.Match.DateWindowDays, s.Note, nullString(s.ManageURL), s.CreatedBy.Kind, s.CreatedBy.ID, toMicros(s.CreatedAt),
			toMicros(s.UpdatedAt))
		if err != nil {
			return mapSubWriteError(err)
		}
		for _, p := range s.Prices {
			if err := r.PutPrice(ctx, s.HouseholdID, s.ID, p); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SubscriptionRepository) Update(ctx context.Context, s subscription.Subscription) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `UPDATE subscriptions SET name = ?, vendor = ?, external_ref = ?, cycle_unit = ?,
		cycle_every = ?, cycle_anchor = ?, trial_ends_at = ?, status = ?, cancelled_at = ?, ends_at = ?, wallet_id = ?, envelope_id = ?,
		reminder_offsets = ?, payee_pattern = ?, amount_tolerance_pct = ?, date_window_days = ?, note = ?, manage_url = ?, updated_at = ?
		WHERE id = ? AND household_id = ?`,
		s.Name, s.Vendor, nullString(s.ExternalRef), s.Cycle.Unit, s.Cycle.Every, s.Cycle.Anchor.String(), nullDate(s.TrialEndsAt),
		s.Status, nullMicros(s.CancelledAt), nullDate(s.EndsAt), nullString(string(s.WalletID)), nullString(string(s.EnvelopeID)),
		offsetsJSON(s.ReminderOffsets), s.Match.PayeePattern, s.Match.AmountTolerancePct, s.Match.DateWindowDays, s.Note,
		nullString(s.ManageURL), toMicros(s.UpdatedAt), s.ID, s.HouseholdID)
	if err != nil {
		return mapSubWriteError(err)
	}
	return affected(res)
}

func (r *SubscriptionRepository) PutPrice(ctx context.Context, householdID, subscriptionID domain.ID, p subscription.PricePoint) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		var n int
		if err := r.db.Exec(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM subscriptions WHERE id = ? AND household_id = ?`,
			subscriptionID, householdID).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		_, err := r.db.Exec(ctx).ExecContext(ctx, `INSERT INTO subscription_prices (id, subscription_id, effective_from, amount, currency, created_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (subscription_id, effective_from) DO UPDATE SET id = excluded.id, amount = excluded.amount,
				currency = excluded.currency, created_at = excluded.created_at`,
			p.ID, subscriptionID, p.EffectiveFrom.String(), p.Amount, p.Currency, toMicros(p.CreatedAt))
		return err
	})
}

func (r *SubscriptionRepository) Delete(ctx context.Context, householdID, id domain.ID) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM subscriptions WHERE id = ? AND household_id = ?`, id, householdID)
	if err != nil {
		return err
	}
	return affected(res)
}

func (r *SubscriptionRepository) ByID(ctx context.Context, householdID, id domain.ID) (subscription.Subscription, error) {
	list, err := r.query(ctx, `SELECT `+subColumns+` FROM subscriptions WHERE id = ? AND household_id = ?`, id, householdID)
	if err != nil {
		return subscription.Subscription{}, err
	}
	if len(list) == 0 {
		return subscription.Subscription{}, domain.ErrNotFound
	}
	return list[0], nil
}

func (r *SubscriptionRepository) List(ctx context.Context, householdID domain.ID) ([]subscription.Subscription, error) {
	var (
		out []subscription.Subscription
		err error
	)
	if householdID == "" {
		out, err = r.query(ctx, `SELECT `+subColumns+` FROM subscriptions ORDER BY household_id, id`)
	} else {
		out, err = r.query(ctx, `SELECT `+subColumns+` FROM subscriptions WHERE household_id = ? ORDER BY id`, householdID)
	}
	if err != nil {
		return nil, err
	}
	subscription.SortByName(out)
	sortByHousehold(out)
	return out, nil
}

// sortByHousehold keeps the name order within each household.
func sortByHousehold(ss []subscription.Subscription) {
	slices.SortStableFunc(ss, func(a, b subscription.Subscription) int { return cmp.Compare(a.HouseholdID, b.HouseholdID) })
}

func (r *SubscriptionRepository) CountByEnvelope(ctx context.Context, householdID, envelopeID domain.ID) (int, error) {
	var n int
	err := r.db.Query(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM subscriptions WHERE household_id = ? AND envelope_id = ?`,
		householdID, envelopeID).Scan(&n)
	return n, err
}

// query reads subscriptions and then their prices.
func (r *SubscriptionRepository) query(ctx context.Context, sqlText string, args ...any) ([]subscription.Subscription, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	out := []subscription.Subscription{}
	for rows.Next() {
		s, err := scanSub(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		s.Prices = []subscription.PricePoint{}
		out = append(out, s)
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
	index := map[domain.ID]int{}
	ids := make([]domain.ID, len(out))
	for i, s := range out {
		index[s.ID], ids[i] = i, s.ID
	}
	for start := 0; start < len(ids); start += idsChunk {
		chunk := ids[start:min(start+idsChunk, len(ids))]
		prows, err := r.db.Query(ctx).QueryContext(ctx, `SELECT subscription_id, id, effective_from, amount, currency, created_at
			FROM subscription_prices WHERE subscription_id IN (`+placeholders(len(chunk))+`) ORDER BY effective_from`, idArgs(chunk)...)
		if err != nil {
			return nil, err
		}
		for prows.Next() {
			var (
				sid     domain.ID
				p       subscription.PricePoint
				from    string
				created int64
			)
			if err := prows.Scan(&sid, &p.ID, &from, &p.Amount, &p.Currency, &created); err != nil {
				prows.Close()
				return nil, err
			}
			if p.EffectiveFrom, err = domain.ParseDate(from); err != nil {
				prows.Close()
				return nil, fmt.Errorf("sqldb: price %s: %w", p.ID, err)
			}
			p.CreatedAt = fromMicros(created)
			out[index[sid]].Prices = append(out[index[sid]].Prices, p)
		}
		if err := prows.Close(); err != nil {
			return nil, err
		}
		if err := prows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}
