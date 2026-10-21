package usecase

import (
	"context"
	"errors"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/imports"
)

// MaxImportFileBytes is the largest CSV file an import takes (FR-812).
const MaxImportFileBytes = 5 << 20

var errCSVUnavailable = errors.New("usecase: no CSV parser is configured")

// ListImportPresets lists the CSV presets (FR-811); any signed-in person
// or bot may read them.
type ListImportPresets struct{ d Deps }

func NewListImportPresets(d Deps) *ListImportPresets { return &ListImportPresets{d: d} }

func (uc *ListImportPresets) Do(_ context.Context, _ Principal) ([]CSVPreset, error) {
	if uc.d.CSV == nil {
		return []CSVPreset{}, nil
	}
	return uc.d.CSV.Presets(), nil
}

// CSVImportInput is a CSV file to import into a wallet.
type CSVImportInput struct {
	WalletID string
	FileName string
	Data     []byte
	Options  CSVOptions
	// PreviewLimit is how many rows a preview returns, 1 to 100.
	PreviewLimit domain.Optional[int]
}

// CSVImportResult is an import of a CSV file and how the file was read.
type CSVImportResult struct {
	Outcome ImportOutcome
	Parse   CSVParseResult
}

// fileName keeps the last element of a path, at most 200 characters.
func fileName(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	return truncateRunes(name, 200)
}

// csvImport checks the principal and the file, reads it and hands the
// rows to ImportTransactions.
func (d Deps) csvImport(ctx context.Context, p Principal, householdID domain.ID, in CSVImportInput, dryRun bool) (CSVImportResult, error) {
	if d.CSV == nil {
		return CSVImportResult{}, errCSVUnavailable
	}
	s, err := d.access().household(ctx, p, householdID)
	if err != nil {
		return CSVImportResult{}, err
	}
	if err := requireScope(s, bot.ScopeTransactionsWrite); err != nil {
		return CSVImportResult{}, err
	}
	if _, err := d.writableWallet(ctx, s, p, domain.ID(strings.TrimSpace(in.WalletID))); err != nil {
		return CSVImportResult{}, err
	}
	limit := DefaultPreviewRows
	if in.PreviewLimit.Set {
		if in.PreviewLimit.Value < 1 || in.PreviewLimit.Value > MaxPreviewRows {
			return CSVImportResult{}, domain.ValidationError{Field: "preview_limit", Message: "must be 1 to 100"}
		}
		limit = in.PreviewLimit.Value
	}
	switch {
	case len(in.Data) == 0:
		return CSVImportResult{}, domain.ValidationError{Field: "file", Message: "is empty"}
	case len(in.Data) > MaxImportFileBytes:
		return CSVImportResult{}, domain.ValidationError{Field: "file", Message: "is larger than 5 MiB"}
	}
	parsed, err := d.CSV.Parse(in.Data, in.Options)
	if err != nil {
		return CSVImportResult{}, err
	}
	preset := cmpOr(strings.TrimSpace(in.Options.Preset), "generic")
	out, err := NewImportTransactions(d).Do(ctx, p, householdID, ImportInput{WalletID: in.WalletID, Source: imports.SourceCSV,
		SourceRef: fileName(in.FileName), Preset: preset, Rows: parsed.Rows, RowErrors: parsed.Errors, DryRun: dryRun,
		PreviewLimit: limit})
	if err != nil {
		return CSVImportResult{}, err
	}
	return CSVImportResult{Outcome: out, Parse: parsed}, nil
}

// PreviewCSVImport reads a CSV file and says what importing it would do,
// storing nothing (FR-813).
type PreviewCSVImport struct{ d Deps }

func NewPreviewCSVImport(d Deps) *PreviewCSVImport { return &PreviewCSVImport{d: d} }

func (uc *PreviewCSVImport) Do(ctx context.Context, p Principal, householdID domain.ID, in CSVImportInput) (CSVImportResult, error) {
	return uc.d.csvImport(ctx, p, householdID, in, true)
}

// ImportCSV imports a CSV file into a wallet (FR-800, FR-810).
type ImportCSV struct{ d Deps }

func NewImportCSV(d Deps) *ImportCSV { return &ImportCSV{d: d} }

func (uc *ImportCSV) Do(ctx context.Context, p Principal, householdID domain.ID, in CSVImportInput) (CSVImportResult, error) {
	in.PreviewLimit = domain.Optional[int]{}
	return uc.d.csvImport(ctx, p, householdID, in, false)
}
