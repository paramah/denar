package fakes

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/connection"
	"github.com/paramah/denar/internal/domain/imports"
	"github.com/paramah/denar/internal/usecase"
)

// CSVParser reads a simple CSV of its own: lines "date;amount;payee" with
// an optional fourth field, the external id, and a fifth, the currency.
// A line starting with "!" is a row error. It knows the presets
// "generic" and "mbank" and remembers the options of the last call.
type CSVParser struct {
	mu   sync.Mutex
	Last usecase.CSVOptions
}

func (p *CSVParser) Presets() []usecase.CSVPreset {
	return []usecase.CSVPreset{
		{Key: "generic", Name: "Ogólny", Description: "Dowolny plik", Options: usecase.CSVOptions{Preset: "generic",
			Columns: usecase.CSVColumns{Date: []string{"date"}, Amount: []string{"amount"}, Payee: []string{"payee"}}}},
		{Key: "mbank", Name: "mBank", Description: "Eksport mBanku", Options: usecase.CSVOptions{Preset: "mbank", Delimiter: ";",
			Encoding: "windows-1250", DateFormat: "YYYY-MM-DD", Decimal: ",",
			Columns: usecase.CSVColumns{Date: []string{"Data operacji"}, Amount: []string{"Kwota"}}}},
	}
}

func (p *CSVParser) Parse(data []byte, opts usecase.CSVOptions) (usecase.CSVParseResult, error) {
	p.mu.Lock()
	p.Last = opts
	p.mu.Unlock()
	if opts.Preset != "" && opts.Preset != "generic" && opts.Preset != "mbank" {
		return usecase.CSVParseResult{}, domain.ValidationError{Field: "options.preset", Message: "is not a known preset"}
	}
	out := usecase.CSVParseResult{Rows: []usecase.ImportedTransaction{}, Errors: []imports.RowError{}, Delimiter: ";",
		Encoding: "utf-8", Headers: []string{}}
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "!") {
			out.Errors = append(out.Errors, imports.RowError{Row: i + 1, Field: "date", Message: "is not a date"})
			continue
		}
		f := strings.Split(line, ";")
		for len(f) < 5 {
			f = append(f, "")
		}
		d, err := domain.ParseDate(f[0])
		if err != nil {
			out.Errors = append(out.Errors, imports.RowError{Row: i + 1, Field: "date", Message: "is not a date"})
			continue
		}
		out.Rows = append(out.Rows, usecase.ImportedTransaction{Row: i + 1, Date: d, Amount: f[1], PayeeRaw: f[2],
			ExternalID: f[3], Currency: f[4]})
	}
	return out, nil
}

// Connector is a bank and billing connector with fixed answers.
// Transactions are per account; Err fails every call.
type Connector struct {
	mu           sync.Mutex
	Accounts     []usecase.ConnectorAccount
	Transactions map[string][]usecase.ImportedTransaction
	Charges      []usecase.BillingCharge
	Err          error
	// Calls records the credentials and since of each fetch.
	Calls []ConnectorCall
}

// ConnectorCall is one fetch a Connector answered.
type ConnectorCall struct {
	Credentials string
	Account     string
	Since       domain.Date
}

func (c *Connector) record(creds []byte, account string, since domain.Date) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Calls = append(c.Calls, ConnectorCall{Credentials: string(creds), Account: account, Since: since})
	return c.Err
}

func (c *Connector) ListAccounts(_ context.Context, creds []byte) ([]usecase.ConnectorAccount, error) {
	if err := c.record(creds, "", domain.Date{}); err != nil {
		return nil, err
	}
	return c.Accounts, nil
}

func (c *Connector) FetchTransactions(_ context.Context, creds []byte, account string, since domain.Date) ([]usecase.ImportedTransaction, error) {
	if err := c.record(creds, account, since); err != nil {
		return nil, err
	}
	return c.Transactions[account], nil
}

func (c *Connector) FetchCharges(_ context.Context, creds []byte, since domain.Date) ([]usecase.BillingCharge, error) {
	if err := c.record(creds, "", since); err != nil {
		return nil, err
	}
	return c.Charges, nil
}

// ErrConnector is an error a Connector may be given.
var ErrConnector = errors.New("bank says no")

// BankProvider and BillingProvider put a Connector in the registry.
func BankProvider(key string, c *Connector) usecase.ConnectorProvider {
	return usecase.ConnectorProvider{Key: key, Name: "Bank " + key, Kind: connection.Bank, Bank: c}
}

func BillingProvider(key string, c *Connector) usecase.ConnectorProvider {
	return usecase.ConnectorProvider{Key: key, Name: "Billing " + key, Kind: connection.Billing, Billing: c}
}
