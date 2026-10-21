// Package fakeconnector is a pretend bank for development
// (DENAR_DEV_FAKE_CONNECTOR=1): one PLN account with a Netflix charge on
// the 5th of each month and groceries every Saturday. It shows the
// connection flow end to end before the real connectors of stage 009.
package fakeconnector

import (
	"context"
	"fmt"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/connection"
	"github.com/paramah/denar/internal/usecase"
)

// Key is the provider's key in the registry.
const Key = "fake"

// AccountID is the one account the bank has.
const AccountID = "fake-checking"

// Bank is the pretend bank. Now gives today; nil means the system clock.
type Bank struct {
	Now func() time.Time
}

var _ usecase.BankConnector = Bank{}

// Provider returns the registry entry of the bank.
func Provider(now func() time.Time) usecase.ConnectorProvider {
	return usecase.ConnectorProvider{Key: Key, Name: "Fake Bank (dev)", Kind: connection.Bank, Bank: Bank{Now: now}}
}

func (b Bank) today() domain.Date {
	now := time.Now
	if b.Now != nil {
		now = b.Now
	}
	return domain.DateOf(now(), time.UTC)
}

func (Bank) ListAccounts(context.Context, []byte) ([]usecase.ConnectorAccount, error) {
	return []usecase.ConnectorAccount{{ExternalID: AccountID, Name: "Fake eKonto", Currency: "PLN"}}, nil
}

// FetchTransactions returns the account's transactions from since to
// today, at most a year of them.
func (b Bank) FetchTransactions(_ context.Context, _ []byte, account string, since domain.Date) ([]usecase.ImportedTransaction, error) {
	if account != AccountID {
		return nil, fmt.Errorf("fake bank: no account %q", account)
	}
	end := b.today()
	if start := end.AddDays(-366); since.Before(start) {
		since = start
	}
	out := []usecase.ImportedTransaction{}
	for d := since; !d.After(end); d = d.AddDays(1) {
		t := d.Time()
		switch {
		case t.Day() == 5:
			out = append(out, usecase.ImportedTransaction{Row: len(out) + 1, ExternalID: "fake-netflix-" + d.String(), Date: d,
				Amount: "-49.00", PayeeRaw: "NETFLIX.COM", Memo: "Płatność kartą"})
		case t.Weekday() == time.Saturday:
			out = append(out, usecase.ImportedTransaction{Row: len(out) + 1, ExternalID: "fake-groceries-" + d.String(), Date: d,
				Amount: fmt.Sprintf("-%d.%02d", 80+t.Day(), t.Day()), PayeeRaw: "BIEDRONKA", Memo: "Zakupy"})
		}
	}
	return out, nil
}
