package notification

import (
	"errors"
	"net/netip"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
)

func TestPrivateRanges(t *testing.T) {
	cases := []struct {
		addr    string
		private bool
	}{
		{"127.0.0.1", true},
		{"127.8.9.10", true},
		{"10.1.2.3", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.32.0.1", false},
		{"192.168.1.10", true},
		{"169.254.169.254", true},
		{"100.64.0.1", true},
		{"100.127.255.255", true},
		{"100.128.0.1", false},
		{"0.0.0.0", true},
		{"224.0.0.1", true},
		{"255.255.255.255", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"::", true},
		{"::1", true},
		{"fe80::1", true},
		{"fe80::1%en0", true},
		{"fc00::1", true},
		{"fd12:3456::1", true},
		{"ff02::1", true},
		{"::ffff:127.0.0.1", true},
		{"::ffff:169.254.169.254", true},
		{"::ffff:10.0.0.1", true},
		{"::ffff:8.8.8.8", false},
		{"64:ff9b::7f00:1", true}, // NAT64 of 127.0.0.1
		{"64:ff9b::808:808", false},
		{"2606:4700:4700::1111", false},
		{"2001:4860:4860::8888", false},
	}
	for _, c := range cases {
		if got := Private(netip.MustParseAddr(c.addr)); got != c.private {
			t.Errorf("Private(%s) = %v, want %v", c.addr, got, c.private)
		}
	}
}

func TestTargetPolicyCheckAddr(t *testing.T) {
	strict := TargetPolicy{}
	open := TargetPolicy{AllowPrivate: true}
	lan := TargetPolicy{Allowed: []netip.Prefix{netip.MustParsePrefix("192.168.1.0/24")}}
	metadata := TargetPolicy{AllowPrivate: true, Allowed: []netip.Prefix{netip.MustParsePrefix("169.254.169.254/32")}}
	cases := []struct {
		name   string
		policy TargetPolicy
		addr   string
		ok     bool
	}{
		{"public", strict, "93.184.216.34", true},
		{"loopback strict", strict, "127.0.0.1", false},
		{"mapped loopback strict", strict, "::ffff:127.0.0.1", false},
		{"loopback opt-in", open, "127.0.0.1", true},
		{"rfc1918 opt-in", open, "10.0.0.5", true},
		{"ula opt-in", open, "fd00::5", true},
		{"metadata opt-in still closed", open, "169.254.169.254", false},
		{"ipv6 link-local opt-in still closed", open, "fe80::1", false},
		{"metadata explicit", metadata, "169.254.169.254", true},
		{"allowlist inside", lan, "192.168.1.20", true},
		{"allowlist mapped inside", lan, "::ffff:192.168.1.20", true},
		{"allowlist outside", lan, "192.168.2.20", false},
		{"allowlist other private", lan, "10.0.0.1", false},
		{"invalid", strict, "", false},
	}
	for _, c := range cases {
		var addr netip.Addr
		if c.addr != "" {
			addr = netip.MustParseAddr(c.addr)
		}
		err := c.policy.CheckAddr(addr)
		if (err == nil) != c.ok {
			t.Errorf("%s: CheckAddr(%s) = %v, want ok=%v", c.name, c.addr, err, c.ok)
		}
		if err != nil && !errors.Is(err, ErrTargetForbidden) {
			t.Errorf("%s: error %v is not ErrTargetForbidden", c.name, err)
		}
	}
}

func TestTargetPolicyCheckTargets(t *testing.T) {
	strict := TargetPolicy{}
	bad := []map[string]string{
		{"url": "http://127.0.0.1:8080/hook"},
		{"url": "http://localhost/hook"},
		{"url": "http://LOCALHOST./hook"},
		{"url": "http://api.localhost/hook"},
		{"url": "http://[::1]/hook"},
		{"url": "http://[::ffff:169.254.169.254]/latest"},
		{"url": "http://169.254.169.254/latest/meta-data"},
		{"server": "http://192.168.1.5", "topic": "x"},
		{"homeserver": "https://10.0.0.1", "room_id": "!a:b"},
	}
	for _, cfg := range bad {
		err := strict.CheckTargets(cfg)
		var verr domain.ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("CheckTargets(%v) = %v, want a validation error", cfg, err)
			continue
		}
		if verr.Message != ErrTargetForbidden.Error() {
			t.Errorf("message %q", verr.Message)
		}
	}
	good := []map[string]string{
		{"url": "https://hooks.example.com/x"},
		{"server": "https://ntfy.sh", "topic": "x"},
		{"homeserver": "https://matrix.org", "room_id": "!a:b"},
		{"address": "a@example.com"},
	}
	for _, cfg := range good {
		if err := strict.CheckTargets(cfg); err != nil {
			t.Errorf("CheckTargets(%v) = %v", cfg, err)
		}
	}
	if err := (TargetPolicy{AllowPrivate: true}).CheckTargets(map[string]string{"url": "http://localhost:5678/webhook"}); err != nil {
		t.Errorf("opt-in: %v", err)
	}
	lan := TargetPolicy{Allowed: []netip.Prefix{netip.MustParsePrefix("192.168.1.0/24")}}
	if err := lan.CheckTargets(map[string]string{"server": "http://192.168.1.5"}); err != nil {
		t.Errorf("allowlist: %v", err)
	}
}
