package rest

import (
	"github.com/oapi-codegen/nullable"

	"github.com/paramah/denar/internal/adapter/rest/api"
	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/imports"
	"github.com/paramah/denar/internal/usecase"
)

func refs(p *api.ColumnRefs) []string {
	if p == nil {
		return nil
	}
	return *p
}

func csvOptions(o api.CsvParseOptions) usecase.CSVOptions {
	out := usecase.CSVOptions{Preset: deref(o.Preset), Delimiter: deref(o.Delimiter), Encoding: deref(o.Encoding),
		DateFormat: deref(o.DateFormat), Decimal: deref(o.DecimalSeparator), SkipRows: deref(o.SkipRows), HasHeader: o.HasHeader,
		InvertAmount: o.InvertAmount, AcceptStates: deref(o.AcceptStates)}
	if c := o.Columns; c != nil {
		out.Columns = usecase.CSVColumns{Date: refs(c.Date), Amount: refs(c.Amount), Debit: refs(c.Debit), Credit: refs(c.Credit),
			ExternalID: refs(c.ExternalId), Currency: refs(c.Currency), Fee: refs(c.Fee), State: refs(c.State), Payee: refs(c.Payee),
			Memo: refs(c.Memo)}
	}
	return out
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func refsPtr(s []string) *api.ColumnRefs {
	if len(s) == 0 {
		return nil
	}
	out := api.ColumnRefs(append([]string{}, s...))
	return &out
}

func apiCSVOptions(o usecase.CSVOptions) api.CsvParseOptions {
	out := api.CsvParseOptions{Preset: strPtr(o.Preset), Delimiter: strPtr(o.Delimiter), Encoding: strPtr(o.Encoding),
		DateFormat: strPtr(o.DateFormat), DecimalSeparator: strPtr(o.Decimal), HasHeader: o.HasHeader, InvertAmount: o.InvertAmount}
	if o.SkipRows > 0 {
		out.SkipRows = &o.SkipRows
	}
	if len(o.AcceptStates) > 0 {
		states := append([]string{}, o.AcceptStates...)
		out.AcceptStates = &states
	}
	c := o.Columns
	out.Columns = &api.CsvColumns{Date: refsPtr(c.Date), Amount: refsPtr(c.Amount), Debit: refsPtr(c.Debit), Credit: refsPtr(c.Credit),
		ExternalId: refsPtr(c.ExternalID), Currency: refsPtr(c.Currency), Fee: refsPtr(c.Fee), State: refsPtr(c.State),
		Payee: refsPtr(c.Payee), Memo: refsPtr(c.Memo)}
	return out
}

func apiRowErrors(errs []imports.RowError) []api.ImportRowError {
	out := make([]api.ImportRowError, len(errs))
	for i, e := range errs {
		out[i] = api.ImportRowError{Row: e.Row, Field: e.Field, Message: e.Message}
	}
	return out
}

func apiActor(a domain.Actor) api.Actor {
	return api.Actor{Kind: api.ActorKind(a.Kind), Id: string(a.ID)}
}

func apiImportBatch(b imports.Batch) api.ImportBatch {
	out := api.ImportBatch{Id: string(b.ID), WalletId: string(b.WalletID), Source: api.ImportBatchSource(b.Source), SourceRef: b.SourceRef,
		Preset: nullableString(b.Preset), Status: api.ImportBatchStatus(b.Status), TotalRows: b.Rows, CreatedCount: b.Created,
		DuplicateCount: b.Duplicates, ErrorCount: b.Failed, MatchedCount: b.Matched, RevertedCount: b.Reverted,
		Errors: apiRowErrors(b.Errors), CreatedBy: apiActor(b.CreatedBy), CreatedAt: b.CreatedAt, RevertedAt: nullableTime(b.RevertedAt),
		RevertedBy: nullable.NewNullNullable[api.Actor]()}
	if !b.RevertedBy.IsZero() {
		out.RevertedBy = nullable.NewNullableWithValue(apiActor(b.RevertedBy))
	}
	return out
}

func apiImportPreview(r usecase.CSVImportResult) api.ImportPreview {
	b := r.Outcome.Batch
	out := api.ImportPreview{WalletId: string(b.WalletID), Preset: b.Preset, TotalRows: b.Rows, NewCount: b.Created,
		DuplicateCount: b.Duplicates, ErrorCount: b.Failed, Errors: apiRowErrors(b.Errors), Rows: make([]api.ImportPreviewRow, len(r.Outcome.Preview)),
		Detected: api.ImportDetected{Delimiter: r.Parse.Delimiter, Encoding: r.Parse.Encoding, HeaderLine: nullable.NewNullNullable[int](),
			Headers: append([]string{}, r.Parse.Headers...)}}
	if r.Parse.HeaderLine > 0 {
		out.Detected.HeaderLine = nullable.NewNullableWithValue(r.Parse.HeaderLine)
	}
	for i, row := range r.Outcome.Preview {
		out.Rows[i] = api.ImportPreviewRow{Row: row.Row, Date: apiDate(row.Date), Amount: minor(row.Amount, row.Currency),
			PayeeRaw: row.PayeeRaw, Memo: row.Memo, ExternalId: row.ExternalID, Kind: api.ImportPreviewRowKind(row.Kind),
			Status: api.ImportPreviewRowStatus(row.Status), PayeeId: nullableID(row.PayeeID), PayeeName: nullableString(row.PayeeName),
			EnvelopeId: nullableID(row.EnvelopeID)}
	}
	return out
}

func apiConnection(v usecase.ConnectionView) api.Connection {
	c := v.Connection
	out := api.Connection{Id: string(c.ID), Provider: c.Provider, ProviderName: v.ProviderName, Kind: api.ConnectionKind(c.Kind),
		Status: api.ConnectionStatus(c.Status), DisplayName: c.DisplayName, HasCredentials: v.HasCredentials,
		LastSyncAt: nullableTime(c.LastSyncAt), LastError: nullableString(c.LastError), CreatedAt: c.CreatedAt,
		Accounts: make([]api.ConnectionAccount, len(v.Accounts))}
	for i, a := range v.Accounts {
		out.Accounts[i] = api.ConnectionAccount{ExternalId: a.ExternalID, Name: a.Name, Currency: nullableString(a.Currency),
			WalletId: nullableID(a.WalletID)}
	}
	return out
}
