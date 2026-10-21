package rest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"

	"git.eiac.dev/tools/denar/internal/adapter/rest/api"
	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// maxOptionsBytes caps the options part of a CSV upload.
const maxOptionsBytes = 64 << 10

// csvImportOptions is the JSON of the options part (CsvImportOptions).
type csvImportOptions struct {
	api.CsvParseOptions
	WalletID     string `json:"wallet_id"`
	PreviewLimit *int   `json:"preview_limit"`
}

// readCSVUpload reads the parts file and options of a CSV upload.
func readCSVUpload(r *multipart.Reader) (usecase.CSVImportInput, error) {
	var (
		in                  usecase.CSVImportInput
		gotFile, gotOptions bool
		opts                csvImportOptions
	)
	malformed := domain.ValidationError{Field: "body", Message: "must be multipart/form-data with the parts file and options"}
	for {
		part, err := r.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return in, domain.ValidationError{Field: "body", Message: "is too large"}
		}
		if err != nil {
			return in, malformed
		}
		switch part.FormName() {
		case "file":
			data, err := io.ReadAll(io.LimitReader(part, usecase.MaxImportFileBytes+1))
			if errors.As(err, &tooLarge) {
				return in, domain.ValidationError{Field: "file", Message: "is larger than 5 MiB"}
			}
			if err != nil {
				return in, malformed
			}
			in.Data, in.FileName, gotFile = data, part.FileName(), true
		case "options":
			raw, err := io.ReadAll(io.LimitReader(part, maxOptionsBytes+1))
			if err != nil {
				return in, malformed
			}
			if len(raw) > maxOptionsBytes {
				return in, domain.ValidationError{Field: "options", Message: "is too large"}
			}
			if err := json.Unmarshal(raw, &opts); err != nil {
				return in, domain.ValidationError{Field: "options", Message: "must be a JSON object of CsvImportOptions"}
			}
			gotOptions = true
		}
		part.Close()
	}
	switch {
	case !gotFile:
		return in, domain.ValidationError{Field: "file", Message: "is required"}
	case !gotOptions:
		return in, domain.ValidationError{Field: "options", Message: "is required"}
	case opts.WalletID == "":
		return in, domain.ValidationError{Field: "options.wallet_id", Message: "is required"}
	}
	in.WalletID, in.Options, in.PreviewLimit = opts.WalletID, csvOptions(opts.CsvParseOptions), optional(opts.PreviewLimit)
	return in, nil
}

func (s *Server) ListImportPresets(ctx context.Context, _ api.ListImportPresetsRequestObject) (api.ListImportPresetsResponseObject, error) {
	ps, err := s.deps.ListImportPresets.Do(ctx, principalFrom(ctx))
	if err != nil {
		return nil, err
	}
	out := api.ListImportPresets200JSONResponse{Items: make([]api.ImportPreset, len(ps))}
	for i, p := range ps {
		out.Items[i] = api.ImportPreset{Key: p.Key, Name: p.Name, Description: p.Description, Options: apiCSVOptions(p.Options)}
	}
	return out, nil
}

func (s *Server) PreviewCsvImport(ctx context.Context, request api.PreviewCsvImportRequestObject) (api.PreviewCsvImportResponseObject, error) {
	in, err := readCSVUpload(request.Body)
	if err != nil {
		return nil, err
	}
	res, err := s.deps.PreviewCSVImport.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), in)
	if err != nil {
		return nil, err
	}
	return api.PreviewCsvImport200JSONResponse(apiImportPreview(res)), nil
}

func (s *Server) ImportCsv(ctx context.Context, request api.ImportCsvRequestObject) (api.ImportCsvResponseObject, error) {
	in, err := readCSVUpload(request.Body)
	if err != nil {
		return nil, err
	}
	res, err := s.deps.ImportCSV.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), in)
	if err != nil {
		return nil, err
	}
	return api.ImportCsv201JSONResponse(apiImportBatch(res.Outcome.Batch)), nil
}

func (s *Server) ListImports(ctx context.Context, request api.ListImportsRequestObject) (api.ListImportsResponseObject, error) {
	p := request.Params
	page, err := s.deps.ListImports.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), usecase.ListImportsInput{
		WalletID: deref(p.WalletId), Cursor: deref(p.Cursor), Limit: optional(p.Limit)})
	if err != nil {
		return nil, err
	}
	out := api.ListImports200JSONResponse{Items: make([]api.ImportBatch, len(page.Items)), NextCursor: nullableString(page.NextCursor)}
	for i, b := range page.Items {
		out.Items[i] = apiImportBatch(b)
	}
	return out, nil
}

func (s *Server) GetImport(ctx context.Context, request api.GetImportRequestObject) (api.GetImportResponseObject, error) {
	b, err := s.deps.GetImport.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.ImportId))
	if err != nil {
		return nil, err
	}
	return api.GetImport200JSONResponse(apiImportBatch(b)), nil
}

func (s *Server) RevertImport(ctx context.Context, request api.RevertImportRequestObject) (api.RevertImportResponseObject, error) {
	b, err := s.deps.RevertImport.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId), domain.ID(request.ImportId))
	if err != nil {
		return nil, err
	}
	return api.RevertImport200JSONResponse(apiImportBatch(b)), nil
}

func (s *Server) ListConnectionProviders(ctx context.Context, _ api.ListConnectionProvidersRequestObject) (api.ListConnectionProvidersResponseObject, error) {
	ps, err := s.deps.ListConnectionProviders.Do(ctx, principalFrom(ctx))
	if err != nil {
		return nil, err
	}
	out := api.ListConnectionProviders200JSONResponse{Items: make([]api.ConnectionProvider, len(ps))}
	for i, p := range ps {
		out.Items[i] = api.ConnectionProvider{Key: p.Key, Name: p.Name, Kind: api.ConnectionProviderKind(p.Kind)}
	}
	return out, nil
}

func (s *Server) ListConnections(ctx context.Context, request api.ListConnectionsRequestObject) (api.ListConnectionsResponseObject, error) {
	cs, err := s.deps.ListConnections.Do(ctx, principalFrom(ctx), domain.ID(request.HouseholdId))
	if err != nil {
		return nil, err
	}
	out := api.ListConnections200JSONResponse{Items: make([]api.Connection, len(cs))}
	for i, c := range cs {
		out.Items[i] = apiConnection(c)
	}
	return out, nil
}
