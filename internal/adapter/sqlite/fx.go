package sqlite

import (
	"context"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
)

var (
	_ usecase.FxRateRepository     = (*FxRateRepository)(nil)
	_ usecase.FxOverrideRepository = (*FxOverrideRepository)(nil)
)

// FxRateRepository is the cache of NBP rates in fx_rates.
type FxRateRepository struct{ db *DB }

func NewFxRateRepository(db *DB) *FxRateRepository { return &FxRateRepository{db: db} }

func scanFxRate(row interface{ Scan(...any) error }) (usecase.FxRate, error) {
	var (
		r              usecase.FxRate
		date, rateDate string
		fetched        int64
	)
	if err := row.Scan(&r.Currency, &date, &rateDate, &r.Rate, &fetched); err != nil {
		return usecase.FxRate{}, err
	}
	var err error
	if r.Date, err = domain.ParseDate(date); err != nil {
		return usecase.FxRate{}, err
	}
	if r.RateDate, err = domain.ParseDate(rateDate); err != nil {
		return usecase.FxRate{}, err
	}
	r.FetchedAt = fromMicros(fetched)
	return r, nil
}

const fxRateColumns = `currency, date, rate_date, rate, fetched_at`

func (r *FxRateRepository) Get(ctx context.Context, currency string, date domain.Date) (usecase.FxRate, error) {
	fx, err := scanFxRate(r.db.Query(ctx).QueryRowContext(ctx,
		`SELECT `+fxRateColumns+` FROM fx_rates WHERE currency = ? AND date = ?`, currency, date.String()))
	return fx, notFound(err)
}

func (r *FxRateRepository) Put(ctx context.Context, fx usecase.FxRate) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx,
		`INSERT INTO fx_rates (`+fxRateColumns+`) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (currency, date) DO UPDATE SET rate_date = excluded.rate_date, rate = excluded.rate, fetched_at = excluded.fetched_at`,
		fx.Currency, fx.Date.String(), fx.RateDate.String(), fx.Rate, toMicros(fx.FetchedAt))
	return err
}

func (r *FxRateRepository) Latest(ctx context.Context, currency string, from, to domain.Date) (usecase.FxRate, error) {
	fx, err := scanFxRate(r.db.Query(ctx).QueryRowContext(ctx,
		`SELECT `+fxRateColumns+` FROM fx_rates WHERE currency = ? AND rate_date BETWEEN ? AND ?
		 ORDER BY rate_date DESC, date DESC LIMIT 1`, currency, from.String(), to.String()))
	return fx, notFound(err)
}

// FxOverrideRepository stores the rates households set by hand.
type FxOverrideRepository struct{ db *DB }

func NewFxOverrideRepository(db *DB) *FxOverrideRepository { return &FxOverrideRepository{db: db} }

const fxOverrideColumns = `household_id, currency, date, rate, created_by_kind, created_by_id, created_at`

func scanFxOverride(row interface{ Scan(...any) error }) (usecase.FxOverride, error) {
	var (
		o       usecase.FxOverride
		date    string
		kind    string
		created int64
	)
	if err := row.Scan(&o.HouseholdID, &o.Currency, &date, &o.Rate, &kind, &o.CreatedBy.ID, &created); err != nil {
		return usecase.FxOverride{}, err
	}
	var err error
	if o.Date, err = domain.ParseDate(date); err != nil {
		return usecase.FxOverride{}, err
	}
	o.CreatedBy.Kind, o.CreatedAt = domain.ActorKind(kind), fromMicros(created)
	return o, nil
}

func (r *FxOverrideRepository) Put(ctx context.Context, o usecase.FxOverride) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx,
		`INSERT INTO fx_rate_overrides (`+fxOverrideColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (household_id, currency, date) DO UPDATE SET rate = excluded.rate,
		   created_by_kind = excluded.created_by_kind, created_by_id = excluded.created_by_id, created_at = excluded.created_at`,
		o.HouseholdID, o.Currency, o.Date.String(), o.Rate, string(o.CreatedBy.Kind), o.CreatedBy.ID, toMicros(o.CreatedAt))
	if isForeignKey(err) {
		return domain.ErrNotFound
	}
	return err
}

func (r *FxOverrideRepository) Get(ctx context.Context, householdID domain.ID, currency string, date domain.Date) (usecase.FxOverride, error) {
	o, err := scanFxOverride(r.db.Query(ctx).QueryRowContext(ctx,
		`SELECT `+fxOverrideColumns+` FROM fx_rate_overrides WHERE household_id = ? AND currency = ? AND date = ?`,
		householdID, currency, date.String()))
	return o, notFound(err)
}

func (r *FxOverrideRepository) List(ctx context.Context, householdID domain.ID) ([]usecase.FxOverride, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx,
		`SELECT `+fxOverrideColumns+` FROM fx_rate_overrides WHERE household_id = ? ORDER BY date DESC, currency`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []usecase.FxOverride{}
	for rows.Next() {
		o, err := scanFxOverride(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
