package rest

import (
	"context"

	"git.eiac.dev/tools/denar/internal/adapter/rest/api"
	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func (s *Server) ListVendors(ctx context.Context, _ api.ListVendorsRequestObject) (api.ListVendorsResponseObject, error) {
	vs := s.deps.ListVendors.Do(ctx, principalFrom(ctx))
	out := api.ListVendors200JSONResponse{Items: make([]api.Vendor, len(vs))}
	for i, v := range vs {
		out.Items[i] = api.Vendor{Key: v.Key, Name: v.Name}
	}
	return out, nil
}

func (s *Server) ListSubscriptions(ctx context.Context, request api.ListSubscriptionsRequestObject) (api.ListSubscriptionsResponseObject, error) {
	p := request.Params
	list, err := s.deps.ListSubscriptions.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.ListSubscriptionsInput{
		Status: deref(p.Status), Vendor: deref(p.Vendor), WalletID: deref(p.WalletId)})
	if err != nil {
		return nil, err
	}
	out := api.ListSubscriptions200JSONResponse{Items: make([]api.Subscription, len(list))}
	for i, v := range list {
		out.Items[i] = apiSubscription(v)
	}
	return out, nil
}

func (s *Server) CreateSubscription(ctx context.Context, request api.CreateSubscriptionRequestObject) (api.CreateSubscriptionResponseObject, error) {
	b := request.Body
	in := usecase.CreateSubscriptionInput{Name: b.Name, Vendor: b.Vendor, ExternalRef: deref(b.ExternalRef), Price: moneyInput(b.Price),
		Cycle: cycleInput(b.Cycle), TrialEndsAt: fromAPIDate(b.TrialEndsAt), Status: deref(b.Status), EndsAt: fromAPIDate(b.EndsAt),
		WalletID: deref(b.WalletId), EnvelopeID: deref(b.EnvelopeId), ReminderOffsets: optional(b.ReminderOffsets),
		Note: deref(b.Note), ManageURL: deref(b.ManageUrl)}
	if b.MatchRule != nil {
		m := matchRuleInput(b.MatchRule)
		in.Match = &m
	}
	v, err := s.deps.CreateSubscription.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), in)
	if err != nil {
		return nil, err
	}
	return api.CreateSubscription201JSONResponse(apiSubscription(v)), nil
}

func (s *Server) GetSubscriptionSummary(ctx context.Context, request api.GetSubscriptionSummaryRequestObject) (api.GetSubscriptionSummaryResponseObject, error) {
	sum, err := s.deps.GetSubscriptionSummary.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId))
	if err != nil {
		return nil, err
	}
	return api.GetSubscriptionSummary200JSONResponse(apiSummary(sum)), nil
}

func (s *Server) ListUpcomingRenewals(ctx context.Context, request api.ListUpcomingRenewalsRequestObject) (api.ListUpcomingRenewalsResponseObject, error) {
	list, err := s.deps.ListUpcomingRenewals.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), optional(request.Params.Days))
	if err != nil {
		return nil, err
	}
	base, err := s.baseCurrency(ctx, request.HouseholdId)
	if err != nil {
		return nil, err
	}
	out := api.ListUpcomingRenewals200JSONResponse{Items: make([]api.UpcomingRenewal, len(list))}
	for i, u := range list {
		out.Items[i] = apiUpcoming(u, base)
	}
	return out, nil
}

func (s *Server) GetSubscription(ctx context.Context, request api.GetSubscriptionRequestObject) (api.GetSubscriptionResponseObject, error) {
	v, err := s.deps.GetSubscription.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.SubscriptionId))
	if err != nil {
		return nil, err
	}
	return api.GetSubscription200JSONResponse(apiSubscription(v)), nil
}

func (s *Server) UpdateSubscription(ctx context.Context, request api.UpdateSubscriptionRequestObject) (api.UpdateSubscriptionResponseObject, error) {
	b := request.Body
	in := usecase.UpdateSubscriptionInput{Name: optional(b.Name), Vendor: optional(b.Vendor), ExternalRef: optionalNullable(b.ExternalRef),
		TrialEndsAt: optionalNullableDate(b.TrialEndsAt), Status: optional(b.Status), EndsAt: optionalNullableDate(b.EndsAt),
		WalletID: optionalNullable(b.WalletId), EnvelopeID: optionalNullable(b.EnvelopeId), ReminderOffsets: optional(b.ReminderOffsets),
		Note: optional(b.Note), ManageURL: optionalNullable(b.ManageUrl)}
	if b.Cycle != nil {
		in.Cycle = domain.Some(cycleInput(*b.Cycle))
	}
	if b.MatchRule != nil {
		in.Match = domain.Some(matchRuleInput(b.MatchRule))
	}
	v, err := s.deps.UpdateSubscription.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.SubscriptionId), in)
	if err != nil {
		return nil, err
	}
	return api.UpdateSubscription200JSONResponse(apiSubscription(v)), nil
}

func (s *Server) DeleteSubscription(ctx context.Context, request api.DeleteSubscriptionRequestObject) (api.DeleteSubscriptionResponseObject, error) {
	if err := s.deps.DeleteSubscription.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.SubscriptionId)); err != nil {
		return nil, err
	}
	return api.DeleteSubscription204Response{}, nil
}

func (s *Server) ListSubscriptionPrices(ctx context.Context, request api.ListSubscriptionPricesRequestObject) (api.ListSubscriptionPricesResponseObject, error) {
	list, err := s.deps.ListSubscriptionPrices.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.SubscriptionId))
	if err != nil {
		return nil, err
	}
	out := api.ListSubscriptionPrices200JSONResponse{Items: make([]api.PricePoint, len(list))}
	for i, p := range list {
		out.Items[i] = apiPricePoint(p)
	}
	return out, nil
}

func (s *Server) ChangeSubscriptionPrice(ctx context.Context, request api.ChangeSubscriptionPriceRequestObject) (api.ChangeSubscriptionPriceResponseObject, error) {
	b := request.Body
	v, err := s.deps.ChangeSubscriptionPrice.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.SubscriptionId),
		usecase.ChangeSubscriptionPriceInput{EffectiveFrom: fromAPIDate(b.EffectiveFrom), Price: moneyInput(b.Price)})
	if err != nil {
		return nil, err
	}
	return api.ChangeSubscriptionPrice201JSONResponse(apiSubscription(v)), nil
}

func (s *Server) ListCharges(ctx context.Context, request api.ListChargesRequestObject) (api.ListChargesResponseObject, error) {
	p := request.Params
	page, err := s.deps.ListCharges.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.ListChargesInput{
		SubscriptionID: deref(p.SubscriptionId), Statuses: deref(p.Status), From: fromAPIDate(p.From), To: fromAPIDate(p.To),
		Cursor: deref(p.Cursor), Limit: optional(p.Limit)})
	if err != nil {
		return nil, err
	}
	out := api.ListCharges200JSONResponse{Items: make([]api.Charge, len(page.Items)), NextCursor: nullableString(page.NextCursor)}
	for i, c := range page.Items {
		out.Items[i] = apiCharge(c)
	}
	return out, nil
}

func (s *Server) GetCharge(ctx context.Context, request api.GetChargeRequestObject) (api.GetChargeResponseObject, error) {
	v, err := s.deps.GetCharge.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.ChargeId))
	if err != nil {
		return nil, err
	}
	return api.GetCharge200JSONResponse(apiCharge(v)), nil
}

func (s *Server) SuggestChargeMatches(ctx context.Context, request api.SuggestChargeMatchesRequestObject) (api.SuggestChargeMatchesResponseObject, error) {
	list, err := s.deps.SuggestTransactions.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.ChargeId),
		optional(request.Params.Limit))
	if err != nil {
		return nil, err
	}
	out := api.SuggestChargeMatches200JSONResponse{Items: make([]api.TransactionSuggestion, len(list))}
	for i, x := range list {
		out.Items[i] = api.TransactionSuggestion{Score: x.Score, Transaction: apiTransaction(x.Transaction)}
	}
	return out, nil
}

func (s *Server) MatchCharge(ctx context.Context, request api.MatchChargeRequestObject) (api.MatchChargeResponseObject, error) {
	v, err := s.deps.ConfirmChargeMatch.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.ChargeId),
		domain.ID(request.Body.TransactionId))
	if err != nil {
		return nil, err
	}
	return api.MatchCharge200JSONResponse(apiCharge(v)), nil
}

func (s *Server) UnmatchCharge(ctx context.Context, request api.UnmatchChargeRequestObject) (api.UnmatchChargeResponseObject, error) {
	v, err := s.deps.UnmatchCharge.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.ChargeId))
	if err != nil {
		return nil, err
	}
	return api.UnmatchCharge200JSONResponse(apiCharge(v)), nil
}

func (s *Server) PostCharge(ctx context.Context, request api.PostChargeRequestObject) (api.PostChargeResponseObject, error) {
	var in usecase.PostChargeInput
	if b := request.Body; b != nil {
		in = usecase.PostChargeInput{WalletID: deref(b.WalletId), Amount: moneyInputPtr(b.Amount), Date: fromAPIDate(b.Date),
			PayeeName: deref(b.PayeeName), Note: deref(b.Note)}
	}
	out, err := s.deps.PostCharge.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.ChargeId), in)
	if err != nil {
		return nil, err
	}
	return api.PostCharge201JSONResponse{Charge: apiCharge(out.Charge), Transaction: apiTransaction(out.Transaction)}, nil
}

func (s *Server) SkipCharge(ctx context.Context, request api.SkipChargeRequestObject) (api.SkipChargeResponseObject, error) {
	v, err := s.deps.SkipCharge.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.ChargeId))
	if err != nil {
		return nil, err
	}
	return api.SkipCharge200JSONResponse(apiCharge(v)), nil
}

func (s *Server) SuggestChargesForTransaction(ctx context.Context, request api.SuggestChargesForTransactionRequestObject) (api.SuggestChargesForTransactionResponseObject, error) {
	list, err := s.deps.SuggestCharges.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.TransactionId))
	if err != nil {
		return nil, err
	}
	out := api.SuggestChargesForTransaction200JSONResponse{Items: make([]api.ChargeSuggestion, len(list))}
	for i, x := range list {
		out.Items[i] = api.ChargeSuggestion{Score: x.Score, Charge: apiCharge(x.Charge)}
	}
	return out, nil
}
