package notification

import (
	"errors"
	"net/netip"
	"net/url"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
)

// ErrTargetForbidden is the error of a notification target in a network
// the server does not send to without the administrator's consent (009).
var ErrTargetForbidden = errors.New("adres docelowy niedozwolony (sieć prywatna) — zobacz DENAR_NOTIFY_ALLOW_PRIVATE")

// TargetPolicy says which addresses notifications may be sent to. The zero
// value sends to public addresses only. AllowPrivate opens the private,
// loopback and other local networks, but not link-local addresses, where
// cloud metadata services live: those, like anything else, can be opened
// one network at a time with Allowed.
type TargetPolicy struct {
	AllowPrivate bool
	Allowed      []netip.Prefix
}

var (
	// linkLocal stays closed under AllowPrivate (169.254.169.254 and its
	// IPv6 kin are metadata services).
	linkLocal = []netip.Prefix{
		netip.MustParsePrefix("169.254.0.0/16"),
		netip.MustParsePrefix("fe80::/10"),
	}
	// forbidden are the networks that are not the public internet.
	forbidden = []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),      // this network, unspecified
		netip.MustParsePrefix("10.0.0.0/8"),     // RFC 1918
		netip.MustParsePrefix("100.64.0.0/10"),  // CGNAT
		netip.MustParsePrefix("127.0.0.0/8"),    // loopback
		netip.MustParsePrefix("172.16.0.0/12"),  // RFC 1918
		netip.MustParsePrefix("192.0.0.0/24"),   // IETF protocol assignments
		netip.MustParsePrefix("192.168.0.0/16"), // RFC 1918
		netip.MustParsePrefix("198.18.0.0/15"),  // benchmarking
		netip.MustParsePrefix("224.0.0.0/4"),    // multicast
		netip.MustParsePrefix("240.0.0.0/4"),    // reserved, broadcast
		netip.MustParsePrefix("::/128"),         // unspecified
		netip.MustParsePrefix("::1/128"),        // loopback
		netip.MustParsePrefix("::/96"),          // IPv4-compatible (deprecated)
		netip.MustParsePrefix("100::/64"),       // discard
		netip.MustParsePrefix("fc00::/7"),       // unique local
		netip.MustParsePrefix("ff00::/8"),       // multicast
		netip.MustParsePrefix("2001:db8::/32"),  // documentation
		netip.MustParsePrefix("64:ff9b:1::/48"), // local-use NAT64
	}
	nat64 = netip.MustParsePrefix("64:ff9b::/96")
)

// Private reports whether addr is outside the public internet: loopback,
// private, link-local, CGNAT, unique local, unspecified, multicast or
// reserved. IPv4-mapped and NAT64 IPv6 addresses are judged by the IPv4
// address inside.
func Private(addr netip.Addr) bool {
	addr = embedded(addr)
	for _, p := range linkLocal {
		if p.Contains(addr) {
			return true
		}
	}
	for _, p := range forbidden {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// embedded returns the IPv4 address an IPv4-mapped or NAT64 address
// carries, or addr itself without a zone.
func embedded(addr netip.Addr) netip.Addr {
	addr = addr.WithZone("").Unmap()
	if addr.Is6() && nat64.Contains(addr) {
		b := addr.As16()
		return netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]})
	}
	return addr
}

// CheckAddr returns ErrTargetForbidden when a notification may not be sent
// to addr.
func (p TargetPolicy) CheckAddr(addr netip.Addr) error {
	if !addr.IsValid() {
		return ErrTargetForbidden
	}
	plain := embedded(addr)
	for _, allowed := range p.Allowed {
		if allowed.Contains(plain) || allowed.Contains(addr.WithZone("")) {
			return nil
		}
	}
	if !Private(plain) {
		return nil
	}
	if p.AllowPrivate {
		for _, ll := range linkLocal {
			if ll.Contains(plain) {
				return ErrTargetForbidden
			}
		}
		return nil
	}
	return ErrTargetForbidden
}

// CheckHost judges the host of a URL before anything is resolved: a
// literal IP address by CheckAddr, a localhost name as loopback. Other
// names are checked when connecting, address by address.
func (p TargetPolicy) CheckHost(host string) error {
	host = strings.TrimSuffix(strings.ToLower(strings.Trim(host, "[]")), ".")
	if addr, err := netip.ParseAddr(host); err == nil {
		return p.CheckAddr(addr)
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return p.CheckAddr(netip.MustParseAddr("127.0.0.1"))
	}
	return nil
}

// CheckTargets checks the URLs a channel configuration sends to (webhook
// url, ntfy server, Matrix homeserver) against the policy.
func (p TargetPolicy) CheckTargets(config map[string]string) error {
	for _, key := range []string{"url", "server", "homeserver"} {
		raw, ok := config[key]
		if !ok || raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil {
			continue // NormalizeConfig has judged the form
		}
		if err := p.CheckHost(u.Hostname()); err != nil {
			return domain.ValidationError{Field: "config." + key, Message: ErrTargetForbidden.Error()}
		}
	}
	return nil
}
