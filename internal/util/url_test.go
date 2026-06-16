package util

import (
	"context"
	"testing"
)

func TestCanonicalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		base     string
		expected string
	}{
		{"simple URL", "https://example.com/path", "", "https://example.com/path"},
		{"removes hash", "https://example.com/path#section", "", "https://example.com/path"},
		{"removes utm params", "https://example.com/path?utm_source=test&foo=bar", "", "https://example.com/path?foo=bar"},
		{"removes fbclid", "https://example.com/path?fbclid=123&keep=yes", "", "https://example.com/path?keep=yes"},
		{"normalizes slashes", "https://example.com//double//slashes", "", "https://example.com/double/slashes"},
		{"lowercase host", "https://EXAMPLE.COM/Path", "", "https://example.com/Path"},
		{"relative with base", "/other", "https://example.com/base/", "https://example.com/other"},
		{"removes trailing slash if not in input", "https://example.com/path/", "", "https://example.com/path/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			var err error
			if tt.base != "" {
				result, err = CanonicalizeURL(tt.input, tt.base)
			} else {
				result, err = CanonicalizeURL(tt.input)
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("CanonicalizeURL(%q, %q) = %q, want %q", tt.input, tt.base, result, tt.expected)
			}
		})
	}
}

func TestSameOrigin(t *testing.T) {
	tests := []struct {
		a, b     string
		expected bool
	}{
		{"https://example.com/a", "https://example.com/b", true},
		{"https://example.com/a", "https://EXAMPLE.COM/b", true},
		{"https://example.com/a", "http://example.com/b", false},
		{"https://example.com/a", "https://other.com/b", false},
		{"https://example.com:443/a", "https://example.com/b", false}, // port explicit vs implicit
	}

	for _, tt := range tests {
		result := SameOrigin(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("SameOrigin(%q, %q) = %v, want %v", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestIsHTTPURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"https://example.com", true},
		{"http://example.com", true},
		{"ftp://example.com", false},
		{"mailto:test@example.com", false},
		{"/relative/path", false},
		{"not a url", false},
	}

	for _, tt := range tests {
		result := IsHTTPURL(tt.input)
		if result != tt.expected {
			t.Errorf("IsHTTPURL(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestIsPrivateNetworkURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// Localhost
		{"http://localhost/path", true},
		{"http://localhost:8080/path", true},
		{"http://sub.localhost/path", true},

		// Loopback IPv4
		{"http://127.0.0.1/path", true},
		{"http://127.0.0.255/path", true},

		// Private IPv4 ranges
		{"http://10.0.0.1/path", true},
		{"http://10.255.255.255/path", true},
		{"http://172.16.0.1/path", true},
		{"http://172.31.255.255/path", true},
		{"http://192.168.0.1/path", true},
		{"http://192.168.255.255/path", true},

		// CGNAT
		{"http://100.64.0.1/path", true},
		{"http://100.127.255.255/path", true},

		// Link-local
		{"http://169.254.0.1/path", true},

		// Public IPs
		{"http://8.8.8.8/path", false},
		{"http://1.1.1.1/path", false},
		{"https://example.com/path", false},

		// IPv6 loopback
		{"http://[::1]/path", true},
		{"http://[::]/path", true},

		// IPv6 link-local
		{"http://[fe80::1]/path", true},
	}

	for _, tt := range tests {
		result := IsPrivateNetworkURL(tt.input)
		if result != tt.expected {
			t.Errorf("IsPrivateNetworkURL(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestResolvesToPrivateNetwork(t *testing.T) {
	ctx := context.Background()

	// Test that already-private URLs return true
	result, err := ResolvesToPrivateNetwork(ctx, "http://127.0.0.1/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected 127.0.0.1 to be private")
	}

	// Test that public IPs return false
	result, err = ResolvesToPrivateNetwork(ctx, "http://8.8.8.8/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected 8.8.8.8 to be public")
	}
}
