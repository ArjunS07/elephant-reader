package ingest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// errBlockedAddress is returned when a feed URL resolves to a host or address
// the fetcher refuses to dial.
var errBlockedAddress = errors.New("blocked address")

// SafeClient is the feed fetcher. It allows only http and https, rejects
// loopback, private, link-local, and Fly-internal addresses, and checks those
// rules again on every redirect. ProxyFromEnvironment is disabled so an
// HTTP_PROXY cannot carry the request to an address this dialer would refuse.
//
// Tests that fetch from httptest.NewServer should set Service.HTTP to a plain
// client. This dialer rejects 127.0.0.1.
func SafeClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return safeDial(ctx, dialer, network, addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	return &http.Client{
		Timeout:       15 * time.Second,
		Transport:     transport,
		CheckRedirect: safeCheckRedirect,
	}
}

func validateFetchScheme(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		return fmt.Errorf("invalid feed url")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("unsupported feed url scheme")
	}
}

func safeCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if req.URL == nil {
		return fmt.Errorf("invalid redirect")
	}
	if err := validateFetchScheme(req.URL.String()); err != nil {
		return err
	}
	host := req.URL.Hostname()
	if blockedHost(host) {
		return fmt.Errorf("%w: %s", errBlockedAddress, host)
	}
	if ip := net.ParseIP(host); ip != nil && blockedIP(ip) {
		return fmt.Errorf("%w: %s", errBlockedAddress, ip)
	}
	return nil
}

func safeDial(ctx context.Context, dialer *net.Dialer, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	host = hostWithoutZone(host)
	if blockedHost(host) {
		return nil, fmt.Errorf("%w: %s", errBlockedAddress, host)
	}
	ips, err := resolveIPs(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if blockedIP(ip) {
			return nil, fmt.Errorf("%w: %s", errBlockedAddress, ip)
		}
	}
	var dialErr error
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		dialErr = err
	}
	if dialErr != nil {
		return nil, dialErr
	}
	return nil, fmt.Errorf("no addresses for %s", host)
}

func resolveIPs(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		if addr.IP != nil {
			ips = append(ips, addr.IP)
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses for %s", host)
	}
	if len(ips) > 16 {
		ips = ips[:16]
	}
	return ips, nil
}

func hostWithoutZone(host string) string {
	host = strings.Trim(host, "[]")
	if i := strings.Index(host, "%"); i >= 0 {
		host = host[:i]
	}
	return host
}

func blockedHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), ".")
	if host == "" {
		return true
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if host == "local" || strings.HasSuffix(host, ".local") {
		return true
	}
	if host == "internal" || strings.HasSuffix(host, ".internal") {
		return true
	}
	return false
}

var blockedNets = mustCIDRs([]string{
	"0.0.0.0/8",
	"10.0.0.0/8",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"::1/128",
	"fc00::/7",
	"fe80::/10",
	"fdaa::/8",
})

func mustCIDRs(cidrs []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(err)
		}
		out = append(out, network)
	}
	return out
}

func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, network := range blockedNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
