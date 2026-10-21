package fakeconnector

import (
	"context"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
)

func TestFakeBank(t *testing.T) {
	p := Provider(func() time.Time { return time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC) })
	accounts, err := p.Bank.ListAccounts(context.Background(), nil)
	if err != nil || len(accounts) != 1 || accounts[0].ExternalID != AccountID {
		t.Fatalf("accounts = %+v %v", accounts, err)
	}
	since, _ := domain.ParseDate("2026-10-01")
	txs, err := p.Bank.FetchTransactions(context.Background(), nil, AccountID, since)
	if err != nil {
		t.Fatal(err)
	}
	// 5 October (Netflix) and the Saturdays 3, 10 and 17 October.
	if len(txs) != 4 || txs[1].PayeeRaw != "NETFLIX.COM" || txs[1].Date.String() != "2026-10-05" || txs[0].Amount != "-83.03" {
		t.Errorf("transactions = %+v", txs)
	}
	if _, err := p.Bank.FetchTransactions(context.Background(), nil, "other", since); err == nil {
		t.Error("an unknown account works")
	}
}
