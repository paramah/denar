package rest

import (
	"context"

	"github.com/paramah/denar/internal/adapter/rest/api"
	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
)

func (s *Server) ListWallets(ctx context.Context, request api.ListWalletsRequestObject) (api.ListWalletsResponseObject, error) {
	list, err := s.deps.ListWallets.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId),
		usecase.ListWalletsInput{IncludeArchived: deref(request.Params.IncludeArchived)})
	if err != nil {
		return nil, err
	}
	items := make([]api.Wallet, len(list))
	for i, v := range list {
		items[i] = apiWallet(v)
	}
	return api.ListWallets200JSONResponse{Items: items}, nil
}

func (s *Server) CreateWallet(ctx context.Context, request api.CreateWalletRequestObject) (api.CreateWalletResponseObject, error) {
	b := request.Body
	onBudget := true
	if b.OnBudget != nil {
		onBudget = *b.OnBudget
	}
	v, err := s.deps.CreateWallet.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.CreateWalletInput{
		Name: b.Name, Type: b.Type, Currency: b.Currency, OnBudget: onBudget, Visibility: deref(b.Visibility),
		MemberIDs: deref(b.MemberIds), OpeningBalance: moneyInputPtr(b.OpeningBalance), OpenedOn: fromAPIDate(b.OpenedOn),
	})
	if err != nil {
		return nil, err
	}
	return api.CreateWallet201JSONResponse(apiWallet(v)), nil
}

func (s *Server) GetWallet(ctx context.Context, request api.GetWalletRequestObject) (api.GetWalletResponseObject, error) {
	v, err := s.deps.GetWallet.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.WalletId))
	if err != nil {
		return nil, err
	}
	return api.GetWallet200JSONResponse(apiWallet(v)), nil
}

func (s *Server) UpdateWallet(ctx context.Context, request api.UpdateWalletRequestObject) (api.UpdateWalletResponseObject, error) {
	b := request.Body
	v, err := s.deps.UpdateWallet.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.WalletId),
		usecase.UpdateWalletInput{Name: optional(b.Name), Type: optional(b.Type), OnBudget: optional(b.OnBudget),
			Visibility: optional(b.Visibility), MemberIDs: optional(b.MemberIds), Archived: optional(b.Archived)})
	if err != nil {
		return nil, err
	}
	return api.UpdateWallet200JSONResponse(apiWallet(v)), nil
}

func (s *Server) ListPayees(ctx context.Context, request api.ListPayeesRequestObject) (api.ListPayeesResponseObject, error) {
	list, err := s.deps.ListPayees.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId))
	if err != nil {
		return nil, err
	}
	items := make([]api.Payee, len(list))
	for i, p := range list {
		items[i] = apiPayee(p)
	}
	return api.ListPayees200JSONResponse{Items: items}, nil
}

func (s *Server) CreatePayee(ctx context.Context, request api.CreatePayeeRequestObject) (api.CreatePayeeResponseObject, error) {
	b := request.Body
	p, err := s.deps.CreatePayee.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.CreatePayeeInput{
		Name: b.Name, Patterns: deref(b.Patterns), DefaultEnvelopeID: deref(b.DefaultEnvelopeId)})
	if err != nil {
		return nil, err
	}
	return api.CreatePayee201JSONResponse(apiPayee(p)), nil
}

func (s *Server) UpdatePayee(ctx context.Context, request api.UpdatePayeeRequestObject) (api.UpdatePayeeResponseObject, error) {
	b := request.Body
	p, err := s.deps.UpdatePayee.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.PayeeId),
		usecase.UpdatePayeeInput{Name: optional(b.Name), Patterns: optional(b.Patterns), DefaultEnvelopeID: optionalNullable(b.DefaultEnvelopeId)})
	if err != nil {
		return nil, err
	}
	return api.UpdatePayee200JSONResponse(apiPayee(p)), nil
}

func (s *Server) DeletePayee(ctx context.Context, request api.DeletePayeeRequestObject) (api.DeletePayeeResponseObject, error) {
	if err := s.deps.DeletePayee.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.PayeeId)); err != nil {
		return nil, err
	}
	return api.DeletePayee204Response{}, nil
}

func (s *Server) QuoteFxRate(ctx context.Context, request api.QuoteFxRateRequestObject) (api.QuoteFxRateResponseObject, error) {
	q, err := s.deps.QuoteFxRate.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.QuoteFxRateInput{
		Currency: request.Params.Currency, Date: fromAPIDate(request.Params.Date)})
	if err != nil {
		return nil, err
	}
	return api.QuoteFxRate200JSONResponse(apiFxQuote(q)), nil
}

func (s *Server) ListFxOverrides(ctx context.Context, request api.ListFxOverridesRequestObject) (api.ListFxOverridesResponseObject, error) {
	list, err := s.deps.ListFxOverrides.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId))
	if err != nil {
		return nil, err
	}
	items := make([]api.FxOverride, len(list))
	for i, o := range list {
		items[i] = apiFxOverride(o)
	}
	return api.ListFxOverrides200JSONResponse{Items: items}, nil
}

func (s *Server) SetFxOverride(ctx context.Context, request api.SetFxOverrideRequestObject) (api.SetFxOverrideResponseObject, error) {
	b := request.Body
	o, err := s.deps.SetFxOverride.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.SetFxOverrideInput{
		Currency: b.Currency, Date: fromAPIDate(&b.Date), Rate: b.Rate})
	if err != nil {
		return nil, err
	}
	return api.SetFxOverride200JSONResponse(apiFxOverride(o)), nil
}
