package rest

import (
	"context"

	"git.eiac.dev/tools/denar/internal/adapter/rest/api"
	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/event"
	"git.eiac.dev/tools/denar/internal/domain/notification"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func apiNotification(n notification.Notification) api.Notification {
	return api.Notification{Id: string(n.ID), HouseholdId: string(n.HouseholdID), Type: api.EventType(n.Type), Title: n.Title,
		Body: n.Body, Link: n.Link, ResourceType: n.ResourceType, ResourceId: string(n.ResourceID), CreatedAt: n.CreatedAt,
		ReadAt: nullableTime(n.ReadAt)}
}

func apiChannel(v usecase.ChannelView) api.Channel {
	config := api.ChannelConfig{}
	for k, val := range v.Config {
		config[k] = val
	}
	return api.Channel{Id: string(v.ID), Type: api.ChannelType(v.Type), Name: v.Name, Config: config, HasSecret: v.HasSecret,
		Enabled: v.Enabled, VerifiedAt: nullableTime(v.VerifiedAt), LastError: nullableString(v.LastError),
		LastErrorAt: nullableTime(v.LastErrorAt), CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}

func apiFields(fs []notification.Field) []api.ChannelField {
	out := make([]api.ChannelField, len(fs))
	for i, f := range fs {
		out[i] = api.ChannelField{Name: f.Name, Required: f.Required}
	}
	return out
}

func apiPreferences(prefs []notification.Preference) api.NotificationPreferences {
	out := api.NotificationPreferences{Items: make([]api.NotificationPreference, len(prefs))}
	for i, p := range prefs {
		ids := make([]string, len(p.ChannelIDs))
		for j, id := range p.ChannelIDs {
			ids[j] = string(id)
		}
		out.Items[i] = api.NotificationPreference{Type: api.EventType(p.Type), InApp: p.InApp, ChannelIds: ids}
	}
	return out
}

func (s *Server) ListNotifications(ctx context.Context, request api.ListNotificationsRequestObject) (api.ListNotificationsResponseObject, error) {
	p := request.Params
	page, err := s.deps.ListNotifications.Do(ctx, principalFrom(ctx), usecase.ListNotificationsInput{HouseholdID: domain.ID(deref(p.HouseholdId)),
		UnreadOnly: deref(p.Unread), Cursor: deref(p.Cursor), Limit: optional(p.Limit)})
	if err != nil {
		return nil, err
	}
	out := api.ListNotifications200JSONResponse{Items: make([]api.Notification, len(page.Items)), NextCursor: nullableString(page.NextCursor)}
	for i, n := range page.Items {
		out.Items[i] = apiNotification(n)
	}
	return out, nil
}

func (s *Server) CountUnreadNotifications(ctx context.Context, request api.CountUnreadNotificationsRequestObject) (api.CountUnreadNotificationsResponseObject, error) {
	n, err := s.deps.CountUnreadNotifications.Do(ctx, principalFrom(ctx), domain.ID(deref(request.Params.HouseholdId)))
	if err != nil {
		return nil, err
	}
	return api.CountUnreadNotifications200JSONResponse{Count: n}, nil
}

func (s *Server) MarkAllNotificationsRead(ctx context.Context, request api.MarkAllNotificationsReadRequestObject) (api.MarkAllNotificationsReadResponseObject, error) {
	var householdID string
	if request.Body != nil {
		householdID = deref(request.Body.HouseholdId)
	}
	n, err := s.deps.MarkAllNotificationsRead.Do(ctx, principalFrom(ctx), domain.ID(householdID))
	if err != nil {
		return nil, err
	}
	return api.MarkAllNotificationsRead200JSONResponse{Updated: n}, nil
}

func (s *Server) MarkNotificationRead(ctx context.Context, request api.MarkNotificationReadRequestObject) (api.MarkNotificationReadResponseObject, error) {
	n, err := s.deps.MarkNotificationRead.Do(ctx, principalFrom(ctx), domain.ID(request.NotificationId))
	if err != nil {
		return nil, err
	}
	return api.MarkNotificationRead200JSONResponse(apiNotification(n)), nil
}

func (s *Server) ListChannelTypes(ctx context.Context, _ api.ListChannelTypesRequestObject) (api.ListChannelTypesResponseObject, error) {
	types, err := s.deps.ListChannelTypes.Do(ctx, principalFrom(ctx))
	if err != nil {
		return nil, err
	}
	out := api.ListChannelTypes200JSONResponse{Items: make([]api.ChannelTypeInfo, len(types))}
	for i, t := range types {
		info := api.ChannelTypeInfo{Type: api.ChannelType(t.Type), Available: t.Available, ConfigFields: apiFields(t.Spec.Config),
			SecretFields: apiFields(t.Spec.Secrets)}
		info.UnavailableReason.SetNull()
		if t.UnavailableReason != "" {
			info.UnavailableReason.Set(api.ChannelTypeInfoUnavailableReason(t.UnavailableReason))
		}
		out.Items[i] = info
	}
	return out, nil
}

func (s *Server) ListChannels(ctx context.Context, _ api.ListChannelsRequestObject) (api.ListChannelsResponseObject, error) {
	list, err := s.deps.ListChannels.Do(ctx, principalFrom(ctx))
	if err != nil {
		return nil, err
	}
	out := api.ListChannels200JSONResponse{Items: make([]api.Channel, len(list))}
	for i, c := range list {
		out.Items[i] = apiChannel(c)
	}
	return out, nil
}

func (s *Server) CreateChannel(ctx context.Context, request api.CreateChannelRequestObject) (api.CreateChannelResponseObject, error) {
	b := request.Body
	in := usecase.CreateChannelInput{Type: notification.ChannelType(b.Type), Name: b.Name, Enabled: optional(b.Enabled)}
	if b.Config != nil {
		in.Config = *b.Config
	}
	if b.Secrets != nil {
		in.Secrets = *b.Secrets
	}
	v, err := s.deps.CreateChannel.Do(ctx, principalFrom(ctx), in)
	if err != nil {
		return nil, err
	}
	return api.CreateChannel201JSONResponse(apiChannel(v)), nil
}

func (s *Server) GetChannel(ctx context.Context, request api.GetChannelRequestObject) (api.GetChannelResponseObject, error) {
	v, err := s.deps.GetChannel.Do(ctx, principalFrom(ctx), domain.ID(request.ChannelId))
	if err != nil {
		return nil, err
	}
	return api.GetChannel200JSONResponse(apiChannel(v)), nil
}

func (s *Server) UpdateChannel(ctx context.Context, request api.UpdateChannelRequestObject) (api.UpdateChannelResponseObject, error) {
	b := request.Body
	in := usecase.UpdateChannelInput{Name: optional(b.Name), Enabled: optional(b.Enabled)}
	if b.Config != nil {
		in.Config = domain.Some(map[string]string(*b.Config))
	}
	if b.Secrets.IsSpecified() {
		if b.Secrets.IsNull() {
			in.Secrets = domain.Some[map[string]string](nil)
		} else {
			in.Secrets = domain.Some(b.Secrets.MustGet())
		}
	}
	v, err := s.deps.UpdateChannel.Do(ctx, principalFrom(ctx), domain.ID(request.ChannelId), in)
	if err != nil {
		return nil, err
	}
	return api.UpdateChannel200JSONResponse(apiChannel(v)), nil
}

func (s *Server) DeleteChannel(ctx context.Context, request api.DeleteChannelRequestObject) (api.DeleteChannelResponseObject, error) {
	if err := s.deps.DeleteChannel.Do(ctx, principalFrom(ctx), domain.ID(request.ChannelId)); err != nil {
		return nil, err
	}
	return api.DeleteChannel204Response{}, nil
}

func (s *Server) TestChannel(ctx context.Context, request api.TestChannelRequestObject) (api.TestChannelResponseObject, error) {
	r, err := s.deps.TestChannel.Do(ctx, principalFrom(ctx), domain.ID(request.ChannelId))
	if err != nil {
		return nil, err
	}
	return api.TestChannel200JSONResponse{Ok: r.OK, Error: nullableString(r.Error), TestedAt: r.TestedAt}, nil
}

func (s *Server) GetNotificationPreferences(ctx context.Context, request api.GetNotificationPreferencesRequestObject) (api.GetNotificationPreferencesResponseObject, error) {
	prefs, err := s.deps.GetNotificationPreferences.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId))
	if err != nil {
		return nil, err
	}
	return api.GetNotificationPreferences200JSONResponse(apiPreferences(prefs)), nil
}

func (s *Server) SetNotificationPreferences(ctx context.Context, request api.SetNotificationPreferencesRequestObject) (api.SetNotificationPreferencesResponseObject, error) {
	in := make([]notification.Preference, len(request.Body.Items))
	for i, p := range request.Body.Items {
		ids := make([]domain.ID, len(p.ChannelIds))
		for j, id := range p.ChannelIds {
			ids[j] = domain.ID(id)
		}
		in[i] = notification.Preference{Type: event.Type(p.Type), InApp: p.InApp, ChannelIDs: ids}
	}
	prefs, err := s.deps.SetNotificationPreferences.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), in)
	if err != nil {
		return nil, err
	}
	return api.SetNotificationPreferences200JSONResponse(apiPreferences(prefs)), nil
}
