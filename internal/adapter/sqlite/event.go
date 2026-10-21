package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/event"
	"github.com/paramah/denar/internal/usecase"
)

var _ usecase.EventOutbox = (*EventOutbox)(nil)

// EventOutbox keeps domain events in domain_events; the unique dedupe key
// of a household makes appending idempotent.
type EventOutbox struct{ db *DB }

func NewEventOutbox(db *DB) *EventOutbox { return &EventOutbox{db: db} }

func (o *EventOutbox) Append(ctx context.Context, e event.Event) (bool, error) {
	payload := e.Payload
	if payload == nil {
		payload = map[string]string{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}
	res, err := o.db.Exec(ctx).ExecContext(ctx, `INSERT INTO domain_events (id, household_id, type, resource_type, resource_id,
		dedupe_key, payload, occurred_at, dispatched_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (household_id, dedupe_key) DO NOTHING`,
		e.ID, e.HouseholdID, e.Type, e.ResourceType, e.ResourceID, e.DedupeKey, string(raw), toMicros(e.OccurredAt), nullMicros(e.DispatchedAt))
	if err != nil {
		if isUnique(err, "domain_events.id") {
			return false, domain.ErrConflict
		}
		if isForeignKey(err) {
			return false, domain.ErrNotFound
		}
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (o *EventOutbox) ListPending(ctx context.Context, limit int) ([]event.Event, error) {
	rows, err := o.db.Query(ctx).QueryContext(ctx, `SELECT id, household_id, type, resource_type, resource_id, dedupe_key, payload,
		occurred_at, dispatched_at FROM domain_events WHERE dispatched_at IS NULL ORDER BY id LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []event.Event{}
	for rows.Next() {
		var (
			e          event.Event
			payload    string
			occurred   int64
			dispatched sql.NullInt64
		)
		if err := rows.Scan(&e.ID, &e.HouseholdID, &e.Type, &e.ResourceType, &e.ResourceID, &e.DedupeKey, &payload, &occurred, &dispatched); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(payload), &e.Payload); err != nil {
			return nil, fmt.Errorf("sqlite: payload of event %s: %w", e.ID, err)
		}
		e.OccurredAt, e.DispatchedAt = fromMicros(occurred), fromNullMicros(dispatched)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (o *EventOutbox) MarkDispatched(ctx context.Context, id domain.ID, at time.Time) error {
	res, err := o.db.Exec(ctx).ExecContext(ctx, `UPDATE domain_events SET dispatched_at = ? WHERE id = ?`, toMicros(at), id)
	if err != nil {
		return err
	}
	return affected(res)
}

func (o *EventOutbox) DeleteDispatchedBefore(ctx context.Context, before time.Time) (int, error) {
	res, err := o.db.Exec(ctx).ExecContext(ctx, `DELETE FROM domain_events WHERE dispatched_at IS NOT NULL AND dispatched_at < ?`, toMicros(before))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}
