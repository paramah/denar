package rest

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// clientAddr returns the address a request came from. That is the peer's
// address, unless the peer is a trusted proxy: then X-Forwarded-For is read
// from the right, where the proxies append, and the first address that is
// not a trusted proxy is the client. Anything a client put into the header
// itself lies further left and is never reached. A header from an untrusted
// peer is ignored.
func clientAddr(r *http.Request, trusted []netip.Prefix) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	client := peer.Unmap()
	if !isTrusted(client, trusted) {
		return client.String()
	}

	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop, ok := parseHop(hops[i])
		if !ok {
			// Unreadable: the last address a trusted proxy vouched for stands.
			break
		}
		client = hop
		if !isTrusted(hop, trusted) {
			break
		}
	}
	return client.String()
}

// parseHop reads one X-Forwarded-For entry, with or without a port.
func parseHop(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if addr, err := netip.ParseAddr(s); err == nil {
		return addr.Unmap(), true
	}
	if addrPort, err := netip.ParseAddrPort(s); err == nil {
		return addrPort.Addr().Unmap(), true
	}
	return netip.Addr{}, false
}

func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
