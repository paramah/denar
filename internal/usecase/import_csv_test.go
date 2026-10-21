package usecase_test

import (
	"strings"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/usecase"
)

const csvFile = "2026-10-01;-10.00;Lidl\n2026-10-02;-20.00;Aldi;X-1\n!broken\n2026-10-03;-30.00;Kiosk;X-1\n"

func TestImportCSV(t *testing.T) {
	f := budgetFamily(t)
	in := usecase.CSVImportInput{WalletID: "w1", FileName: `C:\Users\jan\wyciąg październik.csv`, Data: []byte(csvFile),
		Options: usecase.CSVOptions{Preset: "mbank", Delimiter: ";"}, PreviewLimit: domain.Some(1)}
	pre, err := usecase.NewPreviewCSVImport(f.deps).Do(ctx, human(f.member), "h1", in)
	noErr(t, err)
	if b := pre.Outcome.Batch; b.ID != "" || b.Rows != 4 || b.Created != 2 || b.Duplicates != 1 || b.Failed != 1 || len(pre.Outcome.Preview) != 1 {
		t.Errorf("preview = %+v", pre.Outcome)
	}
	if pre.Parse.Delimiter != ";" || f.CSV.Last.Preset != "mbank" {
		t.Errorf("parse = %+v, options %+v", pre.Parse, f.CSV.Last)
	}
	got, err := usecase.NewImportCSV(f.deps).Do(ctx, human(f.member), "h1", in)
	noErr(t, err)
	b := got.Outcome.Batch
	if b.ID == "" || b.Created != 2 || b.SourceRef != "wyciąg październik.csv" || b.Preset != "mbank" || len(b.Errors) != 1 {
		t.Errorf("batch = %+v", b)
	}
	in.Options.Preset = ""
	got, err = usecase.NewImportCSV(f.deps).Do(ctx, human(f.member), "h1", in)
	noErr(t, err)
	if got.Outcome.Batch.Preset != "generic" || got.Outcome.Batch.Created != 0 {
		t.Errorf("second batch = %+v", got.Outcome.Batch)
	}
}

func TestImportCSVRefusals(t *testing.T) {
	f := budgetFamily(t)
	preview := usecase.NewPreviewCSVImport(f.deps)
	ok := usecase.CSVImportInput{WalletID: "w1", Data: []byte(csvFile)}
	for name, c := range map[string]struct {
		in    usecase.CSVImportInput
		field string
	}{
		"empty":         {usecase.CSVImportInput{WalletID: "w1"}, "file"},
		"too large":     {usecase.CSVImportInput{WalletID: "w1", Data: []byte(strings.Repeat("x", usecase.MaxImportFileBytes+1))}, "file"},
		"preview limit": {usecase.CSVImportInput{WalletID: "w1", Data: []byte(csvFile), PreviewLimit: domain.Some(101)}, "preview_limit"},
		"preset":        {usecase.CSVImportInput{WalletID: "w1", Data: []byte(csvFile), Options: usecase.CSVOptions{Preset: "citi"}}, "options.preset"},
		"no rows":       {usecase.CSVImportInput{WalletID: "w1", Data: []byte("\n\n")}, "file"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := preview.Do(ctx, human(f.member), "h1", c.in)
			wantField(t, err, c.field)
		})
	}
	// Who may import is checked before the file is read.
	_, err := preview.Do(ctx, human(f.outsider), "h1", usecase.CSVImportInput{WalletID: "w1"})
	wantErr(t, err, domain.ErrNotFound)
	_, err = usecase.NewImportCSV(f.deps).Do(ctx, human(f.viewer), "h1", ok)
	wantForbidden(t, err, domain.ForbiddenRole)
	b := f.addBot("b1", "h1", f.member, bot.ScopeTransactionsRead)
	_, err = usecase.NewImportCSV(f.deps).Do(ctx, asBot(b), "h1", ok)
	wantForbidden(t, err, domain.ForbiddenScope)
	_, err = preview.Do(ctx, human(f.member), "h1", usecase.CSVImportInput{WalletID: "nope", Data: []byte(csvFile)})
	wantErr(t, err, domain.ErrNotFound)
}

func TestListImportPresets(t *testing.T) {
	f := budgetFamily(t)
	b := f.addBot("b1", "h1", f.member, bot.ScopeWalletsRead)
	ps, err := usecase.NewListImportPresets(f.deps).Do(ctx, asBot(b))
	noErr(t, err)
	if len(ps) != 2 || ps[0].Key != "generic" {
		t.Errorf("presets = %+v", ps)
	}
	d := f.deps
	d.CSV = nil
	ps, err = usecase.NewListImportPresets(d).Do(ctx, human(f.member))
	noErr(t, err)
	if len(ps) != 0 {
		t.Errorf("without a parser = %+v", ps)
	}
}
