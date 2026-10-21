package usecase

import (
	"context"

	"git.eiac.dev/tools/denar/internal/domain/notification"
)

// PruneResult counts what one retention run removed.
type PruneResult struct {
	Notifications int
	Deliveries    int
	Events        int
}

// PruneNotifications removes read notifications after 180 days, finished
// deliveries after 30 and dispatched events after 400 (FR-552, AD-514).
type PruneNotifications struct{ d Deps }

func NewPruneNotifications(d Deps) *PruneNotifications { return &PruneNotifications{d: d} }

func (uc *PruneNotifications) Do(ctx context.Context) (PruneResult, error) {
	var r PruneResult
	var err error
	now := uc.d.Clock.Now()
	if r.Notifications, err = uc.d.Notifications.DeleteReadBefore(ctx, now.Add(-notification.ReadRetention)); err != nil {
		return r, err
	}
	if r.Deliveries, err = uc.d.Deliveries.DeleteFinishedBefore(ctx, now.Add(-notification.DeliveryRetention)); err != nil {
		return r, err
	}
	r.Events, err = uc.d.Events.DeleteDispatchedBefore(ctx, now.Add(-notification.EventRetention))
	return r, err
}

// NotificationJobResult counts what one run of the notification jobs did.
type NotificationJobResult struct {
	Dispatch DispatchResult
	Delivery DeliveryResult
}

// RunNotificationJobs dispatches pending events, then sends due
// deliveries, those just made included.
type RunNotificationJobs struct {
	dispatch *DispatchEvents
	deliver  *DeliverNotifications
}

func NewRunNotificationJobs(d Deps) *RunNotificationJobs {
	return &RunNotificationJobs{dispatch: NewDispatchEvents(d), deliver: NewDeliverNotifications(d)}
}

func (uc *RunNotificationJobs) Do(ctx context.Context) (NotificationJobResult, error) {
	var r NotificationJobResult
	var err error
	if r.Dispatch, err = uc.dispatch.Do(ctx); err != nil {
		return r, err
	}
	r.Delivery, err = uc.deliver.Do(ctx)
	return r, err
}
