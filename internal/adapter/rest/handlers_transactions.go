package rest

import (
	"context"

	"git.eiac.dev/tools/denar/internal/adapter/rest/api"
	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func (s *Server) SearchTransactions(ctx context.Context, request api.SearchTransactionsRequestObject) (api.SearchTransactionsResponseObject, error) {
	p := request.Params
	page, err := s.deps.SearchTransactions.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.SearchTransactionsInput{
		WalletID: deref(p.WalletId), PayeeID: deref(p.PayeeId), EnvelopeID: deref(p.EnvelopeId), From: fromAPIDate(p.From),
		To: fromAPIDate(p.To), Text: deref(p.Q), Kind: deref(p.Kind), Status: deref(p.Status), Tag: deref(p.Tag),
		Cursor: deref(p.Cursor), Limit: optional(p.Limit),
	})
	if err != nil {
		return nil, err
	}
	items := make([]api.Transaction, len(page.Items))
	for i, v := range page.Items {
		items[i] = apiTransaction(v)
	}
	out := api.SearchTransactions200JSONResponse{Items: items, NextCursor: nullableString(page.NextCursor)}
	return out, nil
}

func (s *Server) CreateTransaction(ctx context.Context, request api.CreateTransactionRequestObject) (api.CreateTransactionResponseObject, error) {
	b := request.Body
	v, err := s.deps.RecordTransaction.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.RecordTransactionInput{
		WalletID: b.WalletId, Date: fromAPIDate(b.Date), Amount: moneyInput(b.Amount), PayeeID: deref(b.PayeeId),
		PayeeName: deref(b.PayeeName), EnvelopeID: deref(b.EnvelopeId), Kind: deref(b.Kind), Status: deref(b.Status),
		Tags: deref(b.Tags), Note: deref(b.Note), ExternalID: deref(b.ExternalId), Splits: splitInputs(b.Splits),
	})
	if err != nil {
		return nil, err
	}
	return api.CreateTransaction201JSONResponse(apiTransaction(v)), nil
}

func (s *Server) GetTransaction(ctx context.Context, request api.GetTransactionRequestObject) (api.GetTransactionResponseObject, error) {
	v, err := s.deps.GetTransaction.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.TransactionId))
	if err != nil {
		return nil, err
	}
	return api.GetTransaction200JSONResponse(apiTransaction(v)), nil
}

func (s *Server) UpdateTransaction(ctx context.Context, request api.UpdateTransactionRequestObject) (api.UpdateTransactionResponseObject, error) {
	b := request.Body
	in := usecase.UpdateTransactionInput{
		PayeeID: optionalNullable(b.PayeeId), PayeeName: optional(b.PayeeName), EnvelopeID: optionalNullable(b.EnvelopeId),
		Kind: optional(b.Kind), Status: optional(b.Status), Tags: optional(b.Tags), Note: optional(b.Note),
		ExternalID: optionalNullable(b.ExternalId),
	}
	if b.Date != nil {
		in.Date = domain.Some(fromAPIDate(b.Date))
	}
	if b.Amount != nil {
		in.Amount = domain.Some(moneyInput(*b.Amount))
	}
	if b.Splits != nil {
		in.Splits = domain.Some(splitInputs(b.Splits))
	}
	v, err := s.deps.UpdateTransaction.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.TransactionId), in)
	if err != nil {
		return nil, err
	}
	return api.UpdateTransaction200JSONResponse(apiTransaction(v)), nil
}

func (s *Server) DeleteTransaction(ctx context.Context, request api.DeleteTransactionRequestObject) (api.DeleteTransactionResponseObject, error) {
	if err := s.deps.DeleteTransaction.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.TransactionId)); err != nil {
		return nil, err
	}
	return api.DeleteTransaction204Response{}, nil
}

func (s *Server) CreateTransfer(ctx context.Context, request api.CreateTransferRequestObject) (api.CreateTransferResponseObject, error) {
	b := request.Body
	v, err := s.deps.CreateTransfer.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.CreateTransferInput{
		FromWalletID: b.FromWalletId, ToWalletID: b.ToWalletId, Date: fromAPIDate(b.Date), Amount: moneyInput(b.Amount),
		ToAmount: moneyInputPtr(b.ToAmount), EnvelopeID: deref(b.EnvelopeId), Status: deref(b.Status), Tags: deref(b.Tags),
		Note: deref(b.Note),
	})
	if err != nil {
		return nil, err
	}
	return api.CreateTransfer201JSONResponse{From: apiTransaction(v.From), To: apiTransaction(v.To)}, nil
}
