// Package util provides utility functions for URL, path, and pattern matching.
package util

import (
	"context"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// TrackingParams are URL parameters to remove during canonicalization.
var trackingParams = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^utm_`),
	regexp.MustCompile(`(?i)^fbclid$`),
	regexp.MustCompile(`(?i)^gclid$`),
	regexp.MustCompile(`(?i)^mc_`),
}

// CanonicalizeURL normalizes a URL by removing tracking parameters, hash, and normalizing the path.
func CanonicalizeURL(input string, base ...string) (string, error) {
	var parsed *url.URL
	var err error

	if len(base) > 0 && base[0] != "" {
		baseURL, err := url.Parse(base[0])
		if err != nil {
			return "", err
		}
		parsed, err = baseURL.Parse(input)
		if err != nil {
			return "", err
		}
	} else {
		parsed, err = url.Parse(input)
		if err != nil {
			return "", err
		}
	}

	// Remove hash
	parsed.Fragment = ""

	// Remove tracking parameters
	query := parsed.Query()
	for key := range query {
		for _, pattern := range trackingParams {
			if pattern.MatchString(key) {
				query.Del(key)
				break
			}
		}
	}
	parsed.RawQuery = query.Encode()

	// Normalize path (remove duplicate slashes)
	parsed.Path = regexp.MustCompile(`/{2,}`).ReplaceAllString(parsed.Path, "/")

	// Remove trailing slash if input didn't have one (except for root)
	if parsed.Path != "/" && strings.HasSuffix(parsed.Path, "/") && !strings.HasSuffix(input, "/") {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	}

	// Lowercase hostname
	parsed.Host = strings.ToLower(parsed.Host)

	return parsed.String(), nil
}

// SameOrigin checks if two URLs have the same origin (scheme + host).
func SameOrigin(a, b string) bool {
	urlA, errA := url.Parse(a)
	urlB, errB := url.Parse(b)
	if errA != nil || errB != nil {
		return false
	}
	return urlA.Scheme == urlB.Scheme && strings.EqualFold(urlA.Host, urlB.Host)
}

// IsHTTPURL checks if a string is a valid HTTP(S) URL.
func IsHTTPURL(input string) bool {
	parsed, err := url.Parse(input)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

// isPrivateIPv4 checks if IPv4 address parts represent a private network.
func isPrivateIPv4(parts []byte) bool {
	if len(parts) != 4 {
		return false
	}
	a, b := parts[0], parts[1]

	// 0.0.0.0/8, 10.0.0.0/8, 127.0.0.0/8
	if a == 0 || a == 10 || a == 127 {
		return true
	}
	// 100.64.0.0/10 (CGNAT)
	if a == 100 && b >= 64 && b <= 127 {
		return true
	}
	// 172.16.0.0/12
	if a == 172 && b >= 16 && b <= 31 {
		return true
	}
	// 192.168.0.0/16
	if a == 192 && b == 168 {
		return true
	}
	// 169.254.0.0/16 (link-local)
	if a == 169 && b == 254 {
		return true
	}
	// 224.0.0.0/4 and above (multicast, reserved)
	if a >= 224 {
		return true
	}
	return false
}

// IsPrivateNetworkURL checks if a URL points to a private network address.
func IsPrivateNetworkURL(input string) bool {
	parsed, err := url.Parse(input)
	if err != nil {
		return false
	}

	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")

	// Check for localhost
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}

	// Check for IPv6 loopback and link-local
	if host == "::" || host == "::1" || strings.HasPrefix(host, "fe80:") {
		return true
	}

	// Parse as IP
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	// IPv4
	if ipv4 := ip.To4(); ipv4 != nil {
		return isPrivateIPv4(ipv4)
	}

	// IPv6
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate() {
		return true
	}

	// Check for IPv4-mapped IPv6
	if len(ip) == 16 {
		// ::ffff:x.x.x.x format
		if ip[0] == 0 && ip[1] == 0 && ip[2] == 0 && ip[3] == 0 &&
			ip[4] == 0 && ip[5] == 0 && ip[6] == 0 && ip[7] == 0 &&
			ip[8] == 0 && ip[9] == 0 && ip[10] == 0xff && ip[11] == 0xff {
			return isPrivateIPv4(ip[12:16])
		}
	}

	return false
}

// ResolvesToPrivateNetwork checks if a URL's hostname resolves to a private IP.
func ResolvesToPrivateNetwork(ctx context.Context, input string) (bool, error) {
	if IsPrivateNetworkURL(input) {
		return true, nil
	}

	parsed, err := url.Parse(input)
	if err != nil {
		return false, err
	}

	host := parsed.Hostname()
	if net.ParseIP(host) != nil {
		// Already an IP, no DNS lookup needed
		return false, nil
	}

	// DNS lookup
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return false, nil // Treat lookup failure as non-private
	}

	for _, addr := range addrs {
		testURL := parsed.Scheme + "://"
		if addr.IP.To4() != nil {
			testURL += addr.IP.String()
		} else {
			testURL += "[" + addr.IP.String() + "]"
		}
		if IsPrivateNetworkURL(testURL) {
			return true, nil
		}
	}

	return false, nil
}

// AssertPublicNetworkURL returns an error if the URL resolves to a private network.
func AssertPublicNetworkURL(ctx context.Context, input string) error {
	isPrivate, err := ResolvesToPrivateNetwork(ctx, input)
	if err != nil {
		return err
	}
	if isPrivate {
		return &PrivateNetworkError{URL: input}
	}
	return nil
}

// PrivateNetworkError is returned when a URL points to a private network.
type PrivateNetworkError struct {
	URL string
}

func (e *PrivateNetworkError) Error() string {
	return "Private network crawl target rejected. Use --allow-private-network for trusted local fixtures."
}
