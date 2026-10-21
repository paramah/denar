package fakes

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/notification"
	"github.com/paramah/denar/internal/usecase"
)

// NotificationRepository keeps notifications of the app in memory.
type NotificationRepository struct {
	store *Store
	items map[domain.ID]notification.Notification
}

func NewNotificationRepository(s *Store) *NotificationRepository {
	r := &NotificationRepository{store: s, items: map[domain.ID]notification.Notification{}}
	s.Register(r)
	return r
}

func (r *NotificationRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.items)
	return func() { r.items = saved }
}

func (r *NotificationRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.items, func(_ domain.ID, n notification.Notification) bool { return n.HouseholdID == id })
}

func (r *NotificationRepository) Create(ctx context.Context, n notification.Notification) (stored bool, err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.items[n.ID]; ok {
			err = domain.ErrConflict
			return
		}
		for _, o := range r.items {
			if o.EventID == n.EventID && o.UserID == n.UserID {
				return
			}
		}
		r.items[n.ID] = n
		stored = true
	})
	return stored, err
}

func (r *NotificationRepository) ByID(ctx context.Context, userID, id domain.ID) (n notification.Notification, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if n, ok = r.items[id]; !ok || n.UserID != userID {
			n, err = notification.Notification{}, domain.ErrNotFound
		}
	})
	return n, err
}

func notificationMatches(f usecase.NotificationFilter, n notification.Notification) bool {
	return n.UserID == f.UserID && (f.HouseholdID == "" || n.HouseholdID == f.HouseholdID) && (!f.UnreadOnly || !n.Read()) &&
		(!f.RestrictWallets || n.WalletID == "" || slices.Contains(f.WalletIDs, n.WalletID))
}

type notificationCursor struct {
	F  string    `json:"f"`
	ID domain.ID `json:"id"`
}

func filterKey(f usecase.NotificationFilter) string {
	raw, _ := json.Marshal(f)
	return string(raw)
}

func (r *NotificationRepository) List(ctx context.Context, f usecase.NotificationFilter, p usecase.Page) ([]notification.Notification, string, error) {
	var before domain.ID
	if p.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(p.Cursor)
		var c notificationCursor
		if err != nil || json.Unmarshal(raw, &c) != nil || c.F != filterKey(f) || c.ID == "" {
			return nil, "", domain.ValidationError{Field: "cursor", Message: "is not a valid cursor of this query"}
		}
		before = c.ID
	}
	var matching []notification.Notification
	r.store.Do(ctx, func() {
		for _, n := range r.items {
			if notificationMatches(f, n) && (before == "" || n.ID < before) {
				matching = append(matching, n)
			}
		}
	})
	slices.SortFunc(matching, func(a, b notification.Notification) int { return cmp.Compare(b.ID, a.ID) })
	items := []notification.Notification{}
	next := ""
	for i, n := range matching {
		if i == p.Limit {
			raw, _ := json.Marshal(notificationCursor{F: filterKey(f), ID: items[len(items)-1].ID})
			next = base64.RawURLEncoding.EncodeToString(raw)
			break
		}
		items = append(items, n)
	}
	return items, next, nil
}

func (r *NotificationRepository) CountUnread(ctx context.Context, f usecase.NotificationFilter) (n int, err error) {
	f.UnreadOnly = true
	r.store.Do(ctx, func() {
		for _, x := range r.items {
			if notificationMatches(f, x) {
				n++
			}
		}
	})
	return n, nil
}

func (r *NotificationRepository) MarkRead(ctx context.Context, userID, id domain.ID, at time.Time) (err error) {
	r.store.Do(ctx, func() {
		n, ok := r.items[id]
		if !ok || n.UserID != userID {
			err = domain.ErrNotFound
			return
		}
		if !n.Read() {
			n.ReadAt = at
			r.items[id] = n
		}
	})
	return err
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, f usecase.NotificationFilter, at time.Time) (count int, err error) {
	f.UnreadOnly = true
	r.store.Do(ctx, func() {
		for id, n := range r.items {
			if notificationMatches(f, n) {
				n.ReadAt = at
				r.items[id] = n
				count++
			}
		}
	})
	return count, nil
}

func (r *NotificationRepository) DeleteReadBefore(ctx context.Context, before time.Time) (count int, err error) {
	r.store.Do(ctx, func() {
		for id, n := range r.items {
			if n.Read() && n.ReadAt.Before(before) {
				delete(r.items, id)
				count++
			}
		}
	})
	return count, nil
}

// All returns every notification by id, for assertions.
func (r *NotificationRepository) All() []notification.Notification {
	var out []notification.Notification
	r.store.Do(context.Background(), func() { out = slices.Collect(maps.Values(r.items)) })
	slices.SortFunc(out, func(a, b notification.Notification) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// ChannelRepository keeps channels in memory; deleting one drops its
// deliveries.
type ChannelRepository struct {
	store      *Store
	items      map[domain.ID]notification.Channel
	deliveries *DeliveryRepository
}

func NewChannelRepository(s *Store, deliveries *DeliveryRepository) *ChannelRepository {
	r := &ChannelRepository{store: s, items: map[domain.ID]notification.Channel{}, deliveries: deliveries}
	s.Register(r)
	return r
}

func (r *ChannelRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.items)
	return func() { r.items = saved }
}

func cloneChannel(c notification.Channel) notification.Channel {
	c.Config = maps.Clone(c.Config)
	if c.Config == nil {
		c.Config = map[string]string{}
	}
	c.SealedSecrets = slices.Clone(c.SealedSecrets)
	return c
}

func (r *ChannelRepository) checkName(c notification.Channel) error {
	for _, o := range r.items {
		if o.ID != c.ID && o.UserID == c.UserID && strings.EqualFold(o.Name, c.Name) {
			return domain.ConflictError{Reason: domain.ConflictDuplicateName}
		}
	}
	return nil
}

func (r *ChannelRepository) Create(ctx context.Context, c notification.Channel) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.items[c.ID]; ok {
			err = domain.ErrConflict
			return
		}
		if err = r.checkName(c); err == nil {
			r.items[c.ID] = cloneChannel(c)
		}
	})
	return err
}

func (r *ChannelRepository) Update(ctx context.Context, c notification.Channel) (err error) {
	r.store.Do(ctx, func() {
		old, ok := r.items[c.ID]
		if !ok || old.UserID != c.UserID {
			err = domain.ErrNotFound
			return
		}
		if err = r.checkName(c); err != nil {
			return
		}
		c.Type, c.CreatedAt = old.Type, old.CreatedAt
		r.items[c.ID] = cloneChannel(c)
	})
	return err
}

func (r *ChannelRepository) Delete(ctx context.Context, userID, id domain.ID) (err error) {
	r.store.Do(ctx, func() {
		c, ok := r.items[id]
		if !ok || c.UserID != userID {
			err = domain.ErrNotFound
			return
		}
		delete(r.items, id)
		if r.deliveries != nil {
			r.deliveries.dropChannel(id)
		}
	})
	return err
}

func (r *ChannelRepository) ByID(ctx context.Context, userID, id domain.ID) (c notification.Channel, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if c, ok = r.items[id]; !ok || c.UserID != userID {
			c, err = notification.Channel{}, domain.ErrNotFound
			return
		}
		c = cloneChannel(c)
	})
	return c, err
}

func (r *ChannelRepository) ListByUser(ctx context.Context, userID domain.ID) (out []notification.Channel, err error) {
	r.store.Do(ctx, func() {
		out = []notification.Channel{}
		for _, c := range r.items {
			if c.UserID == userID {
				out = append(out, cloneChannel(c))
			}
		}
	})
	slices.SortFunc(out, func(a, b notification.Channel) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

type prefKey struct{ user, household domain.ID }

// PreferenceRepository keeps preferences in memory.
type PreferenceRepository struct {
	store      *Store
	households *HouseholdRepository
	items      map[prefKey][]notification.Preference
}

func NewPreferenceRepository(s *Store, households *HouseholdRepository) *PreferenceRepository {
	r := &PreferenceRepository{store: s, households: households, items: map[prefKey][]notification.Preference{}}
	s.Register(r)
	return r
}

func (r *PreferenceRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.items)
	return func() { r.items = saved }
}

func (r *PreferenceRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.items, func(k prefKey, _ []notification.Preference) bool { return k.household == id })
}

func clonePrefs(in []notification.Preference) []notification.Preference {
	out := make([]notification.Preference, len(in))
	for i, p := range in {
		p.ChannelIDs = slices.Clone(p.ChannelIDs)
		if p.ChannelIDs == nil {
			p.ChannelIDs = []domain.ID{}
		}
		out[i] = p
	}
	return out
}

func (r *PreferenceRepository) List(ctx context.Context, userID, householdID domain.ID) (out []notification.Preference, err error) {
	r.store.Do(ctx, func() { out = clonePrefs(r.items[prefKey{userID, householdID}]) })
	return out, nil
}

func (r *PreferenceRepository) Replace(ctx context.Context, userID, householdID domain.ID, prefs []notification.Preference) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.households.households[householdID]; !ok {
			err = domain.ErrNotFound
			return
		}
		if len(prefs) == 0 {
			delete(r.items, prefKey{userID, householdID})
			return
		}
		r.items[prefKey{userID, householdID}] = clonePrefs(prefs)
	})
	return err
}

// DeliveryRepository keeps deliveries in memory.
type DeliveryRepository struct {
	store *Store
	items map[domain.ID]notification.Delivery
}

func NewDeliveryRepository(s *Store) *DeliveryRepository {
	r := &DeliveryRepository{store: s, items: map[domain.ID]notification.Delivery{}}
	s.Register(r)
	return r
}

func (r *DeliveryRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.items)
	return func() { r.items = saved }
}

func (r *DeliveryRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.items, func(_ domain.ID, d notification.Delivery) bool { return d.HouseholdID == id })
}

// dropChannel runs while the store is held.
func (r *DeliveryRepository) dropChannel(id domain.ID) {
	maps.DeleteFunc(r.items, func(_ domain.ID, d notification.Delivery) bool { return d.ChannelID == id })
}

func cloneDelivery(d notification.Delivery) notification.Delivery {
	d.Data = maps.Clone(d.Data)
	if d.Data == nil {
		d.Data = map[string]string{}
	}
	return d
}

func (r *DeliveryRepository) Create(ctx context.Context, d notification.Delivery) (stored bool, err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.items[d.ID]; ok {
			err = domain.ErrConflict
			return
		}
		for _, o := range r.items {
			if o.EventID == d.EventID && o.UserID == d.UserID && o.ChannelID == d.ChannelID {
				return
			}
		}
		r.items[d.ID] = cloneDelivery(d)
		stored = true
	})
	return stored, err
}

func (r *DeliveryRepository) ListDue(ctx context.Context, now time.Time, limit int) (out []notification.Delivery, err error) {
	r.store.Do(ctx, func() {
		out = []notification.Delivery{}
		for _, d := range r.items {
			if d.Status == notification.Pending && !d.NextAttemptAt.After(now) {
				out = append(out, cloneDelivery(d))
			}
		}
	})
	slices.SortFunc(out, func(a, b notification.Delivery) int {
		return cmp.Or(a.NextAttemptAt.Compare(b.NextAttemptAt), cmp.Compare(a.ID, b.ID))
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *DeliveryRepository) ByID(ctx context.Context, id domain.ID) (d notification.Delivery, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if d, ok = r.items[id]; !ok {
			err = domain.ErrNotFound
			return
		}
		d = cloneDelivery(d)
	})
	return d, err
}

func (r *DeliveryRepository) Update(ctx context.Context, d notification.Delivery) (err error) {
	r.store.Do(ctx, func() {
		old, ok := r.items[d.ID]
		if !ok {
			err = domain.ErrNotFound
			return
		}
		old.Status, old.Attempts, old.NextAttemptAt, old.LastError, old.UpdatedAt, old.SentAt =
			d.Status, d.Attempts, d.NextAttemptAt, d.LastError, d.UpdatedAt, d.SentAt
		r.items[d.ID] = old
	})
	return err
}

func (r *DeliveryRepository) DeleteFinishedBefore(ctx context.Context, before time.Time) (count int, err error) {
	r.store.Do(ctx, func() {
		for id, d := range r.items {
			if d.Status != notification.Pending && d.UpdatedAt.Before(before) {
				delete(r.items, id)
				count++
			}
		}
	})
	return count, nil
}

// All returns every delivery by id, for assertions.
func (r *DeliveryRepository) All() []notification.Delivery {
	var out []notification.Delivery
	r.store.Do(context.Background(), func() {
		for _, d := range r.items {
			out = append(out, cloneDelivery(d))
		}
	})
	slices.SortFunc(out, func(a, b notification.Delivery) int { return cmp.Compare(a.ID, b.ID) })
	return out
}
