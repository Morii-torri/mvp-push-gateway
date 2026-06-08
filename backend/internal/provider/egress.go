package provider

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

func NewEgressHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = egressDialContext
	return &http.Client{Timeout: timeout, Transport: transport}
}

func newEgressHTTPClient(timeout time.Duration) *http.Client {
	return NewEgressHTTPClient(timeout)
}

func egressDialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid egress address", ErrInvalidInput)
	}
	dialAddress, err := resolveEgressDialAddress(ctx, host, port)
	if err != nil {
		return nil, err
	}
	return (&net.Dialer{}).DialContext(ctx, network, dialAddress)
}

func resolveEgressDialAddress(ctx context.Context, host string, port string) (string, error) {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" || strings.TrimSpace(port) == "" {
		return "", fmt.Errorf("%w: egress host is required", ErrInvalidInput)
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("%w: invalid egress port", ErrInvalidInput)
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if !egressAddressAllowed(addr) {
			return "", fmt.Errorf("%w: blocked egress target %s", ErrInvalidInput, addr.String())
		}
		return net.JoinHostPort(addr.String(), port), nil
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("%w: egress host resolved no addresses", ErrInvalidInput)
	}
	resolved := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		addr, ok := netipAddrFromIP(ip.IP)
		if !ok {
			return "", fmt.Errorf("%w: invalid resolved egress address", ErrInvalidInput)
		}
		if !egressAddressAllowed(addr) {
			return "", fmt.Errorf("%w: blocked egress target %s", ErrInvalidInput, addr.String())
		}
		resolved = append(resolved, addr)
	}
	return net.JoinHostPort(resolved[0].String(), port), nil
}

func netipAddrFromIP(ip net.IP) (netip.Addr, bool) {
	if v4 := ip.To4(); v4 != nil {
		return netip.AddrFrom4([4]byte{v4[0], v4[1], v4[2], v4[3]}), true
	}
	if v6 := ip.To16(); v6 != nil {
		addr, ok := netip.AddrFromSlice(v6)
		if !ok {
			return netip.Addr{}, false
		}
		return addr.Unmap(), true
	}
	return netip.Addr{}, false
}

func egressAddressAllowed(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || addr.IsUnspecified() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsMulticast() {
		return false
	}
	for _, blocked := range []string{"100.100.100.200", "255.255.255.255"} {
		if addr == netip.MustParseAddr(blocked) {
			return false
		}
	}
	return true
}
