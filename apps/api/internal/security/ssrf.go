package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
)

var (
	ErrInvalidScheme       = errors.New("invalid URL scheme: only http and https are permitted")
	ErrBlockedPrivateIP    = errors.New("outbound request blocked: target resolves to private, loopback, or link-local IP address (SSRF protection)")
	ErrBlockedCloudMeta    = errors.New("outbound request blocked: target matches cloud metadata service endpoint")
	ErrBlockedHost         = errors.New("outbound request blocked: target host is prohibited")
	ErrDNSResolutionFailed = errors.New("outbound request blocked: failed to resolve host IP address")
)

// Private and reserved IP blocks to block for SSRF prevention
var (
	privateIPv4Blocks []*net.IPNet
	privateIPv6Blocks []*net.IPNet
)

func init() {
	ipv4Cidrs := []string{
		"127.0.0.0/8",     // IPv4 loopback
		"10.0.0.0/8",      // RFC 1918 Private-Use
		"172.16.0.0/12",   // RFC 1918 Private-Use
		"192.168.0.0/16",  // RFC 1918 Private-Use
		"169.254.0.0/16",  // RFC 3927 Link-Local / Cloud Metadata
		"0.0.0.0/8",       // Current network
		"100.64.0.0/10",   // Carrier-grade NAT
		"192.0.0.0/24",    // IETF Protocol Assignments
		"192.0.2.0/24",    // TEST-NET-1
		"198.18.0.0/15",   // Benchmark
		"198.51.100.0/24", // TEST-NET-2
		"203.0.113.0/24",  // TEST-NET-3
		"224.0.0.0/4",     // Multicast
		"240.0.0.0/4",     // Reserved for Future Use
	}

	ipv6Cidrs := []string{
		"::1/128",   // IPv6 loopback
		"fc00::/7",  // IPv6 Unique Local Address (ULA)
		"fe80::/10", // IPv6 Link-Local
		"::/128",    // Unspecified address
	}

	for _, cidr := range ipv4Cidrs {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			privateIPv4Blocks = append(privateIPv4Blocks, block)
		}
	}

	for _, cidr := range ipv6Cidrs {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			privateIPv6Blocks = append(privateIPv6Blocks, block)
		}
	}
}

// isSSRFBypassAllowed reports whether the guard is switched off. Only a
// development process may do that: APP_ENV=development together with
// ALLOW_INTERNAL_SSRF=true (or the older IS_DEVELOPMENT=true). Either flag
// on its own does nothing, so a stray variable cannot open production.
func isSSRFBypassAllowed() bool {
	if os.Getenv("APP_ENV") != "development" {
		return false
	}
	return os.Getenv("ALLOW_INTERNAL_SSRF") == "true" || os.Getenv("IS_DEVELOPMENT") == "true"
}

// IsPrivateIP returns true if the given IP address is in a private, loopback, or reserved range.
func IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	// Loopback / Unspecified checks
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}

	// Check IPv4 vs IPv6
	if ip4 := ip.To4(); ip4 != nil {
		for _, block := range privateIPv4Blocks {
			if block.Contains(ip4) {
				return true
			}
		}
		return false
	}

	// Native IPv6 checks
	for _, block := range privateIPv6Blocks {
		if block.Contains(ip) {
			return true
		}
	}

	return false
}

// ValidateOutboundURL inspects target URL for SSRF vulnerabilities.
func ValidateOutboundURL(rawURL string) error {
	if isSSRFBypassAllowed() {
		return nil
	}

	if rawURL == "" {
		return errors.New("empty URL")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrInvalidScheme
	}

	hostname := u.Hostname()
	if hostname == "" {
		return errors.New("missing hostname in URL")
	}
	return validateHost(hostname)
}

// ErrDSNHostRequired is returned for a connection string without a host,
// which lib/pq would turn into a local socket or localhost connection.
var ErrDSNHostRequired = errors.New("connection string must name a host")

// ValidatePostgresDSN checks the host of a postgres:// connection string
// the way ValidateOutboundURL checks a URL: it must name a public host.
// Key=value connection strings are refused because their host is optional.
func ValidatePostgresDSN(dsn string) error {
	if isSSRFBypassAllowed() {
		return nil
	}
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return errors.New("empty connection string")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("invalid connection string: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "postgres" && scheme != "postgresql" {
		return errors.New("connection string must start with postgres://")
	}
	hostname := u.Hostname()
	if hostname == "" {
		return ErrDSNHostRequired
	}
	return validateHost(hostname)
}

// validateHost refuses metadata and local names, then resolves the host and
// refuses any private, loopback, link-local or reserved address.
func validateHost(hostname string) error {
	lowerHost := strings.ToLower(hostname)

	// Block standard cloud metadata hostnames & local alias names
	if lowerHost == "localhost" ||
		lowerHost == "metadata.google.internal" ||
		lowerHost == "metadata.turing.internal" ||
		lowerHost == "169.254.169.254" ||
		strings.HasSuffix(lowerHost, ".internal") ||
		strings.HasSuffix(lowerHost, ".local") {
		return ErrBlockedCloudMeta
	}

	// Resolve hostname IPs
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDNSResolutionFailed, err)
	}

	if len(ips) == 0 {
		return ErrDNSResolutionFailed
	}

	for _, ip := range ips {
		if IsPrivateIP(ip) {
			return ErrBlockedPrivateIP
		}
	}

	return nil
}

// SafeHTTPTransport creates an http.Transport with transport-level DNS resolution
// and socket-level IP validation to prevent DNS rebinding attacks and redirect bypasses.
func SafeHTTPTransport(timeout time.Duration) *http.Transport {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			if isSSRFBypassAllowed() {
				return nil
			}

			host, _, err := net.SplitHostPort(address)
			if err != nil {
				host = address
			}

			ip := net.ParseIP(host)
			if ip != nil && IsPrivateIP(ip) {
				return ErrBlockedPrivateIP
			}
			return nil
		},
	}

	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if !isSSRFBypassAllowed() {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}

				ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
				if err != nil {
					return nil, fmt.Errorf("resolve host: %w", err)
				}
				if len(ips) == 0 {
					return nil, ErrDNSResolutionFailed
				}

				// Validate all resolved IPs
				for _, ip := range ips {
					if IsPrivateIP(ip) {
						return nil, ErrBlockedPrivateIP
					}
				}

				// Use first validated IP
				addr = net.JoinHostPort(ips[0].String(), port)
			}

			return dialer.DialContext(ctx, network, addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
}

// NewSafeHTTPClient returns an http.Client equipped with transport-level SSRF protection
// and strict redirect checks to prevent redirect-based SSRF bypass.
func NewSafeHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: SafeHTTPTransport(timeout),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if isSSRFBypassAllowed() {
				return nil
			}
			// Re-validate redirect target URL
			return ValidateOutboundURL(req.URL.String())
		},
	}
}
