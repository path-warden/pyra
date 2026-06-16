package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCrawlDryRun(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head><title>Test</title></head><body>
				<h1>Welcome</h1>
				<a href="/page1">Page 1</a>
				<a href="/page2">Page 2</a>
			</body></html>`))
		} else if r.URL.Path == "/robots.txt" {
			w.Write([]byte("User-agent: *\nAllow: /"))
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><body><h1>Page</h1></body></html>`))
		}
	}))
	defer server.Close()

	opts := CrawlOptions{
		SeedURL:             server.URL,
		OutDir:              t.TempDir(),
		MaxPages:            10,
		MaxDepth:            2,
		SameOrigin:          true,
		RespectRobots:       true,
		Concurrency:         2,
		DryRun:              true,
		AllowPrivateNetwork: true,
	}

	result, err := Crawl(context.Background(), opts)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	if len(result.DryRunPages) == 0 {
		t.Error("expected dry run pages to be populated")
	}
}

func TestCrawlWithServer(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head><title>Test Site</title></head><body>
				<h1>Welcome to Test</h1>
				<p>This is a test page.</p>
				<a href="/about">About</a>
			</body></html>`))
		} else if r.URL.Path == "/about" {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head><title>About</title></head><body>
				<h1>About Us</h1>
				<p>Learn more about us.</p>
			</body></html>`))
		} else if r.URL.Path == "/robots.txt" {
			w.Write([]byte("User-agent: *\nAllow: /"))
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	outDir := t.TempDir()
	opts := CrawlOptions{
		SeedURL:             server.URL,
		OutDir:              outDir,
		MaxPages:            10,
		MaxDepth:            2,
		SameOrigin:          true,
		RespectRobots:       true,
		Concurrency:         2,
		AllowPrivateNetwork: true,
		Force:               true,
	}

	result, err := Crawl(context.Background(), opts)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	if result.PagesFetched == 0 {
		t.Error("expected pages to be fetched")
	}
	if len(result.Documents) == 0 {
		t.Error("expected documents to be generated")
	}
}
