// Package nbp is the RateProvider on the public API of Narodowy Bank
// Polski: average rates of table A, in PLN per unit of currency.
package nbp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// DefaultBaseURL is the public NBP API.
const DefaultBaseURL = "https://api.nbp.pl"

// DefaultTimeout bounds one request: without the network a transaction
// waits for a rate instead of for the API (NFR-011).
const DefaultTimeout = 5 * time.Second

// window is how many days before the requested one a rate may come from.
const window = 6

var _ usecase.RateProvider = (*Provider)(nil)

// Config sets up a Provider. Zero values take the defaults.
type Config struct {
	BaseURL string
	HTTP    *http.Client
}

// Provider asks NBP for rates of table A.
type Provider struct {
	base string
	http *http.Client
}

func New(cfg Config) *Provider {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	client := cfg.HTTP
	if client == nil {
		client = &http.Client{Timeout: DefaultTimeout}
	}
	return &Provider{base: base, http: client}
}

var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

var decimal = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

type ratesResponse struct {
	Code  string `json:"code"`
	Rates []struct {
		EffectiveDate string      `json:"effectiveDate"`
		Mid           json.Number `json:"mid"`
	} `json:"rates"`
}

// PLNRate asks for every rate of the currency within [date-6, date] in one
// request and takes the newest: NBP publishes on business days only, so
// this is the fallback to the last business day of the week before.
func (p *Provider) PLNRate(ctx context.Context, currency string, date domain.Date) (string, domain.Date, error) {
	if !currencyCode.MatchString(currency) {
		return "", domain.Date{}, fmt.Errorf("nbp: currency %q: %w", currency, usecase.ErrRateUnavailable)
	}
	u := fmt.Sprintf("%s/api/exchangerates/rates/a/%s/%s/%s/?format=json", p.base, url.PathEscape(strings.ToLower(currency)),
		date.AddDays(-window).String(), date.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", domain.Date{}, fmt.Errorf("nbp: %w: %w", usecase.ErrRateUnavailable, err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return "", domain.Date{}, fmt.Errorf("nbp: %w: %w", usecase.ErrRateUnavailable, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		// "Brak danych": no table in the window.
		return "", domain.Date{}, fmt.Errorf("nbp: no rate of %s in the week to %s: %w", currency, date, usecase.ErrRateUnavailable)
	case resp.StatusCode != http.StatusOK:
		return "", domain.Date{}, fmt.Errorf("nbp: status %d: %w", resp.StatusCode, usecase.ErrRateUnavailable)
	}
	var body ratesResponse
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	dec.UseNumber() // rates stay decimal strings, never floats
	if err := dec.Decode(&body); err != nil {
		return "", domain.Date{}, fmt.Errorf("nbp: %w: decoding: %w", usecase.ErrRateUnavailable, err)
	}
	best, bestDate := "", domain.Date{}
	for _, r := range body.Rates {
		d, err := domain.ParseDate(r.EffectiveDate)
		if err != nil || d.After(date) || d.Before(date.AddDays(-window)) || !decimal.MatchString(r.Mid.String()) {
			continue
		}
		if bestDate.IsZero() || d.After(bestDate) {
			best, bestDate = r.Mid.String(), d
		}
	}
	if best == "" {
		return "", domain.Date{}, fmt.Errorf("nbp: no usable rate of %s: %w", currency, usecase.ErrRateUnavailable)
	}
	return best, bestDate, nil
}

// IsUnavailable reports whether err says the rate is unavailable.
func IsUnavailable(err error) bool { return errors.Is(err, usecase.ErrRateUnavailable) }
