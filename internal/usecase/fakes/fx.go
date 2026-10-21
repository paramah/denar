package fakes

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
)

// FxRateRepository keeps the rate cache in memory.
type FxRateRepository struct {
	store *Store
	rates map[string]usecase.FxRate // currency/date
}

func NewFxRateRepository(s *Store) *FxRateRepository {
	r := &FxRateRepository{store: s, rates: map[string]usecase.FxRate{}}
	s.Register(r)
	return r
}

func (r *FxRateRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.rates)
	return func() { r.rates = saved }
}

func rateKey(currency string, d domain.Date) string { return currency + "/" + d.String() }

func (r *FxRateRepository) Get(ctx context.Context, currency string, date domain.Date) (fx usecase.FxRate, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if fx, ok = r.rates[rateKey(currency, date)]; !ok {
			err = domain.ErrNotFound
		}
	})
	return fx, err
}

func (r *FxRateRepository) Put(ctx context.Context, fx usecase.FxRate) error {
	r.store.Do(ctx, func() { r.rates[rateKey(fx.Currency, fx.Date)] = fx })
	return nil
}

func (r *FxRateRepository) Latest(ctx context.Context, currency string, from, to domain.Date) (best usecase.FxRate, err error) {
	r.store.Do(ctx, func() {
		found := false
		for _, fx := range r.rates {
			if fx.Currency != currency || fx.RateDate.Before(from) || fx.RateDate.After(to) {
				continue
			}
			if !found || fx.RateDate.After(best.RateDate) {
				best, found = fx, true
			}
		}
		if !found {
			err = domain.ErrNotFound
		}
	})
	return best, err
}

// FxOverrideRepository keeps rates set by hand in memory.
type FxOverrideRepository struct {
	store     *Store
	overrides map[string]usecase.FxOverride // household/currency/date
}

func NewFxOverrideRepository(s *Store) *FxOverrideRepository {
	r := &FxOverrideRepository{store: s, overrides: map[string]usecase.FxOverride{}}
	s.Register(r)
	return r
}

func (r *FxOverrideRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.overrides)
	return func() { r.overrides = saved }
}

func (r *FxOverrideRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.overrides, func(_ string, o usecase.FxOverride) bool { return o.HouseholdID == id })
}

func overrideKey(householdID domain.ID, currency string, d domain.Date) string {
	return string(householdID) + "/" + rateKey(currency, d)
}

func (r *FxOverrideRepository) Put(ctx context.Context, o usecase.FxOverride) error {
	r.store.Do(ctx, func() { r.overrides[overrideKey(o.HouseholdID, o.Currency, o.Date)] = o })
	return nil
}

func (r *FxOverrideRepository) Get(ctx context.Context, householdID domain.ID, currency string, date domain.Date) (o usecase.FxOverride, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if o, ok = r.overrides[overrideKey(householdID, currency, date)]; !ok {
			err = domain.ErrNotFound
		}
	})
	return o, err
}

func (r *FxOverrideRepository) List(ctx context.Context, householdID domain.ID) ([]usecase.FxOverride, error) {
	out := []usecase.FxOverride{}
	r.store.Do(ctx, func() {
		for _, o := range r.overrides {
			if o.HouseholdID == householdID {
				out = append(out, o)
			}
		}
	})
	slices.SortFunc(out, func(a, b usecase.FxOverride) int {
		return cmp.Or(b.Date.Compare(a.Date), cmp.Compare(a.Currency, b.Currency))
	})
	return out, nil
}

// RateProvider answers from a table of NBP rates. Set Down to make every
// call fail as if the network were gone.
type RateProvider struct {
	mu    sync.Mutex
	Down  bool
	rates map[string]string // currency/date → PLN per unit
	calls []string
}

func NewRateProvider() *RateProvider { return &RateProvider{rates: map[string]string{}} }

// Set publishes a rate for a business day, such as Set("USD", "2026-10-20", "4.0000").
func (p *RateProvider) Set(currency, date, rate string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := domain.ParseDate(date)
	if err != nil {
		panic(err)
	}
	p.rates[rateKey(currency, d)] = rate
}

// SetDown switches the network off or on.
func (p *RateProvider) SetDown(down bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Down = down
}

// Calls returns the requests made, as currency/date.
func (p *RateProvider) Calls() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.calls)
}

func (p *RateProvider) PLNRate(_ context.Context, currency string, date domain.Date) (string, domain.Date, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, rateKey(currency, date))
	if p.Down {
		return "", domain.Date{}, fmt.Errorf("fake provider down: %w", usecase.ErrRateUnavailable)
	}
	for back := range 7 {
		d := date.AddDays(-back)
		if r, ok := p.rates[rateKey(currency, d)]; ok {
			return r, d, nil
		}
	}
	return "", domain.Date{}, usecase.ErrRateUnavailable
}
