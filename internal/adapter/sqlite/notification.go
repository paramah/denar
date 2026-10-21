package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/event"
	"git.eiac.dev/tools/denar/internal/domain/notification"
	"git.eiac.dev/tools/denar/internal/usecase"
)

var (
	_ usecase.NotificationRepository = (*NotificationRepository)(nil)
	_ usecase.ChannelRepository      = (*ChannelRepository)(nil)
	_ usecase.PreferenceRepository   = (*PreferenceRepository)(nil)
	_ usecase.DeliveryRepository     = (*DeliveryRepository)(nil)
)

// NotificationRepository stores notifications of the app.
type NotificationRepository struct{ db *DB }

func NewNotificationRepository(db *DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

const notificationColumns = `id, user_id, household_id, event_id, type, title, body, link, wallet_id, resource_type, resource_id,
	created_at, read_at`

func scanNotification(row interface{ Scan(...any) error }) (notification.Notification, error) {
	var (
		n       notification.Notification
		created int64
		read    sql.NullInt64
	)
	if err := row.Scan(&n.ID, &n.UserID, &n.HouseholdID, &n.EventID, &n.Type, &n.Title, &n.Body, &n.Link, &n.WalletID,
		&n.ResourceType, &n.ResourceID, &created, &read); err != nil {
		return notification.Notification{}, err
	}
	n.CreatedAt, n.ReadAt = fromMicros(created), fromNullMicros(read)
	return n, nil
}

func (r *NotificationRepository) Create(ctx context.Context, n notification.Notification) (bool, error) {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `INSERT INTO notifications (`+notificationColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (event_id, user_id) DO NOTHING`,
		n.ID, n.UserID, n.HouseholdID, n.EventID, n.Type, n.Title, n.Body, n.Link, n.WalletID, n.ResourceType, n.ResourceID,
		toMicros(n.CreatedAt), nullMicros(n.ReadAt))
	switch {
	case isUnique(err, "notifications.id"):
		return false, domain.ErrConflict
	case isForeignKey(err):
		return false, domain.ErrNotFound
	case err != nil:
		return false, err
	}
	count, err := res.RowsAffected()
	return count == 1, err
}

func (r *NotificationRepository) ByID(ctx context.Context, userID, id domain.ID) (notification.Notification, error) {
	n, err := scanNotification(r.db.Query(ctx).QueryRowContext(ctx,
		`SELECT `+notificationColumns+` FROM notifications WHERE id = ? AND user_id = ?`, id, userID))
	return n, notFound(err)
}

// notificationWhere builds the condition of a filter.
func notificationWhere(f usecase.NotificationFilter) (string, []any) {
	where := []string{"user_id = ?"}
	args := []any{f.UserID}
	if f.HouseholdID != "" {
		where, args = append(where, "household_id = ?"), append(args, f.HouseholdID)
	}
	if f.UnreadOnly {
		where = append(where, "read_at IS NULL")
	}
	if f.RestrictWallets {
		if len(f.WalletIDs) == 0 {
			where = append(where, "wallet_id = ''")
		} else {
			where = append(where, "(wallet_id = '' OR wallet_id IN ("+placeholders(len(f.WalletIDs))+"))")
			args = append(args, idArgs(f.WalletIDs)...)
		}
	}
	return strings.Join(where, " AND "), args
}

func notificationFingerprint(f usecase.NotificationFilter) string {
	ids := slices.Clone(f.WalletIDs)
	slices.Sort(ids)
	canon, _ := json.Marshal([]any{f.UserID, f.HouseholdID, f.UnreadOnly, f.RestrictWallets, ids})
	sum := sha256.Sum256(canon)
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}

var errBadNotificationCursor = domain.ValidationError{Field: "cursor", Message: "is not a valid cursor of this query"}

func (r *NotificationRepository) List(ctx context.Context, f usecase.NotificationFilter, p usecase.Page) ([]notification.Notification, string, error) {
	where, args := notificationWhere(f)
	fp := notificationFingerprint(f)
	if p.Cursor != "" {
		raw, err := base64.RawURLEncoding.Strict().DecodeString(p.Cursor)
		var c auditCursorJSON
		if err != nil || json.Unmarshal(raw, &c) != nil || c.F != fp || c.ID == "" {
			return nil, "", errBadNotificationCursor
		}
		where, args = where+" AND id < ?", append(args, c.ID)
	}
	args = append(args, p.Limit+1)
	rows, err := r.db.Query(ctx).QueryContext(ctx,
		`SELECT `+notificationColumns+` FROM notifications WHERE `+where+` ORDER BY id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := []notification.Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, "", err
		}
		items = append(items, n)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > p.Limit {
		items = items[:p.Limit]
		raw, _ := json.Marshal(auditCursorJSON{F: fp, ID: items[len(items)-1].ID})
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return items, next, nil
}

func (r *NotificationRepository) CountUnread(ctx context.Context, f usecase.NotificationFilter) (int, error) {
	f.UnreadOnly = true
	where, args := notificationWhere(f)
	var n int
	err := r.db.Query(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE `+where, args...).Scan(&n)
	return n, err
}

func (r *NotificationRepository) MarkRead(ctx context.Context, userID, id domain.ID, at time.Time) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx,
		`UPDATE notifications SET read_at = COALESCE(read_at, ?) WHERE id = ? AND user_id = ?`, toMicros(at), id, userID)
	if err != nil {
		return err
	}
	return affected(res)
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, f usecase.NotificationFilter, at time.Time) (int, error) {
	f.UnreadOnly = true
	where, args := notificationWhere(f)
	res, err := r.db.Exec(ctx).ExecContext(ctx, `UPDATE notifications SET read_at = ? WHERE `+where, append([]any{toMicros(at)}, args...)...)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

func (r *NotificationRepository) DeleteReadBefore(ctx context.Context, before time.Time) (int, error) {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM notifications WHERE read_at IS NOT NULL AND read_at < ?`, toMicros(before))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// ChannelRepository stores the channels of people; deleting one removes
// its deliveries through the foreign key.
type ChannelRepository struct{ db *DB }

func NewChannelRepository(db *DB) *ChannelRepository { return &ChannelRepository{db: db} }

const channelColumns = `id, user_id, type, name, config, secrets, enabled, verified_at, last_error, last_error_at, created_at, updated_at`

func scanChannel(row interface{ Scan(...any) error }) (notification.Channel, error) {
	var (
		c                 notification.Channel
		config            string
		verified, errored sql.NullInt64
		created, updated  int64
	)
	if err := row.Scan(&c.ID, &c.UserID, &c.Type, &c.Name, &config, &c.SealedSecrets, &c.Enabled, &verified, &c.LastError, &errored,
		&created, &updated); err != nil {
		return notification.Channel{}, err
	}
	if err := json.Unmarshal([]byte(config), &c.Config); err != nil {
		return notification.Channel{}, fmt.Errorf("sqlite: config of channel %s: %w", c.ID, err)
	}
	if c.Config == nil {
		c.Config = map[string]string{}
	}
	if len(c.SealedSecrets) == 0 {
		c.SealedSecrets = nil
	}
	c.VerifiedAt, c.LastErrorAt, c.CreatedAt, c.UpdatedAt = fromNullMicros(verified), fromNullMicros(errored), fromMicros(created), fromMicros(updated)
	return c, nil
}

func channelError(err error) error {
	switch {
	case isUnique(err, "notification_channels.user_id"), isUnique(err, "notification_channels.name"):
		return domain.ConflictError{Reason: domain.ConflictDuplicateName}
	case isUnique(err, "notification_channels."):
		return domain.ErrConflict
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

func configJSON(c map[string]string) string {
	if c == nil {
		c = map[string]string{}
	}
	raw, _ := json.Marshal(c)
	return string(raw)
}

func nullBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func (r *ChannelRepository) Create(ctx context.Context, c notification.Channel) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx, `INSERT INTO notification_channels (`+channelColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.UserID, c.Type, c.Name, configJSON(c.Config), nullBytes(c.SealedSecrets), c.Enabled, nullMicros(c.VerifiedAt), c.LastError,
		nullMicros(c.LastErrorAt), toMicros(c.CreatedAt), toMicros(c.UpdatedAt))
	return channelError(err)
}

func (r *ChannelRepository) Update(ctx context.Context, c notification.Channel) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `UPDATE notification_channels SET name = ?, config = ?, secrets = ?, enabled = ?,
		verified_at = ?, last_error = ?, last_error_at = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		c.Name, configJSON(c.Config), nullBytes(c.SealedSecrets), c.Enabled, nullMicros(c.VerifiedAt), c.LastError, nullMicros(c.LastErrorAt),
		toMicros(c.UpdatedAt), c.ID, c.UserID)
	if err != nil {
		return channelError(err)
	}
	return affected(res)
}

func (r *ChannelRepository) Delete(ctx context.Context, userID, id domain.ID) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM notification_channels WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	return affected(res)
}

func (r *ChannelRepository) ByID(ctx context.Context, userID, id domain.ID) (notification.Channel, error) {
	c, err := scanChannel(r.db.Query(ctx).QueryRowContext(ctx,
		`SELECT `+channelColumns+` FROM notification_channels WHERE id = ? AND user_id = ?`, id, userID))
	return c, notFound(err)
}

func (r *ChannelRepository) ListByUser(ctx context.Context, userID domain.ID) ([]notification.Channel, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx,
		`SELECT `+channelColumns+` FROM notification_channels WHERE user_id = ? ORDER BY name COLLATE NOCASE, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []notification.Channel{}
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// PreferenceRepository stores preferences, one row per type.
type PreferenceRepository struct{ db *DB }

func NewPreferenceRepository(db *DB) *PreferenceRepository { return &PreferenceRepository{db: db} }

func (r *PreferenceRepository) List(ctx context.Context, userID, householdID domain.ID) ([]notification.Preference, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx, `SELECT type, in_app, channel_ids FROM notification_preferences
		WHERE user_id = ? AND household_id = ? ORDER BY position, type`, userID, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []notification.Preference{}
	for rows.Next() {
		var (
			p   notification.Preference
			ids string
		)
		if err := rows.Scan(&p.Type, &p.InApp, &ids); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(ids), &p.ChannelIDs); err != nil {
			return nil, fmt.Errorf("sqlite: channels of a preference: %w", err)
		}
		if p.ChannelIDs == nil {
			p.ChannelIDs = []domain.ID{}
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PreferenceRepository) Replace(ctx context.Context, userID, householdID domain.ID, prefs []notification.Preference) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		var exists int
		err := r.db.Query(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM households WHERE id = ?`, householdID).Scan(&exists)
		if err != nil {
			return err
		}
		if exists == 0 {
			return domain.ErrNotFound
		}
		if _, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM notification_preferences WHERE user_id = ? AND household_id = ?`,
			userID, householdID); err != nil {
			return err
		}
		for _, p := range prefs {
			ids := p.ChannelIDs
			if ids == nil {
				ids = []domain.ID{}
			}
			raw, _ := json.Marshal(ids)
			_, err := r.db.Exec(ctx).ExecContext(ctx, `INSERT INTO notification_preferences (user_id, household_id, type, in_app,
				channel_ids, position) VALUES (?, ?, ?, ?, ?, ?)`, userID, householdID, p.Type, p.InApp, string(raw), slices.Index(event.Types, p.Type))
			if isForeignKey(err) {
				return domain.ErrNotFound
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// DeliveryRepository is the outbox of messages to channels.
type DeliveryRepository struct{ db *DB }

func NewDeliveryRepository(db *DB) *DeliveryRepository { return &DeliveryRepository{db: db} }

const deliveryColumns = `id, event_id, user_id, household_id, channel_id, type, title, body, url, data, occurred_at, status, attempts,
	next_attempt_at, last_error, created_at, updated_at, sent_at`

func scanDelivery(row interface{ Scan(...any) error }) (notification.Delivery, error) {
	var (
		d                                notification.Delivery
		data                             string
		occurred, next, created, updated int64
		sent                             sql.NullInt64
	)
	if err := row.Scan(&d.ID, &d.EventID, &d.UserID, &d.HouseholdID, &d.ChannelID, &d.Type, &d.Title, &d.Body, &d.URL, &data, &occurred,
		&d.Status, &d.Attempts, &next, &d.LastError, &created, &updated, &sent); err != nil {
		return notification.Delivery{}, err
	}
	if err := json.Unmarshal([]byte(data), &d.Data); err != nil {
		return notification.Delivery{}, fmt.Errorf("sqlite: data of delivery %s: %w", d.ID, err)
	}
	if d.Data == nil {
		d.Data = map[string]string{}
	}
	d.OccurredAt, d.NextAttemptAt, d.CreatedAt, d.UpdatedAt, d.SentAt = fromMicros(occurred), fromMicros(next), fromMicros(created),
		fromMicros(updated), fromNullMicros(sent)
	return d, nil
}

func (r *DeliveryRepository) Create(ctx context.Context, d notification.Delivery) (bool, error) {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `INSERT INTO notification_deliveries (`+deliveryColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (event_id, user_id, channel_id) DO NOTHING`,
		d.ID, d.EventID, d.UserID, d.HouseholdID, d.ChannelID, d.Type, d.Title, d.Body, d.URL, configJSON(d.Data), toMicros(d.OccurredAt),
		d.Status, d.Attempts, toMicros(d.NextAttemptAt), d.LastError, toMicros(d.CreatedAt), toMicros(d.UpdatedAt), nullMicros(d.SentAt))
	switch {
	case isUnique(err, "notification_deliveries.id"):
		return false, domain.ErrConflict
	case isForeignKey(err):
		return false, domain.ErrNotFound
	case err != nil:
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (r *DeliveryRepository) ListDue(ctx context.Context, now time.Time, limit int) ([]notification.Delivery, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx, `SELECT `+deliveryColumns+` FROM notification_deliveries
		WHERE status = 'pending' AND next_attempt_at <= ? ORDER BY next_attempt_at, id LIMIT ?`, toMicros(now), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []notification.Delivery{}
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *DeliveryRepository) ByID(ctx context.Context, id domain.ID) (notification.Delivery, error) {
	d, err := scanDelivery(r.db.Query(ctx).QueryRowContext(ctx, `SELECT `+deliveryColumns+` FROM notification_deliveries WHERE id = ?`, id))
	return d, notFound(err)
}

func (r *DeliveryRepository) Update(ctx context.Context, d notification.Delivery) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `UPDATE notification_deliveries SET status = ?, attempts = ?, next_attempt_at = ?,
		last_error = ?, updated_at = ?, sent_at = ? WHERE id = ?`,
		d.Status, d.Attempts, toMicros(d.NextAttemptAt), d.LastError, toMicros(d.UpdatedAt), nullMicros(d.SentAt), d.ID)
	if err != nil {
		return err
	}
	return affected(res)
}

func (r *DeliveryRepository) DeleteFinishedBefore(ctx context.Context, before time.Time) (int, error) {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM notification_deliveries WHERE status <> 'pending' AND updated_at < ?`,
		toMicros(before))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}
