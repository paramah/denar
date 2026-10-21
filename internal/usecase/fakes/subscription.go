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
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/event"
	"github.com/paramah/denar/internal/domain/subscription"
	"github.com/paramah/denar/internal/usecase"
)

// SubscriptionRepository keeps subscriptions in memory; deleting one drops
// its charges in the ChargeRepository linked to it.
type SubscriptionRepository struct {
	store   *Store
	subs    map[domain.ID]subscription.Subscription
	charges *ChargeRepository
}

func NewSubscriptionRepository(s *Store) *SubscriptionRepository {
	r := &SubscriptionRepository{store: s, subs: map[domain.ID]subscription.Subscription{}}
	s.Register(r)
	return r
}

func (r *SubscriptionRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.subs)
	return func() { r.subs = saved }
}

func (r *SubscriptionRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.subs, func(_ domain.ID, s subscription.Subscription) bool { return s.HouseholdID == id })
}

func cloneSub(s subscription.Subscription) subscription.Subscription {
	s.ReminderOffsets = slices.Clone(s.ReminderOffsets)
	if s.ReminderOffsets == nil {
		s.ReminderOffsets = []int{}
	}
	s.Prices = slices.Clone(s.Prices)
	subscription.SortPrices(s.Prices)
	return s
}

func (r *SubscriptionRepository) checkName(s subscription.Subscription) error {
	for _, o := range r.subs {
		if o.ID != s.ID && o.HouseholdID == s.HouseholdID && strings.EqualFold(o.Name, s.Name) {
			return domain.ConflictError{Reason: domain.ConflictDuplicateName}
		}
	}
	return nil
}

func (r *SubscriptionRepository) Create(ctx context.Context, s subscription.Subscription) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.subs[s.ID]; ok {
			err = domain.ErrConflict
			return
		}
		if err = r.checkName(s); err == nil {
			r.subs[s.ID] = cloneSub(s)
		}
	})
	return err
}

func (r *SubscriptionRepository) Update(ctx context.Context, s subscription.Subscription) (err error) {
	r.store.Do(ctx, func() {
		old, ok := r.subs[s.ID]
		if !ok || old.HouseholdID != s.HouseholdID {
			err = domain.ErrNotFound
			return
		}
		if err = r.checkName(s); err != nil {
			return
		}
		s = cloneSub(s)
		s.Prices, s.CreatedBy, s.CreatedAt = old.Prices, old.CreatedBy, old.CreatedAt
		r.subs[s.ID] = s
	})
	return err
}

func (r *SubscriptionRepository) PutPrice(ctx context.Context, householdID, subscriptionID domain.ID, p subscription.PricePoint) (err error) {
	r.store.Do(ctx, func() {
		s, ok := r.subs[subscriptionID]
		if !ok || s.HouseholdID != householdID {
			err = domain.ErrNotFound
			return
		}
		s = cloneSub(s)
		s.Prices = slices.DeleteFunc(s.Prices, func(q subscription.PricePoint) bool { return q.EffectiveFrom == p.EffectiveFrom })
		s.Prices = append(s.Prices, p)
		subscription.SortPrices(s.Prices)
		r.subs[subscriptionID] = s
	})
	return err
}

func (r *SubscriptionRepository) Delete(ctx context.Context, householdID, id domain.ID) (err error) {
	r.store.Do(ctx, func() {
		if s, ok := r.subs[id]; !ok || s.HouseholdID != householdID {
			err = domain.ErrNotFound
			return
		}
		delete(r.subs, id)
		if r.charges != nil {
			r.charges.dropSubscription(id)
		}
	})
	return err
}

func (r *SubscriptionRepository) ByID(ctx context.Context, householdID, id domain.ID) (s subscription.Subscription, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if s, ok = r.subs[id]; !ok || s.HouseholdID != householdID {
			s, err = subscription.Subscription{}, domain.ErrNotFound
			return
		}
		s = cloneSub(s)
	})
	return s, err
}

func (r *SubscriptionRepository) List(ctx context.Context, householdID domain.ID) ([]subscription.Subscription, error) {
	out := []subscription.Subscription{}
	r.store.Do(ctx, func() {
		for _, s := range r.subs {
			if householdID == "" || s.HouseholdID == householdID {
				out = append(out, cloneSub(s))
			}
		}
	})
	subscription.SortByName(out)
	slices.SortStableFunc(out, func(a, b subscription.Subscription) int { return cmp.Compare(a.HouseholdID, b.HouseholdID) })
	return out, nil
}

func (r *SubscriptionRepository) CountByEnvelope(ctx context.Context, householdID, envelopeID domain.ID) (n int, _ error) {
	r.store.Do(ctx, func() {
		for _, s := range r.subs {
			if s.HouseholdID == householdID && s.EnvelopeID == envelopeID {
				n++
			}
		}
	})
	return n, nil
}

// ChargeRepository keeps charges in memory.
type ChargeRepository struct {
	store   *Store
	charges map[domain.ID]subscription.Charge
}

// NewChargeRepository links the charges to the subscriptions, whose
// deletion drops them.
func NewChargeRepository(s *Store, subs *SubscriptionRepository) *ChargeRepository {
	r := &ChargeRepository{store: s, charges: map[domain.ID]subscription.Charge{}}
	subs.charges = r
	s.Register(r)
	return r
}

func (r *ChargeRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.charges)
	return func() { r.charges = saved }
}

func (r *ChargeRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.charges, func(_ domain.ID, c subscription.Charge) bool { return c.HouseholdID == id })
}

func (r *ChargeRepository) dropSubscription(id domain.ID) {
	maps.DeleteFunc(r.charges, func(_ domain.ID, c subscription.Charge) bool { return c.SubscriptionID == id })
}

func (r *ChargeRepository) checkTransaction(c subscription.Charge) error {
	if c.TransactionID == "" {
		return nil
	}
	for _, o := range r.charges {
		if o.ID != c.ID && o.TransactionID == c.TransactionID {
			return domain.ConflictError{Reason: domain.ConflictTransactionLinked}
		}
	}
	return nil
}

func (r *ChargeRepository) Create(ctx context.Context, c subscription.Charge) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.charges[c.ID]; ok {
			err = domain.ErrConflict
			return
		}
		for _, o := range r.charges {
			if o.SubscriptionID == c.SubscriptionID && o.DueDate == c.DueDate {
				err = domain.ConflictError{Reason: domain.ConflictDuplicateCharge}
				return
			}
		}
		if err = r.checkTransaction(c); err == nil {
			r.charges[c.ID] = c
		}
	})
	return err
}

func (r *ChargeRepository) Update(ctx context.Context, c subscription.Charge) (err error) {
	r.store.Do(ctx, func() {
		old, ok := r.charges[c.ID]
		if !ok || old.HouseholdID != c.HouseholdID {
			err = domain.ErrNotFound
			return
		}
		if err = r.checkTransaction(c); err != nil {
			return
		}
		old.Amount, old.Currency, old.Status, old.TransactionID, old.UpdatedAt = c.Amount, c.Currency, c.Status, c.TransactionID, c.UpdatedAt
		r.charges[c.ID] = old
	})
	return err
}

func (r *ChargeRepository) Delete(ctx context.Context, householdID, id domain.ID) (err error) {
	r.store.Do(ctx, func() {
		if c, ok := r.charges[id]; !ok || c.HouseholdID != householdID {
			err = domain.ErrNotFound
			return
		}
		delete(r.charges, id)
	})
	return err
}

func (r *ChargeRepository) ByID(ctx context.Context, householdID, id domain.ID) (c subscription.Charge, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if c, ok = r.charges[id]; !ok || c.HouseholdID != householdID {
			c, err = subscription.Charge{}, domain.ErrNotFound
		}
	})
	return c, err
}

func (r *ChargeRepository) ByTransaction(ctx context.Context, householdID, transactionID domain.ID) (c subscription.Charge, err error) {
	err = domain.ErrNotFound
	r.store.Do(ctx, func() {
		for _, o := range r.charges {
			if transactionID != "" && o.HouseholdID == householdID && o.TransactionID == transactionID {
				c, err = o, nil
				return
			}
		}
	})
	return c, err
}

func (r *ChargeRepository) find(ctx context.Context, keep func(subscription.Charge) bool) []subscription.Charge {
	out := []subscription.Charge{}
	r.store.Do(ctx, func() {
		for _, c := range r.charges {
			if keep(c) {
				out = append(out, c)
			}
		}
	})
	slices.SortFunc(out, func(a, b subscription.Charge) int {
		return cmp.Or(a.DueDate.Compare(b.DueDate), cmp.Compare(a.ID, b.ID))
	})
	return out
}

func inRange(d, from, to domain.Date) bool {
	return (from.IsZero() || !d.Before(from)) && (to.IsZero() || !d.After(to))
}

type chargeCursor struct {
	F  string    `json:"f"`
	D  string    `json:"d"`
	ID domain.ID `json:"id"`
}

func chargeFingerprint(f usecase.ChargeFilter) string {
	ids := slices.Clone(f.SubscriptionIDs)
	slices.Sort(ids)
	statuses := slices.Clone(f.Statuses)
	slices.Sort(statuses)
	canon, _ := json.Marshal([]any{f.HouseholdID, ids, statuses, f.From.String(), f.To.String()})
	sum := sha256.Sum256(canon)
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}

func (r *ChargeRepository) List(ctx context.Context, f usecase.ChargeFilter, p usecase.Page) ([]subscription.Charge, string, error) {
	fp := chargeFingerprint(f)
	var after *chargeCursor
	if p.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(p.Cursor)
		var c chargeCursor
		if err != nil || json.Unmarshal(raw, &c) != nil || c.F != fp || c.ID == "" {
			return nil, "", domain.ValidationError{Field: "cursor", Message: "is not a valid cursor of this query"}
		}
		after = &c
	}
	found := r.find(ctx, func(c subscription.Charge) bool {
		if c.HouseholdID != f.HouseholdID || !slices.Contains(f.SubscriptionIDs, c.SubscriptionID) ||
			(len(f.Statuses) > 0 && !slices.Contains(f.Statuses, c.Status)) || !inRange(c.DueDate, f.From, f.To) {
			return false
		}
		if after != nil {
			d := c.DueDate.String()
			return d > after.D || (d == after.D && c.ID > after.ID)
		}
		return true
	})
	next := ""
	if len(found) > p.Limit {
		found = found[:p.Limit]
		last := found[len(found)-1]
		raw, _ := json.Marshal(chargeCursor{F: fp, D: last.DueDate.String(), ID: last.ID})
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return found, next, nil
}

func (r *ChargeRepository) ListBySubscription(ctx context.Context, householdID, subscriptionID domain.ID, from domain.Date) ([]subscription.Charge, error) {
	return r.find(ctx, func(c subscription.Charge) bool {
		return c.HouseholdID == householdID && c.SubscriptionID == subscriptionID && inRange(c.DueDate, from, domain.Date{})
	}), nil
}

func (r *ChargeRepository) ListOpen(ctx context.Context, householdID domain.ID, from, to domain.Date, limit int) ([]subscription.Charge, error) {
	found := r.find(ctx, func(c subscription.Charge) bool {
		return (householdID == "" || c.HouseholdID == householdID) && c.Open() && inRange(c.DueDate, from, to)
	})
	if len(found) > limit {
		found = found[:limit]
	}
	return found, nil
}

// EventOutbox keeps domain events in memory.
type EventOutbox struct {
	store  *Store
	events []event.Event
}

func NewEventOutbox(s *Store) *EventOutbox {
	o := &EventOutbox{store: s}
	s.Register(o)
	return o
}

func (o *EventOutbox) Snapshot() (restore func()) {
	saved := slices.Clone(o.events)
	return func() { o.events = saved }
}

func (o *EventOutbox) dropHousehold(id domain.ID) {
	o.events = slices.DeleteFunc(o.events, func(e event.Event) bool { return e.HouseholdID == id })
}

func cloneEvent(e event.Event) event.Event {
	e.Payload = maps.Clone(e.Payload)
	if e.Payload == nil {
		e.Payload = map[string]string{}
	}
	return e
}

func (o *EventOutbox) Append(ctx context.Context, e event.Event) (stored bool, err error) {
	o.store.Do(ctx, func() {
		for _, x := range o.events {
			if x.ID == e.ID {
				err = domain.ErrConflict
				return
			}
			if x.HouseholdID == e.HouseholdID && x.DedupeKey == e.DedupeKey {
				return
			}
		}
		o.events = append(o.events, cloneEvent(e))
		stored = true
	})
	return stored, err
}

func (o *EventOutbox) ListPending(ctx context.Context, limit int) ([]event.Event, error) {
	out := []event.Event{}
	o.store.Do(ctx, func() {
		for _, e := range o.events {
			if !e.Dispatched() {
				out = append(out, cloneEvent(e))
			}
		}
	})
	slices.SortFunc(out, func(a, b event.Event) int { return cmp.Compare(a.ID, b.ID) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (o *EventOutbox) MarkDispatched(ctx context.Context, id domain.ID, at time.Time) (err error) {
	err = domain.ErrNotFound
	o.store.Do(ctx, func() {
		for i, e := range o.events {
			if e.ID == id {
				o.events[i].DispatchedAt, err = at, nil
				return
			}
		}
	})
	return err
}

// All returns every event, in the order appended.
func (o *EventOutbox) All() []event.Event {
	var out []event.Event
	o.store.Do(context.Background(), func() {
		for _, e := range o.events {
			out = append(out, cloneEvent(e))
		}
	})
	return out
}

func (o *EventOutbox) DeleteDispatchedBefore(ctx context.Context, before time.Time) (n int, err error) {
	o.store.Do(ctx, func() {
		o.events = slices.DeleteFunc(o.events, func(e event.Event) bool {
			if e.Dispatched() && e.DispatchedAt.Before(before) {
				n++
				return true
			}
			return false
		})
	})
	return n, nil
}
