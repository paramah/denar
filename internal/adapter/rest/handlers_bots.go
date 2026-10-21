package rest

import (
	"context"
	"time"

	"github.com/paramah/denar/internal/adapter/rest/api"
	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
)

func (s *Server) ListBots(ctx context.Context, request api.ListBotsRequestObject) (api.ListBotsResponseObject, error) {
	list, err := s.deps.ListBots.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId))
	if err != nil {
		return nil, err
	}
	items := make([]api.Bot, len(list))
	for i, b := range list {
		items[i] = apiBot(b)
	}
	return api.ListBots200JSONResponse{Items: items}, nil
}

func (s *Server) CreateBot(ctx context.Context, request api.CreateBotRequestObject) (api.CreateBotResponseObject, error) {
	b := request.Body
	in := usecase.CreateBotInput{Name: b.Name, Scopes: b.Scopes, WalletIDs: deref(b.WalletIds)}
	if b.ExpiresAt.IsSpecified() && !b.ExpiresAt.IsNull() {
		in.ExpiresAt = b.ExpiresAt.MustGet()
	}
	created, err := s.deps.CreateBot.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), in)
	if err != nil {
		return nil, err
	}
	return api.CreateBot201JSONResponse{
		Body:    api.BotWithToken{Bot: apiBot(created.BotView), Token: created.Token},
		Headers: api.CreateBot201ResponseHeaders{CacheControl: &noStore},
	}, nil
}

func (s *Server) GetBot(ctx context.Context, request api.GetBotRequestObject) (api.GetBotResponseObject, error) {
	v, err := s.deps.GetBot.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.BotId))
	if err != nil {
		return nil, err
	}
	return api.GetBot200JSONResponse(apiBot(v)), nil
}

func (s *Server) UpdateBot(ctx context.Context, request api.UpdateBotRequestObject) (api.UpdateBotResponseObject, error) {
	b := request.Body
	in := usecase.UpdateBotInput{
		Name: optional(b.Name), Scopes: optional(b.Scopes), WalletIDs: optional(b.WalletIds), Disabled: optional(b.Disabled),
	}
	if b.ExpiresAt.IsSpecified() {
		var t time.Time
		if !b.ExpiresAt.IsNull() {
			t = b.ExpiresAt.MustGet()
		}
		in.ExpiresAt = domain.Some(t)
	}
	v, err := s.deps.UpdateBot.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.BotId), in)
	if err != nil {
		return nil, err
	}
	return api.UpdateBot200JSONResponse(apiBot(v)), nil
}

func (s *Server) DeleteBot(ctx context.Context, request api.DeleteBotRequestObject) (api.DeleteBotResponseObject, error) {
	if err := s.deps.DeleteBot.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.BotId)); err != nil {
		return nil, err
	}
	return api.DeleteBot204Response{}, nil
}

func (s *Server) RotateBotToken(ctx context.Context, request api.RotateBotTokenRequestObject) (api.RotateBotTokenResponseObject, error) {
	rotated, err := s.deps.RotateBotToken.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.BotId))
	if err != nil {
		return nil, err
	}
	return api.RotateBotToken200JSONResponse{
		Body:    api.BotWithToken{Bot: apiBot(rotated.BotView), Token: rotated.Token},
		Headers: api.RotateBotToken200ResponseHeaders{CacheControl: &noStore},
	}, nil
}

func (s *Server) ListAudit(ctx context.Context, request api.ListAuditRequestObject) (api.ListAuditResponseObject, error) {
	p := request.Params
	page, err := s.deps.ListAudit.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.ListAuditInput{
		ActorKind: string(deref(p.ActorKind)), ActorID: deref(p.ActorId), ResourceType: deref(p.ResourceType),
		Cursor: deref(p.Cursor), Limit: optional(p.Limit),
	})
	if err != nil {
		return nil, err
	}
	items := make([]api.AuditEntry, len(page.Items))
	for i, e := range page.Items {
		items[i] = apiAuditEntry(e)
	}
	out := api.ListAudit200JSONResponse{Items: items}
	if page.NextCursor != "" {
		out.NextCursor.Set(page.NextCursor)
	} else {
		out.NextCursor.SetNull()
	}
	return out, nil
}
