package wallet

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/paramah/denar/internal/domain"
)

func valid() Wallet {
	return Wallet{Name: " Konto ", Type: TypeChecking, Currency: "PLN", Visibility: Shared, CreatedBy: "u1"}
}

func TestNormalize(t *testing.T) {
	w := valid()
	w.MemberIDs = []domain.ID{"u2"}
	w = w.Normalize()
	if w.Name != "Konto" || len(w.MemberIDs) != 0 {
		t.Errorf("shared = %+v", w)
	}
	p := valid()
	p.Visibility, p.MemberIDs = Private, []domain.ID{"u3", "u2", "u3"}
	p = p.Normalize()
	if !slices.Equal(p.MemberIDs, []domain.ID{"u1", "u2", "u3"}) {
		t.Errorf("private members = %v", p.MemberIDs)
	}
	if !p.VisibleTo("u2") || p.VisibleTo("u9") || !valid().VisibleTo("u9") {
		t.Error("VisibleTo")
	}
}

func TestValidate(t *testing.T) {
	if err := valid().Normalize().Validate(); err != nil {
		t.Fatal(err)
	}
	tests := map[string]func(*Wallet){
		"name":       func(w *Wallet) { w.Name = "" },
		"type":       func(w *Wallet) { w.Type = "piggy" },
		"currency":   func(w *Wallet) { w.Currency = "XYZ" },
		"visibility": func(w *Wallet) { w.Visibility = "secret" },
		"member_ids": func(w *Wallet) {
			for i := range 51 {
				w.MemberIDs = append(w.MemberIDs, domain.ID(strings.Repeat("x", i+1)))
			}
		},
	}
	for field, change := range tests {
		w := valid()
		change(&w)
		var ve domain.ValidationError
		if err := w.Validate(); !errors.As(err, &ve) || ve.Field != field {
			t.Errorf("%s: err = %v", field, err)
		}
	}
	long := valid()
	long.Name = strings.Repeat("ż", 101)
	if long.Validate() == nil {
		t.Error("long name accepted")
	}
}
