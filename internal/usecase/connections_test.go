package usecase_test

import (
	"errors"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/connection"
	"git.eiac.dev/tools/denar/internal/domain/imports"
	"git.eiac.dev/tools/denar/internal/domain/subscription"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
	"git.eiac.dev/tools/denar/internal/usecase"
	"git.eiac.dev/tools/denar/internal/usecase/fakes"
)

// connectionFamily has the subscription family with a bank "fakebank"
// and a billing provider "fakebill" in the registry, and a master key.
func connectionFamily(t *testing.T) (*family, *fakes.Connector, *fakes.Connector) {
	f := subscriptionFamily(t)
	bank, bill := &fakes.Connector{}, &fakes.Connector{}
	f.deps.Connectors = map[string]usecase.ConnectorProvider{
		"fakebank": fakes.BankProvider("fakebank", bank),
		"fakebill": fakes.BillingProvider("fakebill", bill),
	}
	f.deps.Secrets = fakes.SecretBox{}
	return f, bank, bill
}

func TestConnectionProvidersAndList(t *testing.T) {
	f, _, _ := connectionFamily(t)
	ps, err := usecase.NewListConnectionProviders(f.deps).Do(ctx, human(f.viewer))
	noErr(t, err)
	if len(ps) != 2 || ps[0].Key != "fakebank" || ps[0].Kind != connection.Bank || ps[1].Kind != connection.Billing {
		t.Errorf("providers = %+v", ps)
	}
	b := f.addBot("b1", "h1", f.owner, bot.Scopes...)
	_, err = usecase.NewListConnectionProviders(f.deps).Do(ctx, asBot(b))
	wantForbidden(t, err, domain.ForbiddenHumanOnly)
	empty := f.deps
	empty.Connectors = nil
	ps, err = usecase.NewListConnectionProviders(empty).Do(ctx, human(f.owner))
	noErr(t, err)
	if len(ps) != 0 {
		t.Errorf("empty registry = %+v", ps)
	}

	created, err := usecase.NewCreateConnection(f.deps).Do(ctx, human(f.admin), "h1", usecase.CreateConnectionInput{
		Provider: "fakebank", DisplayName: " mBank ", Credentials: []byte(`{"token":"s3cret"}`)})
	noErr(t, err)
	c := created.Connection
	if c.ID == "" || c.Status != connection.Active || c.Kind != connection.Bank || c.DisplayName != "mBank" ||
		!created.HasCredentials || created.Connection.SealedCredentials != nil || created.ProviderName != "Bank fakebank" {
		t.Errorf("created = %+v", created)
	}
	stored, err := f.Connections.ByID(ctx, "h1", c.ID)
	noErr(t, err)
	if plain, err := (fakes.SecretBox{}).Open(stored.SealedCredentials, []byte(c.ID)); err != nil || string(plain) != `{"token":"s3cret"}` {
		t.Errorf("credentials = %q %v", plain, err)
	}
	for _, e := range f.Audit.All() {
		for _, ch := range e.Changes {
			if s, _ := ch.To.(string); s == `{"token":"s3cret"}` {
				t.Error("the audit log holds the credentials")
			}
		}
	}
	list, err := usecase.NewListConnections(f.deps).Do(ctx, human(f.owner), "h1")
	noErr(t, err)
	if len(list) != 1 || list[0].Connection.ID != c.ID || list[0].Connection.SealedCredentials != nil {
		t.Errorf("list = %+v", list)
	}
	_, err = usecase.NewListConnections(f.deps).Do(ctx, human(f.member), "h1")
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = usecase.NewListConnections(f.deps).Do(ctx, asBot(b), "h1")
	wantForbidden(t, err, domain.ForbiddenHumanOnly)
	_, err = usecase.NewListConnections(f.deps).Do(ctx, human(f.outsider), "h1")
	wantErr(t, err, domain.ErrNotFound)
}

func TestCreateConnectionRefusals(t *testing.T) {
	f, _, _ := connectionFamily(t)
	create := usecase.NewCreateConnection(f.deps)
	_, err := create.Do(ctx, human(f.owner), "h1", usecase.CreateConnectionInput{Provider: "gocardless"})
	wantField(t, err, "provider")
	_, err = create.Do(ctx, human(f.member), "h1", usecase.CreateConnectionInput{Provider: "fakebank"})
	wantForbidden(t, err, domain.ForbiddenRole)
	noKey := f.deps
	noKey.Secrets = nil
	_, err = usecase.NewCreateConnection(noKey).Do(ctx, human(f.owner), "h1", usecase.CreateConnectionInput{Provider: "fakebank"})
	wantConflict(t, err, domain.ConflictSecretsUnavailable)
}

func (f *family) connect(provider string) domain.ID {
	f.t.Helper()
	v, err := usecase.NewCreateConnection(f.deps).Do(ctx, human(f.owner), "h1", usecase.CreateConnectionInput{Provider: provider,
		Credentials: []byte("creds-" + provider)})
	f.must(err)
	return v.Connection.ID
}

func TestMapConnectionAccount(t *testing.T) {
	f, _, _ := connectionFamily(t)
	f.addWallet("wp", "h1", f.member, wallet.Private)
	id := f.connect("fakebank")
	mapAcc := usecase.NewMapConnectionAccount(f.deps)
	a, err := mapAcc.Do(ctx, human(f.owner), "h1", usecase.MapConnectionAccountInput{ConnectionID: string(id), ExternalID: "acc-1",
		Name: "eKonto", Currency: "pln", WalletID: "w1"})
	noErr(t, err)
	if a.WalletID != "w1" || a.Currency != "PLN" {
		t.Errorf("account = %+v", a)
	}
	_, err = mapAcc.Do(ctx, human(f.owner), "h1", usecase.MapConnectionAccountInput{ConnectionID: string(id), ExternalID: "acc-2",
		Currency: "PLN", WalletID: "w2"})
	wantField(t, err, "wallet_id") // USD wallet
	_, err = mapAcc.Do(ctx, human(f.owner), "h1", usecase.MapConnectionAccountInput{ConnectionID: string(id), ExternalID: "acc-2", WalletID: "wp"})
	wantField(t, err, "wallet_id") // someone else's private wallet
	_, err = mapAcc.Do(ctx, human(f.owner), "h1", usecase.MapConnectionAccountInput{ConnectionID: "nope", ExternalID: "acc-2"})
	wantErr(t, err, domain.ErrNotFound)
	_, err = mapAcc.Do(ctx, human(f.member), "h1", usecase.MapConnectionAccountInput{ConnectionID: string(id), ExternalID: "acc-2"})
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = mapAcc.Do(ctx, human(f.owner), "h1", usecase.MapConnectionAccountInput{ConnectionID: string(id)})
	wantField(t, err, "external_id")
}

func TestSyncBankConnection(t *testing.T) {
	f, bank, _ := connectionFamily(t)
	id := f.connect("fakebank")
	for ext, w := range map[string]string{"acc-1": "w1", "acc-unmapped": ""} {
		_, err := usecase.NewMapConnectionAccount(f.deps).Do(ctx, human(f.owner), "h1",
			usecase.MapConnectionAccountInput{ConnectionID: string(id), ExternalID: ext, WalletID: w})
		noErr(t, err)
	}
	bank.Transactions = map[string][]usecase.ImportedTransaction{
		"acc-1": {
			{Row: 1, ExternalID: "tx-1", Date: day("2026-10-31"), Amount: "-80.00", PayeeRaw: "OpenAI", Memo: "ChatGPT Plus"},
			{Row: 2, ExternalID: "tx-2", Date: day("2026-10-20"), Amount: "-15.00", PayeeRaw: "Kiosk"},
		},
		"acc-unmapped": {{Row: 1, ExternalID: "x", Date: day("2026-10-20"), Amount: "-1.00"}},
	}
	sync := usecase.NewSyncConnection(f.deps)
	res, err := sync.Do(ctx, human(f.owner), "h1", id)
	noErr(t, err)
	if res.Error != "" || len(res.Batches) != 1 || res.Batches[0].Created != 2 || res.Batches[0].Source != imports.SourceConnection ||
		res.Batches[0].SourceRef != string(id) || res.Batches[0].Matched != 1 {
		t.Fatalf("sync = %+v", res)
	}
	if res.Connection.Connection.Status != connection.Active || res.Connection.Connection.LastSyncAt.IsZero() {
		t.Errorf("connection = %+v", res.Connection)
	}
	if len(bank.Calls) != 1 || bank.Calls[0].Credentials != "creds-fakebank" || bank.Calls[0].Account != "acc-1" ||
		bank.Calls[0].Since != day("2026-07-23") {
		t.Errorf("calls = %+v", bank.Calls)
	}
	if c := f.charge(f.vars["gpt"], "2026-10-31"); c.Status != subscription.Matched {
		t.Errorf("charge = %+v", c)
	}

	// The next sync looks back a week from the last one; nothing is new.
	res, err = sync.Do(ctx, human(f.owner), "h1", id)
	noErr(t, err)
	if res.Batches[0].Created != 0 || res.Batches[0].Duplicates != 2 || bank.Calls[1].Since != day("2026-10-14") {
		t.Errorf("second sync = %+v, calls %+v", res, bank.Calls)
	}

	// The bank fails: the connection says so.
	bank.Err = fakes.ErrConnector
	res, err = sync.Do(ctx, human(f.owner), "h1", id)
	noErr(t, err)
	if res.Error != fakes.ErrConnector.Error() || res.Connection.Connection.Status != connection.Error ||
		res.Connection.Connection.LastError != fakes.ErrConnector.Error() {
		t.Errorf("failed sync = %+v", res)
	}
	acts := f.actions()
	if acts[len(acts)-1] != "connection.synced" {
		t.Errorf("audit = %v", acts)
	}
}

func TestSyncBillingConnection(t *testing.T) {
	f, _, bill := connectionFamily(t)
	id := f.connect("fakebill")
	sync := usecase.NewSyncConnection(f.deps)
	_, err := sync.Do(ctx, human(f.owner), "h1", id)
	wantField(t, err, "accounts")
	_, err = usecase.NewMapConnectionAccount(f.deps).Do(ctx, human(f.owner), "h1",
		usecase.MapConnectionAccountInput{ConnectionID: string(id), ExternalID: "org-1", WalletID: "w2"})
	noErr(t, err)
	bill.Charges = []usecase.BillingCharge{{ExternalID: "cost-2026-10-20", Date: day("2026-10-20"), Amount: "12.50", Currency: "USD",
		Description: "API usage"}}
	res, err := sync.Do(ctx, human(f.owner), "h1", id)
	noErr(t, err)
	if len(res.Batches) != 1 || res.Batches[0].Created != 1 {
		t.Fatalf("sync = %+v", res)
	}
	txs := f.walletTxs("w2")
	if len(txs) != 1 || txs[0].Amount != -1250 || txs[0].BaseAmount != -5000 || txs[0].ExternalID != "cost-2026-10-20" ||
		txs[0].Note != "Billing fakebill · API usage" {
		t.Errorf("transactions = %+v", txs)
	}
}

func TestSyncConnectionRefusals(t *testing.T) {
	f, _, _ := connectionFamily(t)
	id := f.connect("fakebank")
	sync := usecase.NewSyncConnection(f.deps)
	_, err := sync.Do(ctx, human(f.member), "h1", id)
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = sync.Do(ctx, human(f.owner), "h1", "nope")
	wantErr(t, err, domain.ErrNotFound)
	noKey := f.deps
	noKey.Secrets = nil
	_, err = usecase.NewSyncConnection(noKey).Do(ctx, human(f.owner), "h1", id)
	wantConflict(t, err, domain.ConflictSecretsUnavailable)
	gone := f.deps
	gone.Connectors = nil
	_, err = usecase.NewSyncConnection(gone).Do(ctx, human(f.owner), "h1", id)
	wantConflict(t, err, domain.ConflictProviderUnavailable)
	if errors.Is(err, domain.ErrNotFound) {
		t.Error("unexpected")
	}
}
