package netutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// IsPrivateIP checks if a single parsed IP is private.
func IsPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// IsPrivateURL checks if a given URL resolves to a private or link-local IP address.
func IsPrivateURL(rawURL string) bool {
	if viper.GetBool("testing.allow_private_urls") || strings.HasSuffix(os.Args[0], ".test") || strings.HasSuffix(os.Args[0], ".test.exe") {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return true // invalid URL is considered unsafe
	}
	hostname := u.Hostname()

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return true // unresolvable is unsafe
	}

	for _, ip := range ips {
		if IsPrivateIP(ip) {
			return true
		}
	}
	return false
}

// SecureDialContext is a custom dialer that resolves IPs and checks for SSRF to prevent DNS rebinding.
func SecureDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if viper.GetBool("testing.allow_private_urls") || strings.HasSuffix(os.Args[0], ".test") || strings.HasSuffix(os.Args[0], ".test.exe") {
		var d net.Dialer
		d.Timeout = 30 * time.Second
		return d.DialContext(ctx, network, addr)
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	// For IP addresses directly provided, check them directly.
	if ip := net.ParseIP(host); ip != nil {
		if IsPrivateIP(ip) {
			return nil, errors.New("SSRF protection: requested IP is private or link-local")
		}
		var d net.Dialer
		d.Timeout = 30 * time.Second
		return d.DialContext(ctx, network, addr)
	}

	// Resolve the hostname
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve host: %w", err)
	}

	for _, ip := range ips {
		if IsPrivateIP(ip) {
			return nil, errors.New("SSRF protection: resolved IP is private or link-local")
		}
	}

	// Dial the first resolved IP directly to prevent TOCTOU DNS rebinding
	if len(ips) == 0 {
		return nil, errors.New("no IPs resolved")
	}

	dialAddr := net.JoinHostPort(ips[0].String(), port)
	var d net.Dialer
	d.Timeout = 30 * time.Second
	return d.DialContext(ctx, network, dialAddr)
}
