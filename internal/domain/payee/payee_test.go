package payee

import (
	"strings"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
)

func TestResolve(t *testing.T) {
	payees := []Payee{
		{ID: "p2", Name: "Lidl", Patterns: []string{"LIDL SP"}},
		{ID: "p1", Name: "Biedronka", Patterns: []string{"JMP S.A.", "biedronka"}},
		{ID: "p3", Name: "Allegro", Patterns: []string{"sp"}},
	}
	tests := map[string]string{
		"lidl":                   "p2",
		"  BIEDRONKA ":           "p1",
		"Platnosc JMP S.A. 1234": "p1",
		"LIDL SP Z O O Warszawa": "p3", // patterns are tried by payee name: Allegro's "sp" comes first
		"Zabka":                  "",
		"":                       "",
	}
	for text, want := range tests {
		p, ok := Resolve(payees, text)
		if (want == "") == ok || string(p.ID) != want {
			t.Errorf("Resolve(%q) = %s, %v; want %q", text, p.ID, ok, want)
		}
	}
}

func TestNormalizeValidate(t *testing.T) {
	p := Payee{Name: " Lidl ", Patterns: []string{" lidl ", "LIDL", "", "x"}}.Normalize()
	if p.Name != "Lidl" || len(p.Patterns) != 2 {
		t.Errorf("normalize = %+v", p)
	}
	if err := p.Validate(); err != nil {
		t.Error(err)
	}
	for _, bad := range []Payee{
		{Name: ""},
		{Name: strings.Repeat("a", 101)},
		{Name: "a", Patterns: []string{strings.Repeat("b", 101)}},
		{Name: "a", DefaultEnvelopeID: domain.ID("e" + strings.Repeat("1", 64))},
	} {
		if bad.Normalize().Validate() == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
}
