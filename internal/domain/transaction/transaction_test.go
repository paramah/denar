package transaction

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/paramah/denar/internal/domain"
)

func valid() Transaction {
	return Transaction{Date: domain.NewDate(2026, 10, 20), Amount: -1000, Currency: "PLN", Kind: Expense,
		Status: Cleared, Source: SourceManual}
}

func fields(err error) []string {
	var out []string
	var walk func(error)
	walk = func(e error) {
		var ve domain.ValidationError
		switch x := e.(type) {
		case interface{ Unwrap() []error }:
			for _, c := range x.Unwrap() {
				walk(c)
			}
		default:
			if errors.As(e, &ve) {
				out = append(out, ve.Field)
			}
		}
	}
	if err != nil {
		walk(err)
	}
	slices.Sort(out)
	return out
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Transaction)
		want   []string
	}{
		{"valid", func(*Transaction) {}, nil},
		{"refund is a positive expense", func(t *Transaction) { t.Amount = 500; t.EnvelopeID = "e1" }, nil},
		{"no date", func(t *Transaction) { t.Date = domain.Date{} }, []string{"date"}},
		{"bad enums", func(t *Transaction) { t.Kind, t.Status, t.Source = "x", "y", "z" }, []string{"kind", "source", "status"}},
		{"long note", func(t *Transaction) { t.Note = strings.Repeat("a", 1001) }, []string{"note"}},
		{"long external id", func(t *Transaction) { t.ExternalID = strings.Repeat("a", 201) }, []string{"external_id"}},
		{"too many tags", func(t *Transaction) {
			for i := range 21 {
				t.Tags = append(t.Tags, strings.Repeat("t", i+1))
			}
		}, []string{"tags"}},
		{"long tag", func(t *Transaction) { t.Tags = []string{strings.Repeat("t", 41)} }, []string{"tags"}},
		{"empty tag", func(t *Transaction) { t.Tags = []string{""} }, []string{"tags"}},
		{"income with envelope", func(t *Transaction) { t.Kind, t.EnvelopeID = Income, "e1" }, []string{"envelope_id"}},
		{"income split", func(t *Transaction) {
			t.Kind, t.Splits = Income, []Split{{Amount: -1000}}
		}, []string{"splits"}},
		{"transfer by hand", func(t *Transaction) { t.Kind = Transfer }, []string{"kind"}},
		{"transfer with payee", func(t *Transaction) {
			t.Kind, t.TransferPairID, t.TransferWalletID, t.PayeeID = Transfer, "p", "w2", "pay"
		}, []string{"payee_id"}},
		{"splits add up", func(t *Transaction) {
			t.Splits = []Split{{EnvelopeID: "e1", Amount: -600}, {EnvelopeID: "e2", Amount: -400}}
		}, nil},
		{"splits do not add up", func(t *Transaction) {
			t.Splits = []Split{{EnvelopeID: "e1", Amount: -600}, {Amount: -300}}
		}, []string{"splits"}},
		{"split with own envelope", func(t *Transaction) {
			t.EnvelopeID, t.Splits = "e1", []Split{{Amount: -1000}}
		}, []string{"envelope_id"}},
		{"split line envelope id too long", func(t *Transaction) {
			t.Splits = []Split{{EnvelopeID: domain.ID(strings.Repeat("e", 65)), Amount: -1000}}
		}, []string{"splits[0].envelope_id"}},
	}
	for _, tt := range tests {
		tx := valid()
		tt.change(&tx)
		if got := fields(tx.Normalize().Validate()); !slices.Equal(got, tt.want) {
			t.Errorf("%s: fields = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{" Food ", "food", "Trip"})
	if !slices.Equal(got, []string{"food", "trip"}) {
		t.Errorf("tags = %v", got)
	}
	tx := Transaction{Note: "  x ", ExternalID: " e ", Splits: []Split{{Note: " s "}}}.Normalize()
	if tx.Note != "x" || tx.ExternalID != "e" || tx.Splits[0].Note != "s" || tx.Tags == nil {
		t.Errorf("normalize = %+v", tx)
	}
}
