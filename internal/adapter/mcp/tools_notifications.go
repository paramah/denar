package mcp

import (
	"context"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/usecase"
)

type listNotificationsIn struct {
	UnreadOnly bool   `json:"unread_only,omitempty" jsonschema:"only notifications not read yet"`
	Limit      int    `json:"limit,omitempty" jsonschema:"page size, 1 to 200; 50 by default"`
	Cursor     string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous page, with the same unread_only"`
}

type notificationOut struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Link      string    `json:"link"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
	Read      bool      `json:"read"`
}

type notificationsOut struct {
	Notifications []notificationOut `json:"notifications"`
	UnreadCount   int               `json:"unread_count"`
	NextCursor    *string           `json:"next_cursor"`
}

func (s *server) addNotificationTools(m *sdk.Server) {
	addTool(s, m, "list_notifications",
		"List the notifications of the person who created you in this household, newest first: renewals coming up, trials ending, "+
			"price changes, missed payments, overspent envelopes and goals due. Notifications of wallets outside your wallet list are "+
			"left out; you cannot mark them read. Needs notifications:read.",
		true, func(ctx context.Context, p usecase.Principal, hid domain.ID, in listNotificationsIn) (any, error) {
			page, err := s.uc.listNotifications.Do(ctx, p, usecase.ListNotificationsInput{HouseholdID: hid, UnreadOnly: in.UnreadOnly,
				Cursor: in.Cursor, Limit: optLimit(in.Limit)})
			if err != nil {
				return nil, err
			}
			unread, err := s.uc.countUnread.Do(ctx, p, hid)
			if err != nil {
				return nil, err
			}
			out := notificationsOut{Notifications: []notificationOut{}, UnreadCount: unread, NextCursor: optString(page.NextCursor)}
			for _, n := range page.Items {
				out.Notifications = append(out.Notifications, notificationOut{ID: string(n.ID), Type: string(n.Type), Title: n.Title,
					Body: n.Body, Link: n.Link, URL: s.deps.UseCases.BaseURL + n.Link, CreatedAt: n.CreatedAt, Read: n.Read()})
			}
			return out, nil
		})
}
