package rest

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestCSVUploadCarriesTheOptions(t *testing.T) {
	w := newWorld(t)
	h := w.handler()
	body, header := upload(statement, `{"wallet_id":"w1","preset":"mbank","delimiter":";","encoding":"windows-1250",
		"date_format":"YYYY-MM-DD","decimal_separator":",","skip_rows":2,"has_header":true,"invert_amount":false,
		"accept_states":["COMPLETED"],"columns":{"date":["Data operacji"],"amount":["Kwota"],"memo":["Tytuł","3"]},"preview_limit":1}`)
	rec := w.as(h, "member").do("POST", "/api/households/h1/imports/csv/preview", body, header...)
	if rec.Code != 200 {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	o := w.CSV.Last
	if o.Preset != "mbank" || o.Delimiter != ";" || o.Encoding != "windows-1250" || o.DateFormat != "YYYY-MM-DD" || o.Decimal != "," ||
		o.SkipRows != 2 || o.HasHeader == nil || !*o.HasHeader || o.InvertAmount == nil || *o.InvertAmount ||
		!slices.Equal(o.AcceptStates, []string{"COMPLETED"}) || !slices.Equal(o.Columns.Memo, []string{"Tytuł", "3"}) ||
		!slices.Equal(o.Columns.Date, []string{"Data operacji"}) {
		t.Errorf("options = %+v", o)
	}
	var preview struct {
		NewCount   int `json:"new_count"`
		ErrorCount int `json:"error_count"`
		Rows       []struct {
			Status string `json:"status"`
			Amount struct {
				Amount string `json:"amount"`
			} `json:"amount"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.NewCount != 2 || preview.ErrorCount != 1 || len(preview.Rows) != 1 || preview.Rows[0].Amount.Amount != "-10.00" {
		t.Errorf("preview = %+v", preview)
	}
	if n, _ := w.Transactions.ExternalIDs(t.Context(), "h1", "w1", []string{"X-1"}); len(n) != 0 {
		t.Error("the preview stored a transaction")
	}
}

func TestMultipartOnlyOnTheUploads(t *testing.T) {
	w := newWorld(t)
	h := w.handler()
	body, header := upload(statement, `{"wallet_id":"w1"}`)
	rec := w.as(h, "member").do("POST", "/api/households/h1/payees", body, header...)
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "Content-Type") {
		t.Errorf("multipart elsewhere = %d %s", rec.Code, rec.Body)
	}
	rec = w.as(h, "member").do("POST", "/api/households/h1/imports/csv", `{"wallet_id":"w1"}`)
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "multipart") {
		t.Errorf("JSON to the upload = %d %s", rec.Code, rec.Body)
	}
	big := strings.Repeat("2026-10-01;-1.00;x\n", (6<<20)/19+10)
	body, header = upload(big, `{"wallet_id":"w1"}`)
	rec = w.as(h, "member").do("POST", "/api/households/h1/imports/csv", body, header...)
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "too large") && !strings.Contains(rec.Body.String(), "5 MiB") {
		t.Errorf("too large = %d %s", rec.Code, rec.Body)
	}
}
