package rest

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientAddr(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("fd00::/8")}
	tests := []struct {
		name       string
		trusted    []netip.Prefix
		remoteAddr string
		forwarded  []string // X-Forwarded-For header lines
		want       string
	}{
		{"direct, no proxies configured", nil, "203.0.113.5:4000", nil, "203.0.113.5"},
		{"header ignored without trusted proxies", nil, "203.0.113.5:4000", []string{"198.51.100.1"}, "203.0.113.5"},
		{"header ignored from an untrusted peer", trusted, "203.0.113.5:4000", []string{"198.51.100.1"}, "203.0.113.5"},
		{"client behind a trusted proxy", trusted, "10.0.0.2:4000", []string{"198.51.100.1"}, "198.51.100.1"},
		{"trusted proxy without the header", trusted, "10.0.0.2:4000", nil, "10.0.0.2"},
		{"entries the client made up are skipped", trusted, "10.0.0.2:4000", []string{"1.1.1.1, 2.2.2.2, 198.51.100.1"}, "198.51.100.1"},
		{"chain of trusted proxies", trusted, "10.0.0.2:4000", []string{"1.1.1.1, 198.51.100.1, 10.0.0.3, 10.0.0.4"}, "198.51.100.1"},
		{"several header lines", trusted, "10.0.0.2:4000", []string{"1.1.1.1", "198.51.100.1, 10.0.0.3"}, "198.51.100.1"},
		{"only trusted entries", trusted, "10.0.0.2:4000", []string{"10.0.0.9, 10.0.0.3"}, "10.0.0.9"},
		{"garbage next to the proxy", trusted, "10.0.0.2:4000", []string{"198.51.100.1, not-an-address"}, "10.0.0.2"},
		{"garbage behind a trusted hop", trusted, "10.0.0.2:4000", []string{"unknown, 10.0.0.3"}, "10.0.0.3"},
		{"empty header", trusted, "10.0.0.2:4000", []string{""}, "10.0.0.2"},
		{"entry with a port", trusted, "10.0.0.2:4000", []string{"198.51.100.1:5555"}, "198.51.100.1"},
		{"IPv6 client", trusted, "[fd00::2]:4000", []string{"2001:db8::7"}, "2001:db8::7"},
		{"IPv6 entry with a port", trusted, "[fd00::2]:4000", []string{"[2001:db8::7]:5555"}, "2001:db8::7"},
		{"IPv4-mapped peer", trusted, "[::ffff:10.0.0.2]:4000", []string{"198.51.100.1"}, "198.51.100.1"},
		{"IPv4-mapped entry", trusted, "10.0.0.2:4000", []string{"::ffff:198.51.100.1"}, "198.51.100.1"},
		{"remote address without a port", trusted, "203.0.113.5", []string{"198.51.100.1"}, "203.0.113.5"},
		{"remote address that is no address", trusted, "@", []string{"198.51.100.1"}, "@"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for _, line := range tt.forwarded {
				req.Header.Add("X-Forwarded-For", line)
			}
			if got := clientAddr(req, tt.trusted); got != tt.want {
				t.Errorf("clientAddr() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Behind a trusted proxy the limit follows the client, not the proxy, and
// a client cannot dodge it by making up X-Forwarded-For entries.
