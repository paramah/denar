package rest

import (
	"context"

	"git.eiac.dev/tools/denar/internal/adapter/rest/api"
	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func (s *Server) GetBudgetMonth(ctx context.Context, request api.GetBudgetMonthRequestObject) (api.GetBudgetMonthResponseObject, error) {
	v, err := s.deps.GetBudgetMonth.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), request.Month)
	if err != nil {
		return nil, err
	}
	return api.GetBudgetMonth200JSONResponse(apiBudgetMonth(v)), nil
}

func (s *Server) ListEnvelopeActivity(ctx context.Context, request api.ListEnvelopeActivityRequestObject) (api.ListEnvelopeActivityResponseObject, error) {
	page, err := s.deps.ListEnvelopeActivity.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.EnvelopeId),
		usecase.ListEnvelopeActivityInput{Month: request.Month, Cursor: deref(request.Params.Cursor), Limit: optional(request.Params.Limit)})
	if err != nil {
		return nil, err
	}
	items := make([]api.Transaction, len(page.Items))
	for i, v := range page.Items {
		items[i] = apiTransaction(v)
	}
	return api.ListEnvelopeActivity200JSONResponse{Items: items, NextCursor: nullableString(page.NextCursor)}, nil
}

func (s *Server) SetAssignment(ctx context.Context, request api.SetAssignmentRequestObject) (api.SetAssignmentResponseObject, error) {
	v, err := s.deps.SetAssignment.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.EnvelopeId),
		usecase.SetAssignmentInput{Month: request.Month, Amount: moneyInput(request.Body.Amount)})
	if err != nil {
		return nil, err
	}
	return api.SetAssignment200JSONResponse(apiBudgetMonth(v)), nil
}

func (s *Server) MoveBetweenEnvelopes(ctx context.Context, request api.MoveBetweenEnvelopesRequestObject) (api.MoveBetweenEnvelopesResponseObject, error) {
	b := request.Body
	v, err := s.deps.MoveBetweenEnvelopes.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.MoveBetweenEnvelopesInput{
		Month: request.Month, FromEnvelopeID: b.FromEnvelopeId, ToEnvelopeID: b.ToEnvelopeId, Amount: moneyInput(b.Amount)})
	if err != nil {
		return nil, err
	}
	return api.MoveBetweenEnvelopes200JSONResponse(apiBudgetMonth(v)), nil
}

func (s *Server) AutoAssign(ctx context.Context, request api.AutoAssignRequestObject) (api.AutoAssignResponseObject, error) {
	res, err := s.deps.AutoAssign.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), request.Month)
	if err != nil {
		return nil, err
	}
	out := api.AutoAssign200JSONResponse{Month: apiBudgetMonth(res.Month)}
	out.Assigned = make([]struct {
		Amount     api.Money `json:"amount"`
		EnvelopeId string    `json:"envelope_id"`
	}, len(res.Assigned))
	for i, a := range res.Assigned {
		out.Assigned[i].EnvelopeId, out.Assigned[i].Amount = string(a.EnvelopeID), minor(a.Amount, res.Month.Currency)
	}
	return out, nil
}

func (s *Server) ListEnvelopeGroups(ctx context.Context, request api.ListEnvelopeGroupsRequestObject) (api.ListEnvelopeGroupsResponseObject, error) {
	list, err := s.deps.ListEnvelopeGroups.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId))
	if err != nil {
		return nil, err
	}
	items := make([]api.EnvelopeGroup, len(list))
	for i, g := range list {
		items[i] = apiEnvelopeGroup(g)
	}
	return api.ListEnvelopeGroups200JSONResponse{Items: items}, nil
}

func (s *Server) CreateEnvelopeGroup(ctx context.Context, request api.CreateEnvelopeGroupRequestObject) (api.CreateEnvelopeGroupResponseObject, error) {
	g, err := s.deps.CreateEnvelopeGroup.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId),
		usecase.CreateEnvelopeGroupInput{Name: request.Body.Name, Position: deref(request.Body.Position)})
	if err != nil {
		return nil, err
	}
	return api.CreateEnvelopeGroup201JSONResponse(apiEnvelopeGroup(g)), nil
}

func (s *Server) UpdateEnvelopeGroup(ctx context.Context, request api.UpdateEnvelopeGroupRequestObject) (api.UpdateEnvelopeGroupResponseObject, error) {
	g, err := s.deps.UpdateEnvelopeGroup.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.GroupId),
		usecase.UpdateEnvelopeGroupInput{Name: optional(request.Body.Name), Position: optional(request.Body.Position)})
	if err != nil {
		return nil, err
	}
	return api.UpdateEnvelopeGroup200JSONResponse(apiEnvelopeGroup(g)), nil
}

func (s *Server) DeleteEnvelopeGroup(ctx context.Context, request api.DeleteEnvelopeGroupRequestObject) (api.DeleteEnvelopeGroupResponseObject, error) {
	if err := s.deps.DeleteEnvelopeGroup.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.GroupId)); err != nil {
		return nil, err
	}
	return api.DeleteEnvelopeGroup204Response{}, nil
}

// baseCurrency is the household's base currency for answers about
// envelopes; the use cases already checked access.
func (s *Server) baseCurrency(ctx context.Context, householdID string) (string, error) {
	v, err := s.deps.GetHousehold.Do(ctx, principalFrom(ctx), domain.ID(householdID))
	if err != nil {
		return "", err
	}
	return v.Household.BaseCurrency, nil
}

func (s *Server) ListEnvelopes(ctx context.Context, request api.ListEnvelopesRequestObject) (api.ListEnvelopesResponseObject, error) {
	list, err := s.deps.ListEnvelopes.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId),
		usecase.ListEnvelopesInput{IncludeHidden: deref(request.Params.IncludeHidden)})
	if err != nil {
		return nil, err
	}
	base, err := s.baseCurrency(ctx, request.HouseholdId)
	if err != nil {
		return nil, err
	}
	items := make([]api.Envelope, len(list))
	for i, e := range list {
		items[i] = apiEnvelope(e, base)
	}
	return api.ListEnvelopes200JSONResponse{Items: items}, nil
}

func (s *Server) CreateEnvelope(ctx context.Context, request api.CreateEnvelopeRequestObject) (api.CreateEnvelopeResponseObject, error) {
	b := request.Body
	e, err := s.deps.CreateEnvelope.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.CreateEnvelopeInput{
		GroupID: b.GroupId, Name: b.Name, Position: deref(b.Position), Hidden: deref(b.Hidden), Goal: goalInput(b.Goal)})
	if err != nil {
		return nil, err
	}
	base, err := s.baseCurrency(ctx, request.HouseholdId)
	if err != nil {
		return nil, err
	}
	return api.CreateEnvelope201JSONResponse(apiEnvelope(e, base)), nil
}

func (s *Server) GetEnvelope(ctx context.Context, request api.GetEnvelopeRequestObject) (api.GetEnvelopeResponseObject, error) {
	e, err := s.deps.GetEnvelope.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.EnvelopeId))
	if err != nil {
		return nil, err
	}
	base, err := s.baseCurrency(ctx, request.HouseholdId)
	if err != nil {
		return nil, err
	}
	return api.GetEnvelope200JSONResponse(apiEnvelope(e, base)), nil
}

func (s *Server) UpdateEnvelope(ctx context.Context, request api.UpdateEnvelopeRequestObject) (api.UpdateEnvelopeResponseObject, error) {
	b := request.Body
	in := usecase.UpdateEnvelopeInput{GroupID: optional(b.GroupId), Name: optional(b.Name), Position: optional(b.Position),
		Hidden: optional(b.Hidden)}
	if b.Goal.IsSpecified() {
		if b.Goal.IsNull() {
			in.Goal = domain.Some[*usecase.GoalInput](nil)
		} else {
			g := b.Goal.MustGet()
			in.Goal = domain.Some(goalInput(&g))
		}
	}
	e, err := s.deps.UpdateEnvelope.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.EnvelopeId), in)
	if err != nil {
		return nil, err
	}
	base, err := s.baseCurrency(ctx, request.HouseholdId)
	if err != nil {
		return nil, err
	}
	return api.UpdateEnvelope200JSONResponse(apiEnvelope(e, base)), nil
}

func (s *Server) DeleteEnvelope(ctx context.Context, request api.DeleteEnvelopeRequestObject) (api.DeleteEnvelopeResponseObject, error) {
	if err := s.deps.DeleteEnvelope.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.EnvelopeId)); err != nil {
		return nil, err
	}
	return api.DeleteEnvelope204Response{}, nil
}
