package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/paramah/denar/internal/adapter/fakeconnector"
	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
	"github.com/paramah/denar/internal/usecase/fakes"
)

// upload posts a CSV file with its options as multipart/form-data.
func (c *client) upload(status int, path string, file []byte, options string, out any) {
	c.e.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "lista_operacji.csv")
	fw.Write(file)
	mw.WriteField("options", options)
	mw.Close()
	req, err := http.NewRequest("POST", c.e.base+path, &body)
	if err != nil {
		c.e.t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != status {
		c.e.t.Fatalf("POST %s = %d, want %d: %s", path, resp.StatusCode, status, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			c.e.t.Fatalf("POST %s: body %q: %v", path, raw, err)
		}
	}
}

type importBatch struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	SourceRef      string `json:"source_ref"`
	TotalRows      int    `json:"total_rows"`
	CreatedCount   int    `json:"created_count"`
	DuplicateCount int    `json:"duplicate_count"`
	ErrorCount     int    `json:"error_count"`
	MatchedCount   int    `json:"matched_count"`
	RevertedCount  int    `json:"reverted_count"`
	Errors         []struct {
		Row   int    `json:"row"`
		Field string `json:"field"`
	} `json:"errors"`
}

// TestImportEndToEnd is the walk of stage 008: an mBank statement in
// Windows-1250 with a Netflix payment is previewed and imported, the
// payment matches the subscription's charge, the same file again brings
// only duplicates, the first batch is reverted and the charge is open
// again; a bot imports through REST with its token.
func TestImportEndToEnd(t *testing.T) {
	nbpServer := httptest.NewServer(&fakeNBP{rates: map[string]string{}})
	defer nbpServer.Close()
	setEnv(t, "DENAR_AUTO_MIGRATE", "true", "DENAR_NBP_URL", nbpServer.URL)
	clock := fakes.NewClock(time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC))
	e := startE2EWith(t, appOptions{clock: clock, noSubscriptionJobs: true, noNotificationJobs: true})
	statement, err := os.ReadFile("../../internal/adapter/csvimport/testdata/mbank.csv")
	if err != nil {
		t.Fatal(err)
	}

	admin := e.browser()
	admin.must(200, "POST", "/api/auth/login", `{"email":"`+adminEmail+`","password":"`+adminPassword+`"}`, nil)
	var h household
	admin.must(201, "POST", "/api/households", `{"name":"Dom","base_currency":"PLN","timezone":"Europe/Warsaw","month_start_day":1}`, &h)
	api := "/api/households/" + h.ID
	var account wallet
	admin.must(201, "POST", api+"/wallets", `{"name":"eKonto","type":"checking","currency":"PLN"}`, &account)
	var netflix subscriptionJSON
	admin.must(201, "POST", api+"/subscriptions", `{"name":"Netflix","vendor":"netflix","price":{"amount":"49.00","currency":"PLN"},
		"cycle":{"unit":"month","anchor":"2026-10-31"},"wallet_id":"`+account.ID+`"}`, &netflix)
	chargeStatus := func() string {
		t.Helper()
		var page struct {
			Items []chargeJSON `json:"items"`
		}
		admin.must(200, "GET", api+"/charges?subscription_id="+netflix.ID, "", &page)
		if len(page.Items) == 0 || page.Items[0].DueDate != "2026-10-31" {
			t.Fatalf("charges = %+v", page.Items)
		}
		return page.Items[0].Status
	}

	var presets struct {
		Items []struct {
			Key string `json:"key"`
		} `json:"items"`
	}
	admin.must(200, "GET", "/api/import-presets", "", &presets)
	if len(presets.Items) != 5 || presets.Items[1].Key != "mbank" {
		t.Fatalf("presets = %+v", presets)
	}

	options := `{"wallet_id":"` + account.ID + `","preset":"mbank"}`
	var preview struct {
		Detected struct {
			Encoding   string `json:"encoding"`
			HeaderLine int    `json:"header_line"`
		} `json:"detected"`
		NewCount   int `json:"new_count"`
		ErrorCount int `json:"error_count"`
		Rows       []struct {
			Status   string `json:"status"`
			PayeeRaw string `json:"payee_raw"`
			Amount   money  `json:"amount"`
		} `json:"rows"`
	}
	admin.upload(200, api+"/imports/csv/preview", statement, options, &preview)
	if preview.Detected.Encoding != "windows-1250" || preview.Detected.HeaderLine != 23 || preview.NewCount != 3 || preview.ErrorCount != 1 ||
		len(preview.Rows) != 3 || preview.Rows[0].Amount.Amount != "-49.00" || preview.Rows[0].Status != "new" {
		t.Fatalf("preview = %+v", preview)
	}
	if chargeStatus() != "expected" {
		t.Fatal("the preview matched the charge")
	}

	var first importBatch
	admin.upload(201, api+"/imports/csv", statement, options, &first)
	if first.CreatedCount != 3 || first.ErrorCount != 1 || first.MatchedCount != 1 || first.SourceRef != "lista_operacji.csv" ||
		len(first.Errors) != 1 || first.Errors[0].Row != 27 || first.Errors[0].Field != "date" {
		t.Fatalf("import = %+v", first)
	}
	if chargeStatus() != "matched" {
		t.Error("the Netflix payment did not match its charge")
	}
	var txs struct {
		Items []txn `json:"items"`
	}
	admin.must(200, "GET", api+"/transactions?wallet_id="+account.ID, "", &txs)
	if len(txs.Items) != 3 || txs.Items[0].Source != "import" || txs.Items[0].Amount.Amount != "-49.00" || txs.Items[2].Kind != "income" {
		t.Fatalf("transactions = %+v", txs.Items)
	}

	var again importBatch
	admin.upload(201, api+"/imports/csv", statement, options, &again)
	if again.CreatedCount != 0 || again.DuplicateCount != 3 || again.MatchedCount != 0 {
		t.Fatalf("second import = %+v", again)
	}
	var batches struct {
		Items      []importBatch `json:"items"`
		NextCursor *string       `json:"next_cursor"`
	}
	admin.must(200, "GET", api+"/imports?wallet_id="+account.ID, "", &batches)
	if len(batches.Items) != 2 || batches.Items[0].ID != again.ID || batches.NextCursor != nil {
		t.Fatalf("batches = %+v", batches)
	}

	var reverted importBatch
	admin.must(200, "DELETE", api+"/imports/"+first.ID, "", &reverted)
	if reverted.Status != "reverted" || reverted.RevertedCount != 3 {
		t.Fatalf("reverted = %+v", reverted)
	}
	admin.must(409, "DELETE", api+"/imports/"+first.ID, "", nil)
	if chargeStatus() != "expected" {
		t.Error("the charge stayed matched after the revert")
	}
	admin.must(200, "GET", api+"/transactions?wallet_id="+account.ID, "", &txs)
	if len(txs.Items) != 0 {
		t.Fatalf("transactions after the revert = %+v", txs.Items)
	}

	// A bot may import too; without subscriptions:write it matches nothing.
	var created botWithToken
	admin.must(201, "POST", api+"/bots", `{"name":"Importer","scopes":["transactions:write","transactions:read"]}`, &created)
	importer := e.bot(created.Token)
	var byBot importBatch
	importer.upload(201, api+"/imports/csv", statement, options, &byBot)
	if byBot.CreatedCount != 3 || byBot.MatchedCount != 0 {
		t.Fatalf("bot import = %+v", byBot)
	}
	importer.must(403, "GET", "/api/connection-providers", "", nil)

	var providers struct {
		Items []struct {
			Key string `json:"key"`
		} `json:"items"`
	}
	admin.must(200, "GET", "/api/connection-providers", "", &providers)
	if len(providers.Items) != 0 {
		t.Errorf("providers without DENAR_DEV_FAKE_CONNECTOR = %+v", providers)
	}
	var conns struct {
		Items []any `json:"items"`
	}
	admin.must(200, "GET", api+"/connections", "", &conns)
	if len(conns.Items) != 0 {
		t.Errorf("connections = %+v", conns)
	}
}

// With DENAR_DEV_FAKE_CONNECTOR the pretend bank is a provider, and a
// connection to it syncs through the import.
func TestFakeConnectorEndToEnd(t *testing.T) {
	nbpServer := httptest.NewServer(&fakeNBP{rates: map[string]string{}})
	defer nbpServer.Close()
	key := "AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyA="
	setEnv(t, "DENAR_AUTO_MIGRATE", "true", "DENAR_NBP_URL", nbpServer.URL, "DENAR_DEV_FAKE_CONNECTOR", "1", "DENAR_MASTER_KEY", key)
	clock := fakes.NewClock(time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC))
	e := startE2EWith(t, appOptions{clock: clock, noSubscriptionJobs: true, noNotificationJobs: true})
	admin := e.browser()
	admin.must(200, "POST", "/api/auth/login", `{"email":"`+adminEmail+`","password":"`+adminPassword+`"}`, nil)
	var providers struct {
		Items []struct {
			Key  string `json:"key"`
			Kind string `json:"kind"`
		} `json:"items"`
	}
	admin.must(200, "GET", "/api/connection-providers", "", &providers)
	if len(providers.Items) != 1 || providers.Items[0].Key != "fake" || providers.Items[0].Kind != "bank" {
		t.Fatalf("providers = %+v", providers)
	}

	// No endpoint creates connections yet (stage 009): the use cases do.
	var h household
	admin.must(201, "POST", "/api/households", `{"name":"Dom","base_currency":"PLN","timezone":"Europe/Warsaw","month_start_day":1}`, &h)
	var account wallet
	admin.must(201, "POST", "/api/households/"+h.ID+"/wallets", `{"name":"eKonto","type":"checking","currency":"PLN"}`, &account)
	ctx := context.Background()
	u, err := e.app.deps.Users.ByEmail(ctx, adminEmail)
	if err != nil {
		t.Fatal(err)
	}
	p := usecase.Principal{User: u}
	conn, err := usecase.NewCreateConnection(e.app.deps).Do(ctx, p, domain.ID(h.ID), usecase.CreateConnectionInput{Provider: "fake",
		Credentials: []byte("token")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := usecase.NewMapConnectionAccount(e.app.deps).Do(ctx, p, domain.ID(h.ID), usecase.MapConnectionAccountInput{
		ConnectionID: string(conn.Connection.ID), ExternalID: fakeconnector.AccountID, Currency: "PLN", WalletID: account.ID}); err != nil {
		t.Fatal(err)
	}
	res, err := usecase.NewSyncConnection(e.app.deps).Do(ctx, p, domain.ID(h.ID), conn.Connection.ID)
	if err != nil || res.Error != "" || len(res.Batches) != 1 || res.Batches[0].Created == 0 {
		t.Fatalf("sync = %+v %v", res, err)
	}
	var conns struct {
		Items []struct {
			Status         string  `json:"status"`
			HasCredentials bool    `json:"has_credentials"`
			LastSyncAt     *string `json:"last_sync_at"`
			Accounts       []struct {
				WalletID *string `json:"wallet_id"`
			} `json:"accounts"`
		} `json:"items"`
	}
	admin.must(200, "GET", "/api/households/"+h.ID+"/connections", "", &conns)
	if len(conns.Items) != 1 || conns.Items[0].Status != "active" || !conns.Items[0].HasCredentials || conns.Items[0].LastSyncAt == nil ||
		len(conns.Items[0].Accounts) != 1 || *conns.Items[0].Accounts[0].WalletID != account.ID {
		t.Fatalf("connections = %+v", conns)
	}
	var batches struct {
		Items []importBatch `json:"items"`
	}
	admin.must(200, "GET", "/api/households/"+h.ID+"/imports", "", &batches)
	if len(batches.Items) != 1 || batches.Items[0].CreatedCount != res.Batches[0].Created {
		t.Fatalf("batches = %+v", batches)
	}
}
