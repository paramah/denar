package rest

import (
	"context"
	"net/http"

	"git.eiac.dev/tools/denar/internal/adapter/rest/api"
	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func (s *Server) ListHouseholds(ctx context.Context, _ api.ListHouseholdsRequestObject) (api.ListHouseholdsResponseObject, error) {
	list, err := s.deps.ListHouseholds.Do(ctx, principalFrom(ctx))
	if err != nil {
		return nil, err
	}
	return api.ListHouseholds200JSONResponse{Items: apiHouseholds(list)}, nil
}

func (s *Server) CreateHousehold(ctx context.Context, request api.CreateHouseholdRequestObject) (api.CreateHouseholdResponseObject, error) {
	b := request.Body
	v, err := s.deps.CreateHousehold.Do(ctx, principalFrom(ctx), usecase.CreateHouseholdInput{
		Name: b.Name, BaseCurrency: b.BaseCurrency, Timezone: b.Timezone, MonthStartDay: b.MonthStartDay})
	if err != nil {
		return nil, err
	}
	return api.CreateHousehold201JSONResponse(apiHousehold(v)), nil
}

func (s *Server) GetHousehold(ctx context.Context, request api.GetHouseholdRequestObject) (api.GetHouseholdResponseObject, error) {
	v, err := s.deps.GetHousehold.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId))
	if err != nil {
		return nil, err
	}
	return api.GetHousehold200JSONResponse(apiHousehold(v)), nil
}

func (s *Server) UpdateHousehold(ctx context.Context, request api.UpdateHouseholdRequestObject) (api.UpdateHouseholdResponseObject, error) {
	b := request.Body
	v, err := s.deps.UpdateHousehold.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.UpdateHouseholdInput{
		Name: optional(b.Name), Timezone: optional(b.Timezone), MonthStartDay: optional(b.MonthStartDay)})
	if err != nil {
		return nil, err
	}
	return api.UpdateHousehold200JSONResponse(apiHousehold(v)), nil
}

func (s *Server) DeleteHousehold(ctx context.Context, request api.DeleteHouseholdRequestObject) (api.DeleteHouseholdResponseObject, error) {
	if err := s.deps.DeleteHousehold.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId)); err != nil {
		return nil, err
	}
	return api.DeleteHousehold204Response{}, nil
}

func (s *Server) ListMembers(ctx context.Context, request api.ListMembersRequestObject) (api.ListMembersResponseObject, error) {
	list, err := s.deps.ListMembers.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId))
	if err != nil {
		return nil, err
	}
	items := make([]api.Member, len(list))
	for i, m := range list {
		items[i] = apiMember(m)
	}
	return api.ListMembers200JSONResponse{Items: items}, nil
}

func (s *Server) UpdateMember(ctx context.Context, request api.UpdateMemberRequestObject) (api.UpdateMemberResponseObject, error) {
	v, err := s.deps.UpdateMemberRole.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.UserId),
		membership.Role(request.Body.Role))
	if err != nil {
		return nil, err
	}
	return api.UpdateMember200JSONResponse(apiMember(v)), nil
}

func (s *Server) RemoveMember(ctx context.Context, request api.RemoveMemberRequestObject) (api.RemoveMemberResponseObject, error) {
	if err := s.deps.RemoveMember.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.UserId)); err != nil {
		return nil, err
	}
	return api.RemoveMember204Response{}, nil
}

func (s *Server) ListInvitations(ctx context.Context, request api.ListInvitationsRequestObject) (api.ListInvitationsResponseObject, error) {
	list, err := s.deps.ListInvitations.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId))
	if err != nil {
		return nil, err
	}
	items := make([]api.Invitation, len(list))
	for i, v := range list {
		items[i] = apiInvitation(v)
	}
	return api.ListInvitations200JSONResponse{Items: items}, nil
}

var noStore = "no-store"

func (s *Server) CreateInvitation(ctx context.Context, request api.CreateInvitationRequestObject) (api.CreateInvitationResponseObject, error) {
	inv, err := s.deps.CreateInvitation.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.CreateInvitationInput{
		Email: request.Body.Email, Role: membership.Role(request.Body.Role), SendEmail: deref(request.Body.SendEmail)})
	if err != nil {
		return nil, err
	}
	body := api.InvitationCreated{
		Invitation: apiInvitation(inv.InvitationView), Token: inv.Token,
		Url: s.deps.BaseURL + "/invite#" + inv.Token, EmailSent: &inv.EmailSent,
	}
	if inv.EmailError != "" {
		body.EmailError = nullableString(inv.EmailError)
	}
	return api.CreateInvitation201JSONResponse{
		Body:    body,
		Headers: api.CreateInvitation201ResponseHeaders{CacheControl: &noStore},
	}, nil
}

func (s *Server) RevokeInvitation(ctx context.Context, request api.RevokeInvitationRequestObject) (api.RevokeInvitationResponseObject, error) {
	if err := s.deps.RevokeInvitation.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.InvitationId)); err != nil {
		return nil, err
	}
	return api.RevokeInvitation204Response{}, nil
}

func (s *Server) PreviewInvitation(ctx context.Context, request api.PreviewInvitationRequestObject) (api.PreviewInvitationResponseObject, error) {
	p, err := s.deps.PreviewInvitation.Do(ctx, request.Body.Token)
	if err != nil {
		return nil, err
	}
	return api.PreviewInvitation200JSONResponse{
		HouseholdName: p.HouseholdName, Email: p.Email, Role: api.Role(p.Role), ExpiresAt: p.ExpiresAt, AccountExists: p.AccountExists,
	}, nil
}

func (s *Server) AcceptInvitation(ctx context.Context, request api.AcceptInvitationRequestObject) (api.AcceptInvitationResponseObject, error) {
	v, err := s.deps.AcceptInvitation.Do(ctx, principalFrom(ctx), request.Body.Token)
	if err != nil {
		return nil, err
	}
	return api.AcceptInvitation200JSONResponse(apiHousehold(v)), nil
}

// registered answers a registration with the account and its session.
type registered struct {
	body   api.Registered
	cookie *http.Cookie
}

func (r registered) VisitRegisterWithInvitationResponse(w http.ResponseWriter) error {
	http.SetCookie(w, r.cookie)
	w.Header().Set("Cache-Control", "no-store")
	return api.RegisterWithInvitation201JSONResponse(r.body).VisitRegisterWithInvitationResponse(w)
}

func (s *Server) RegisterWithInvitation(ctx context.Context, request api.RegisterWithInvitationRequestObject) (api.RegisterWithInvitationResponseObject, error) {
	b := request.Body
	reg, err := s.deps.RegisterWithInvitation.Do(ctx, usecase.RegisterWithInvitationInput{Token: b.Token, Name: b.Name, Password: b.Password})
	if err != nil {
		return nil, err
	}
	return registered{
		body:   api.Registered{User: apiUser(reg.User), Household: apiHousehold(reg.Household)},
		cookie: sessionCookie(s.deps, reg.Token),
	}, nil
}
