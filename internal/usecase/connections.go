package usecase

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
	"git.eiac.dev/tools/denar/internal/domain/connection"
	"git.eiac.dev/tools/denar/internal/domain/imports"
	"git.eiac.dev/tools/denar/internal/domain/membership"
)

// ProviderView is a provider of the registry.
type ProviderView struct {
	Key  string
	Name string
	Kind connection.Kind
}

// ListConnectionProviders lists the providers the server can connect to
// (FR-831); people only.
type ListConnectionProviders struct{ d Deps }

func NewListConnectionProviders(d Deps) *ListConnectionProviders { return &ListConnectionProviders{d: d} }

func (uc *ListConnectionProviders) Do(_ context.Context, p Principal) ([]ProviderView, error) {
	if err := requireHuman(p); err != nil {
		return nil, err
	}
	out := []ProviderView{}
	for _, pr := range uc.d.Connectors {
		out = append(out, ProviderView{Key: pr.Key, Name: pr.Name, Kind: pr.Kind})
	}
	slices.SortFunc(out, func(a, b ProviderView) int { return cmp.Compare(a.Key, b.Key) })
	return out, nil
}

// ConnectionView is a connection with its accounts; it never carries the
// credentials, only whether there are any.
type ConnectionView struct {
	Connection     connection.Connection
	ProviderName   string
	HasCredentials bool
	Accounts       []connection.Account
}

func (d Deps) connectionView(ctx context.Context, c connection.Connection) (ConnectionView, error) {
	accounts, err := d.Connections.ListAccounts(ctx, c.HouseholdID, c.ID)
	if err != nil {
		return ConnectionView{}, err
	}
	v := ConnectionView{Connection: c, ProviderName: c.Provider, HasCredentials: len(c.SealedCredentials) > 0, Accounts: accounts}
	if pr, ok := d.Connectors[c.Provider]; ok {
		v.ProviderName = pr.Name
	}
	v.Connection.SealedCredentials = nil
	return v, nil
}

// connectionScope is the scope of a person who may manage the household's
// connections: an owner or an admin (FR-833, P-003).
func (d Deps) connectionScope(ctx context.Context, p Principal, householdID domain.ID) (HouseholdScope, error) {
	if err := requireHuman(p); err != nil {
		return HouseholdScope{}, err
	}
	s, err := d.access().household(ctx, p, householdID)
	if err != nil {
		return HouseholdScope{}, err
	}
	return s, requireRole(s, membership.RoleAdmin)
}

// ListConnections lists the household's connections (FR-833).
type ListConnections struct{ d Deps }

func NewListConnections(d Deps) *ListConnections { return &ListConnections{d: d} }

func (uc *ListConnections) Do(ctx context.Context, p Principal, householdID domain.ID) ([]ConnectionView, error) {
	if _, err := uc.d.connectionScope(ctx, p, householdID); err != nil {
		return nil, err
	}
	cs, err := uc.d.Connections.ListByHousehold(ctx, householdID)
	if err != nil {
		return nil, err
	}
	out := make([]ConnectionView, 0, len(cs))
	for _, c := range cs {
		v, err := uc.d.connectionView(ctx, c)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

type CreateConnectionInput struct {
	Provider    string
	DisplayName string
	// Credentials are sealed with the master key, bound to the new id.
	Credentials []byte
}

// CreateConnection links the household to a provider of the registry
// (FR-830). Stage 009 gives it an endpoint with the bank's consent flow.
type CreateConnection struct{ d Deps }

func NewCreateConnection(d Deps) *CreateConnection { return &CreateConnection{d: d} }

func (uc *CreateConnection) Do(ctx context.Context, p Principal, householdID domain.ID, in CreateConnectionInput) (ConnectionView, error) {
	if _, err := uc.d.connectionScope(ctx, p, householdID); err != nil {
		return ConnectionView{}, err
	}
	pr, ok := uc.d.Connectors[strings.TrimSpace(in.Provider)]
	if !ok {
		return ConnectionView{}, domain.ValidationError{Field: "provider", Message: "is not a provider of this server"}
	}
	if uc.d.Secrets == nil {
		return ConnectionView{}, domain.ConflictError{Reason: domain.ConflictSecretsUnavailable}
	}
	now := uc.d.Clock.Now()
	c := connection.Connection{ID: uc.d.IDs.NewID(), HouseholdID: householdID, Provider: pr.Key, Kind: pr.Kind, Status: connection.Active,
		DisplayName: cmpOr(strings.TrimSpace(in.DisplayName), pr.Name), CreatedBy: p.User.ID, CreatedAt: now, UpdatedAt: now}.Normalize()
	if err := c.Validate(); err != nil {
		return ConnectionView{}, err
	}
	sealed, err := uc.d.Secrets.Seal(in.Credentials, []byte(c.ID))
	if err != nil {
		return ConnectionView{}, err
	}
	c.SealedCredentials = sealed
	var out ConnectionView
	err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := uc.d.connectionScope(ctx, p, householdID); err != nil {
			return err
		}
		if err := uc.d.Connections.Create(ctx, c); err != nil {
			return err
		}
		if err := uc.d.record(ctx, householdID, p.Actor(), "connection.created", audit.ResourceConnection, c.ID,
			audit.Changes{}.Set("provider", c.Provider).Set("kind", string(c.Kind)).Set("display_name", c.DisplayName)); err != nil {
			return err
		}
		out, err = uc.d.connectionView(ctx, c)
		return err
	})
	return out, err
}

type MapConnectionAccountInput struct {
	ConnectionID string
	ExternalID   string
	Name         string
	Currency     string
	// WalletID is where the account's transactions go; "" unmaps it.
	WalletID string
}

// MapConnectionAccount stores an account of a connection and the wallet
// it feeds (FR-830).
type MapConnectionAccount struct{ d Deps }

func NewMapConnectionAccount(d Deps) *MapConnectionAccount { return &MapConnectionAccount{d: d} }

func (uc *MapConnectionAccount) Do(ctx context.Context, p Principal, householdID domain.ID, in MapConnectionAccountInput) (connection.Account, error) {
	a := connection.Account{ConnectionID: domain.ID(strings.TrimSpace(in.ConnectionID)), ExternalID: in.ExternalID, Name: in.Name,
		Currency: in.Currency, WalletID: domain.ID(strings.TrimSpace(in.WalletID))}.Normalize()
	if err := a.Validate(); err != nil {
		return connection.Account{}, err
	}
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.connectionScope(ctx, p, householdID)
		if err != nil {
			return err
		}
		if _, err := uc.d.Connections.ByID(ctx, householdID, a.ConnectionID); err != nil {
			return plainNotFound(err)
		}
		if a.WalletID != "" {
			w, err := uc.d.writableWallet(ctx, s, p, a.WalletID)
			if errors.Is(err, domain.ErrNotFound) {
				return domain.ValidationError{Field: "wallet_id", Message: "is not a wallet you can write to"}
			}
			if err != nil {
				return err
			}
			if a.Currency != "" && a.Currency != w.Currency {
				return domain.ValidationError{Field: "wallet_id", Message: "has another currency than the account"}
			}
		}
		if err := uc.d.Connections.PutAccount(ctx, householdID, a); err != nil {
			return err
		}
		return uc.d.record(ctx, householdID, p.Actor(), "connection.account_mapped", audit.ResourceConnection, a.ConnectionID,
			audit.Changes{}.Set("external_id", a.ExternalID).Set("wallet_id", string(a.WalletID)))
	})
	return a, err
}

// SyncResult is what a sync brought in. Error is the connector's failure,
// also stored on the connection; the batches before it stand.
type SyncResult struct {
	Connection ConnectionView
	Batches    []imports.Batch
	Error      string
}

// syncLookback is how far before the last sync a sync fetches again;
// firstSyncDays how far back the first one goes.
const (
	syncLookback  = 7
	firstSyncDays = 90
)

// SyncConnection fetches what a connection's provider has since the last
// sync and imports it with ImportTransactions (FR-832): bank transactions
// into the wallets of their accounts, billing charges as expenses into the
// wallet of the first mapped account.
type SyncConnection struct{ d Deps }

func NewSyncConnection(d Deps) *SyncConnection { return &SyncConnection{d: d} }

func (uc *SyncConnection) Do(ctx context.Context, p Principal, householdID, id domain.ID) (SyncResult, error) {
	s, err := uc.d.connectionScope(ctx, p, householdID)
	if err != nil {
		return SyncResult{}, err
	}
	c, err := uc.d.Connections.ByID(ctx, householdID, id)
	if err != nil {
		return SyncResult{}, plainNotFound(err)
	}
	pr, ok := uc.d.Connectors[c.Provider]
	if !ok || (c.Kind == connection.Bank && pr.Bank == nil) || (c.Kind == connection.Billing && pr.Billing == nil) {
		return SyncResult{}, domain.ConflictError{Reason: domain.ConflictProviderUnavailable}
	}
	if uc.d.Secrets == nil {
		return SyncResult{}, domain.ConflictError{Reason: domain.ConflictSecretsUnavailable}
	}
	creds, err := uc.d.Secrets.Open(c.SealedCredentials, []byte(c.ID))
	if err != nil {
		return SyncResult{}, err
	}
	accounts, err := uc.d.Connections.ListAccounts(ctx, householdID, c.ID)
	if err != nil {
		return SyncResult{}, err
	}
	now := uc.d.Clock.Now()
	since := today(s.Household, now).AddDays(-firstSyncDays)
	if !c.LastSyncAt.IsZero() {
		since = today(s.Household, c.LastSyncAt).AddDays(-syncLookback)
	}
	importer := NewImportTransactions(uc.d)
	out := SyncResult{Batches: []imports.Batch{}}
	run := func(walletID domain.ID, rows []ImportedTransaction) error {
		if len(rows) == 0 {
			return nil
		}
		o, err := importer.Do(ctx, p, householdID, ImportInput{WalletID: string(walletID), Source: imports.SourceConnection,
			SourceRef: string(c.ID), Rows: rows})
		if err != nil {
			return err
		}
		out.Batches = append(out.Batches, o.Batch)
		return nil
	}
	var syncErr error
	switch c.Kind {
	case connection.Bank:
		for _, a := range accounts {
			if a.WalletID == "" {
				continue
			}
			rows, err := pr.Bank.FetchTransactions(ctx, creds, a.ExternalID, since)
			if err == nil {
				err = run(a.WalletID, rows)
			}
			if err != nil {
				syncErr = err
				break
			}
		}
	case connection.Billing:
		i := slices.IndexFunc(accounts, func(a connection.Account) bool { return a.WalletID != "" })
		if i < 0 {
			return SyncResult{}, domain.ValidationError{Field: "accounts", Message: "map an account to a wallet first"}
		}
		charges, err := pr.Billing.FetchCharges(ctx, creds, since)
		if err == nil {
			rows := make([]ImportedTransaction, len(charges))
			for n, ch := range charges {
				rows[n] = ImportedTransaction{Row: n + 1, ExternalID: ch.ExternalID, Date: ch.Date,
					Amount: "-" + strings.TrimPrefix(strings.TrimSpace(ch.Amount), "-"), Currency: ch.Currency,
					PayeeRaw: pr.Name, Memo: ch.Description}
			}
			err = run(accounts[i].WalletID, rows)
		}
		syncErr = err
	}
	if syncErr != nil {
		var fe domain.ForbiddenError
		if errors.As(syncErr, &fe) || errors.Is(syncErr, domain.ErrUnauthenticated) {
			return SyncResult{}, syncErr
		}
		c.Status, c.LastError = connection.Error, syncErr.Error()
		out.Error = connection.Truncate(syncErr.Error(), connection.MaxLastError)
	} else {
		c.Status, c.LastError, c.LastSyncAt = connection.Active, "", now
	}
	c.UpdatedAt = now
	c = c.Normalize()
	created := 0
	for _, b := range out.Batches {
		created += b.Created
	}
	err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.d.Connections.Update(ctx, c); err != nil {
			return err
		}
		changes := audit.Changes{}.Set("status", string(c.Status)).Set("batches", len(out.Batches)).Set("created", created)
		if out.Error != "" {
			changes.Set("error", out.Error)
		}
		if err := uc.d.record(ctx, householdID, p.Actor(), "connection.synced", audit.ResourceConnection, c.ID, changes); err != nil {
			return err
		}
		out.Connection, err = uc.d.connectionView(ctx, c)
		return err
	})
	return out, err
}
