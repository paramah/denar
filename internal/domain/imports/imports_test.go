package imports_test

import (
	"strings"
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/imports"
)

func TestHashExternalID(t *testing.T) {
	d, _ := domain.ParseDate("2026-10-20")
	d2, _ := domain.ParseDate("2026-10-21")
	base := imports.HashExternalID(d, -4900, "NETFLIX.COM  Amsterdam", 1)
	if !strings.HasPrefix(base, "h:") || len(base) != 34 {
		t.Fatalf("id = %q", base)
	}
	if again := imports.HashExternalID(d, -4900, "  netflix.com amsterdam ", 1); again != base {
		t.Errorf("normalised text gives %q, want %q", again, base)
	}
	for name, other := range map[string]string{
		"date":       imports.HashExternalID(d2, -4900, "NETFLIX.COM Amsterdam", 1),
		"amount":     imports.HashExternalID(d, -4901, "NETFLIX.COM Amsterdam", 1),
		"text":       imports.HashExternalID(d, -4900, "NETFLIX.COM Berlin", 1),
		"occurrence": imports.HashExternalID(d, -4900, "NETFLIX.COM Amsterdam", 2),
	} {
		if other == base {
			t.Errorf("a different %s gives the same id", name)
		}
	}
}

func TestNormalizeText(t *testing.T) {
	if got := imports.NormalizeText("  Zakup  PRZY\tużyciu\nkarty "); got != "zakup przy użyciu karty" {
		t.Errorf("NormalizeText = %q", got)
	}
}

func TestKeepErrors(t *testing.T) {
	errs := make([]imports.RowError, 150)
	if got := imports.KeepErrors(errs); len(got) != imports.MaxStoredErrors {
		t.Errorf("kept %d", len(got))
	}
	if got := imports.KeepErrors(nil); got == nil || len(got) != 0 {
		t.Errorf("KeepErrors(nil) = %#v", got)
	}
}
