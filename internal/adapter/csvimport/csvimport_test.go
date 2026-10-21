package csvimport

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
)

func sample(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// rows prints the rows as "line|date|amount|currency|payee|memo|id".
func rows(res usecase.CSVParseResult) []string {
	out := []string{}
	for _, r := range res.Rows {
		out = append(out, fmt.Sprintf("%d|%s|%s|%s|%s|%s|%s", r.Row, r.Date, r.Amount, r.Currency, r.PayeeRaw, r.Memo, r.ExternalID))
	}
	return out
}

func errs(res usecase.CSVParseResult) string {
	out := []string{}
	for _, e := range res.Errors {
		out = append(out, fmt.Sprintf("%d:%s", e.Row, e.Field))
	}
	return strings.Join(out, " ")
}

func wantRows(t *testing.T, res usecase.CSVParseResult, want ...string) {
	t.Helper()
	got := rows(res)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func parse(t *testing.T, data []byte, opts usecase.CSVOptions) usecase.CSVParseResult {
	t.Helper()
	res, err := New().Parse(data, opts)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return res
}

func TestPresetMBank(t *testing.T) {
	res := parse(t, sample(t, "mbank.csv"), usecase.CSVOptions{Preset: "mbank"})
	if res.Encoding != "windows-1250" || res.Delimiter != ";" || res.HeaderLine != 23 || res.Headers[0] != "Data operacji" {
		t.Errorf("detected = %s %s %d %v", res.Encoding, res.Delimiter, res.HeaderLine, res.Headers)
	}
	wantRows(t, res,
		"24|2026-10-31|-49.00||NETFLIX.COM /AMSTERDAM ZAKUP PRZY UŻYCIU KARTY||",
		"25|2026-10-28|-123.45||BIEDRONKA 1234 WARSZAWA ZAKUP PRZY UŻYCIU KARTY||",
		"26|2026-10-10|5000.00||WYNAGRODZENIE ACME SP. Z O.O. PRZELEW PRZYCHODZĄCY||",
	)
	if errs(res) != "27:date" {
		t.Errorf("errors = %s", errs(res))
	}
}

func TestPresetMBankOldFormat(t *testing.T) {
	res := parse(t, sample(t, "mbank_old.csv"), usecase.CSVOptions{Preset: "mbank"})
	wantRows(t, res,
		"2|2026-10-30|-49.00||ZAKUP PRZY UŻYCIU KARTY|NETFLIX.COM /AMSTERDAM|",
		"3|2026-10-10|5000.00||ACME SP. Z O.O. UL. PROSTA 1 WARSZAWA|WYNAGRODZENIE ZA PAŹDZIERNIK · PRZELEW PRZYCHODZĄCY|",
	)
}

func TestPresetING(t *testing.T) {
	res := parse(t, sample(t, "ing.csv"), usecase.CSVOptions{Preset: "ing"})
	wantRows(t, res,
		"7|2026-10-31|-49.00|PLN|NETFLIX.COM Amsterdam NL|Płatność kartą 31.10.2026 Nr karty 4246xx1234 · TR.KART|202630400012345",
		"9|2026-10-10|5000.00|PLN|ACME SP. Z O.O.|Wynagrodzenie 10/2026 · PRZELEW|202628300000001",
	)
	// A card block has no amount yet.
	if errs(res) != "8:amount" {
		t.Errorf("errors = %s", errs(res))
	}
}

func TestPresetPKO(t *testing.T) {
	res := parse(t, sample(t, "pko.csv"), usecase.CSVOptions{Preset: "pko"})
	wantRows(t, res,
		"2|2026-10-31|-49.00|PLN||Płatność kartą · Tytuł: 000498849 74230786304123456789012 · Lokalizacja: Adres: NETFLIX.COM Miasto: Amsterdam Kraj: HOLANDIA · Data wykonania operacji: 2026-10-30 · Oryginalna kwota operacji: 49.00 PLN · Numer karty: 425125******1234|",
		"3|2026-10-10|5000.00|PLN||Przelew na rachunek · Rachunek nadawcy: 11 1111 1111 1111 1111 1111 1111 · Nazwa nadawcy: ACME SP Z O O · Adres nadawcy: UL. PROSTA 1 WARSZAWA · Tytuł: WYNAGRODZENIE|",
	)
}

func TestPresetRevolut(t *testing.T) {
	res := parse(t, sample(t, "revolut.csv"), usecase.CSVOptions{Preset: "revolut"})
	wantRows(t, res,
		"2|2026-10-31|-49.00|PLN|Netflix|CARD_PAYMENT|",
		"3|2026-10-01|1000.00|PLN|Top-up by *1234|TOPUP|",
		"5|2026-10-15|-100.50|PLN|Exchanged to EUR|EXCHANGE|",
		"7|2026-10-18|-12.00|EUR|Café de Paris|CARD_PAYMENT|",
	)
	// The pending payment has no completion date yet and is left out.
	if errs(res) != "6:state" {
		t.Errorf("errors = %s", errs(res))
	}
}

func TestGenericDetectsEverything(t *testing.T) {
	for _, name := range []string{"generic_cp1250.csv", "generic_utf8.csv"} {
		t.Run(name, func(t *testing.T) {
			res := parse(t, sample(t, name), usecase.CSVOptions{})
			wantRows(t, res,
				"2|2026-10-31|-1234.56||Sklep Żółw|Zakupy|A-1",
				"3|2026-10-30|2000.00||Pracodawca|Pensja|A-2",
			)
			if errs(res) != "4:amount" {
				t.Errorf("errors = %s", errs(res))
			}
		})
	}
}

func TestMappingByNumberWithoutHeader(t *testing.T) {
	data := []byte("2026-10-01|wydatek|10,00|\n2026-10-02|wpływ||20,00\n2026-10-03|nic||\n")
	no := false
	res := parse(t, data, usecase.CSVOptions{Delimiter: "|", HasHeader: &no, Decimal: ",", Columns: usecase.CSVColumns{
		Date: []string{"1"}, Payee: []string{"2"}, Debit: []string{"3"}, Credit: []string{"4"}}})
	wantRows(t, res,
		"1|2026-10-01|-10.00||wydatek||",
		"2|2026-10-02|20.00||wpływ||",
	)
	if errs(res) != "3:amount" || res.HeaderLine != 0 {
		t.Errorf("errors = %s, header %d", errs(res), res.HeaderLine)
	}
}

func TestOverridesAndInversion(t *testing.T) {
	data := []byte("x;y\nnot;a header\nWhen;How much;Who\n01/10/2026;12.50;Shop\n02/10/2026;(3.00);Refund\n")
	yes := true
	res := parse(t, data, usecase.CSVOptions{SkipRows: 1, DateFormat: "DD/MM/YYYY", InvertAmount: &yes,
		Columns: usecase.CSVColumns{Date: []string{"when"}, Amount: []string{"How much"}, Payee: []string{"WHO"}}})
	wantRows(t, res,
		"4|2026-10-01|-12.50||Shop||",
		"5|2026-10-02|3.00||Refund||",
	)
	if res.HeaderLine != 3 {
		t.Errorf("header line = %d", res.HeaderLine)
	}
}

func TestParseNumber(t *testing.T) {
	for _, c := range []struct {
		in      string
		decimal byte
		want    string
	}{
		{"-1 234,56 PLN", ',', "-1234.56"},
		{"1 234,56", 0, "1234.56"},
		{"1.234,56", 0, "1234.56"},
		{"1,234.56", 0, "1234.56"},
		{"1'234.5", 0, "1234.5"},
		{"−" + "49,00", 0, "-49.00"},
		{"49,00-", ',', "-49.00"},
		{"+5000.00", '.', "5000.00"},
		{"(12.00)", 0, "-12.00"},
		{"1,234,567", 0, "1234567"},
		{"1.234.567", 0, "1234567"},
		{"12", 0, "12"},
		{"$12.99", 0, "12.99"},
		{"0,5", ',', "0.5"},
		{",5", ',', "0.5"},
	} {
		v, places, ok := parseNumber(c.in, c.decimal)
		if !ok || v.FloatString(places) != c.want {
			t.Errorf("parseNumber(%q) = %v %d %v, want %s", c.in, v, places, ok, c.want)
		}
	}
	for _, bad := range []string{"", "abc", "1,2,3.4.5", "12#"} {
		if _, _, ok := parseNumber(bad, ','); ok {
			t.Errorf("parseNumber(%q) is ok", bad)
		}
	}
}

func TestParseDate(t *testing.T) {
	layout, err := dateLayout("D.M.YYYY")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ in, layout, want string }{
		{"2026-10-31", "", "2026-10-31"},
		{"31.10.2026", "", "2026-10-31"},
		{"31-10-2026", "", "2026-10-31"},
		{"2026/10/31", "", "2026-10-31"},
		{"2026-10-31 09:12:01", "", "2026-10-31"},
		{"2026-10-31T09:12:01Z", "", "2026-10-31"},
		{"1.2.2026", layout, "2026-02-01"},
	} {
		d, err := parseDate(c.in, c.layout)
		if err != nil || d.String() != c.want {
			t.Errorf("parseDate(%q) = %s %v", c.in, d, err)
		}
	}
	if _, err := parseDate("2026-02-30", ""); err == nil {
		t.Error("30 February is a date")
	}
	for _, bad := range []string{"YYYY-MM", "Q-YYYY-MM-DD", "DD/MM"} {
		if _, err := dateLayout(bad); err == nil {
			t.Errorf("dateLayout(%q) is ok", bad)
		}
	}
}

func TestFileErrors(t *testing.T) {
	cases := map[string]struct {
		data  string
		opts  usecase.CSVOptions
		field string
	}{
		"unknown preset":       {"a", usecase.CSVOptions{Preset: "citi"}, "options.preset"},
		"bad delimiter":        {"a", usecase.CSVOptions{Delimiter: "x"}, "options.delimiter"},
		"bad encoding":         {"a", usecase.CSVOptions{Encoding: "latin2"}, "options.encoding"},
		"bad decimal":          {"a", usecase.CSVOptions{Decimal: ";"}, "options.decimal_separator"},
		"bad date format":      {"a", usecase.CSVOptions{DateFormat: "MM"}, "options.date_format"},
		"bad skip":             {"a", usecase.CSVOptions{SkipRows: -1}, "options.skip_rows"},
		"no header":            {"x;y\n1;2\n", usecase.CSVOptions{}, "options.columns.date"},
		"no amount column":     {"Data;Opis\n2026-01-01;x\n", usecase.CSVOptions{}, "options.columns.amount"},
		"date column missing":  {"a;b\n", usecase.CSVOptions{Columns: usecase.CSVColumns{Date: []string{"9"}}, HasHeader: ptr(false)}, ""},
		"empty date in preset": {"a", usecase.CSVOptions{Columns: usecase.CSVColumns{Date: []string{" "}}}, ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := New().Parse([]byte(c.data), c.opts)
			if c.field == "" {
				return // anything but a panic
			}
			var ve domain.ValidationError
			if !errors.As(err, &ve) || ve.Field != c.field {
				t.Errorf("error = %v, want one on %s", err, c.field)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestPresetsAreComplete(t *testing.T) {
	ps := New().Presets()
	keys := []string{}
	for _, p := range ps {
		keys = append(keys, p.Key)
		if p.Name == "" || p.Description == "" || p.Options.Preset != p.Key || len(p.Options.Columns.Date) == 0 ||
			len(p.Options.Columns.Amount) == 0 {
			t.Errorf("preset %+v", p)
		}
	}
	if strings.Join(keys, " ") != "generic mbank ing pko revolut" {
		t.Errorf("presets = %v", keys)
	}
}

func TestInvalidCSVLinesAreRowErrors(t *testing.T) {
	data := []byte("Data;Kwota\n2026-10-01;1,00\n2026-10-02;2,00\n")
	res := parse(t, data, usecase.CSVOptions{})
	if len(res.Rows) != 2 || len(res.Errors) != 0 {
		t.Errorf("rows %v errors %v", rows(res), res.Errors)
	}
}
