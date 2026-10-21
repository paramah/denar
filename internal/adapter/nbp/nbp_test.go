package nbp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func day(s string) domain.Date {
	d, err := domain.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

// fakeNBP answers like api.nbp.pl and remembers the request paths.
type fakeNBP struct {
	mu     sync.Mutex
	paths  []string
	status int
	body   string
}

func (f *fakeNBP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.paths = append(f.paths, r.URL.Path+"?"+r.URL.RawQuery)
	status, body := f.status, f.body
	f.mu.Unlock()
	if status == 0 {
		status = 200
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write([]byte(body))
}

func start(t *testing.T, f *fakeNBP) *Provider {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return New(Config{BaseURL: srv.URL + "/"})
}

func TestNewestRateOfTheWindow(t *testing.T) {
	f := &fakeNBP{body: `{"table":"A","currency":"dolar amerykański","code":"USD","rates":[
		{"no":"200/A/NBP/2026","effectiveDate":"2026-10-15","mid":3.9100},
		{"no":"201/A/NBP/2026","effectiveDate":"2026-10-16","mid":3.95123456789012}]}`}
	p := start(t, f)
	rate, rateDate, err := p.PLNRate(context.Background(), "USD", day("2026-10-18"))
	if err != nil {
		t.Fatal(err)
	}
	if rate != "3.95123456789012" || rateDate != day("2026-10-16") {
		t.Errorf("rate = %s on %s", rate, rateDate)
	}
	if len(f.paths) != 1 || f.paths[0] != "/api/exchangerates/rates/a/usd/2026-10-12/2026-10-18/?format=json" {
		t.Errorf("requests = %v", f.paths)
	}
}

func TestUnavailable(t *testing.T) {
	cases := map[string]*fakeNBP{
		"404":             {status: 404, body: `404 NotFound - Not Found - Brak danych`},
		"500":             {status: 500, body: `oops`},
		"not json":        {body: `<html>`},
		"no rates":        {body: `{"rates":[]}`},
		"outside window":  {body: `{"rates":[{"effectiveDate":"2026-10-30","mid":4.1}]}`},
		"not a decimal":   {body: `{"rates":[{"effectiveDate":"2026-10-16","mid":-4.1}]}`},
		"exponent number": {body: `{"rates":[{"effectiveDate":"2026-10-16","mid":4e1}]}`},
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := start(t, f).PLNRate(context.Background(), "USD", day("2026-10-18"))
			if !errors.Is(err, usecase.ErrRateUnavailable) || !IsUnavailable(err) {
				t.Errorf("err = %v", err)
			}
		})
	}
	t.Run("bad code goes nowhere", func(t *testing.T) {
		f := &fakeNBP{}
		_, _, err := start(t, f).PLNRate(context.Background(), "us/d", day("2026-10-18"))
		if !errors.Is(err, usecase.ErrRateUnavailable) || len(f.paths) != 0 {
			t.Errorf("err = %v, requests %v", err, f.paths)
		}
	})
	t.Run("unreachable", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		srv.Close()
		_, _, err := New(Config{BaseURL: srv.URL}).PLNRate(context.Background(), "USD", day("2026-10-18"))
		if !errors.Is(err, usecase.ErrRateUnavailable) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		block := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-block:
			case <-r.Context().Done():
			}
		}))
		defer srv.Close()
		defer close(block)
		p := New(Config{BaseURL: srv.URL, HTTP: &http.Client{Timeout: 50 * time.Millisecond}})
		_, _, err := p.PLNRate(context.Background(), "USD", day("2026-10-18"))
		if !errors.Is(err, usecase.ErrRateUnavailable) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestDefaults(t *testing.T) {
	p := New(Config{})
	if p.base != DefaultBaseURL || p.http.Timeout != DefaultTimeout {
		t.Errorf("provider = %+v", p)
	}
}
