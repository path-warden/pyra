// Package crawler handles web crawling for documentation sites.
package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/temoto/robotstxt"

	"github.com/chasedputnam/pyra/internal/normalize"
	"github.com/chasedputnam/pyra/internal/types"
	"github.com/chasedputnam/pyra/internal/util"
	"github.com/chasedputnam/pyra/internal/writer"
)

const (
	userAgent        = "okfy/0.1 (+https://github.com/chasedputnam/pyra)"
	maxResponseBytes = 5 * 1024 * 1024 // 5MB
	maxRedirects     = 10
)

// CrawlOptions configures the crawler.
type CrawlOptions struct {
	SeedURL                      string
	OutDir                       string
	MaxPages                     int
	MaxDepth                     int
	Include                      []string
	Exclude                      []string
	SameOrigin                   bool
	RespectRobots                bool
	Concurrency                  int
	Title                        string
	Force                        bool
	DryRun                       bool
	AllowPrivateNetwork          bool
	DangerouslyAllowUnsafeOutput bool
	StableTimestamps             bool
	Timestamp                    string
	OnProgress                   func(types.CrawlProgressEvent)
}

// Crawl crawls a website and writes an OKF bundle.
func Crawl(ctx context.Context, opts CrawlOptions) (*types.CrawlResult, error) {
	// Set defaults
	if opts.MaxPages <= 0 {
		opts.MaxPages = 100
	}
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 4
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}

	// Canonicalize seed URL
	seed, err := util.CanonicalizeURL(opts.SeedURL)
	if err != nil {
		return nil, fmt.Errorf("invalid seed URL: %w", err)
	}

	// Check for private network
	if !opts.AllowPrivateNetwork {
		if util.IsPrivateNetworkURL(seed) {
			return nil, fmt.Errorf("private network crawl target rejected. Use --allow-private-network for trusted local fixtures")
		}
		private, err := util.ResolvesToPrivateNetwork(ctx, seed)
		if err == nil && private {
			return nil, fmt.Errorf("private network crawl target rejected. Use --allow-private-network for trusted local fixtures")
		}
	}

	// Load robots.txt
	var robots *robotstxt.RobotsData
	if opts.RespectRobots {
		robots = loadRobots(ctx, seed)
	}

	// Report start
	if opts.OnProgress != nil {
		opts.OnProgress(types.CrawlProgressEvent{
			Type:     "start",
			Seed:     seed,
			MaxPages: opts.MaxPages,
			MaxDepth: opts.MaxDepth,
		})
	}

	// Initialize crawler state
	state := &crawlState{
		opts:       opts,
		seed:       seed,
		robots:     robots,
		visited:    make(map[string]bool),
		queued:     make(map[string]bool),
		documents:  make([]types.NormalizedDocument, 0),
		dryRunURLs: make([]string, 0),
	}

	// Add seed to queue
	state.queue = append(state.queue, queueItem{url: seed, depth: 0})
	state.queued[seed] = true

	// Process queue
	if err := state.processQueue(ctx); err != nil {
		return nil, err
	}

	// Handle dry run
	if opts.DryRun {
		return &types.CrawlResult{
			PagesFetched: 0,
			Skipped:      state.skipped,
			Failed:       state.failed,
			DryRunPages:  state.dryRunURLs,
		}, nil
	}

	// Write bundle
	if opts.OnProgress != nil {
		opts.OnProgress(types.CrawlProgressEvent{
			Type:     "writing",
			Concepts: len(state.documents),
			OutDir:   opts.OutDir,
		})
	}

	timestamp := opts.Timestamp
	if timestamp == "" && opts.StableTimestamps {
		timestamp = "2026-06-14T00:00:00.000Z"
	}

	written, err := writer.WriteOKFBundle(state.documents, writer.WriteOptions{
		OutDir:                       opts.OutDir,
		Title:                        opts.Title,
		Force:                        opts.Force,
		DangerouslyAllowUnsafeOutput: opts.DangerouslyAllowUnsafeOutput,
		Timestamp:                    timestamp,
		Source:                       opts.SeedURL,
	})
	if err != nil {
		return nil, err
	}

	return &types.CrawlResult{
		PagesFetched: state.fetched,
		Skipped:      state.skipped,
		Failed:       state.failed,
		Written:      written,
		Documents:    state.documents,
	}, nil
}

type queueItem struct {
	url   string
	depth int
}

type crawlState struct {
	opts       CrawlOptions
	seed       string
	robots     *robotstxt.RobotsData
	queue      []queueItem
	visited    map[string]bool
	queued     map[string]bool
	documents  []types.NormalizedDocument
	dryRunURLs []string
	fetched    int
	skipped    int
	failed     int
	mu         sync.Mutex
}

func (s *crawlState) processQueue(ctx context.Context) error {
	sem := make(chan struct{}, s.opts.Concurrency)

	for len(s.queue) > 0 && s.fetched < s.opts.MaxPages {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Get next batch
		batchSize := min(len(s.queue), s.opts.MaxPages-s.fetched, s.opts.Concurrency)
		batch := s.queue[:batchSize]
		s.queue = s.queue[batchSize:]

		var wg sync.WaitGroup
		for _, item := range batch {
			if s.fetched >= s.opts.MaxPages {
				break
			}

			wg.Add(1)
			sem <- struct{}{}

			go func(item queueItem) {
				defer wg.Done()
				defer func() { <-sem }()

				s.processURL(ctx, item)
			}(item)
		}
		wg.Wait()
	}

	return nil
}

func (s *crawlState) processURL(ctx context.Context, item queueItem) {
	s.mu.Lock()
	if s.visited[item.url] {
		s.mu.Unlock()
		return
	}
	s.visited[item.url] = true
	s.mu.Unlock()

	// Check if should visit
	if !s.shouldVisit(item.url) {
		s.mu.Lock()
		s.skipped++
		s.mu.Unlock()

		if s.opts.OnProgress != nil {
			s.opts.OnProgress(types.CrawlProgressEvent{
				Type:     "skipped",
				URL:      item.url,
				Fetched:  s.fetched,
				Queued:   len(s.queue),
				MaxPages: s.opts.MaxPages,
			})
		}
		return
	}

	// Dry run mode
	if s.opts.DryRun {
		s.mu.Lock()
		s.dryRunURLs = append(s.dryRunURLs, item.url)
		s.fetched++
		s.mu.Unlock()
		return
	}

	// Report fetch start
	if s.opts.OnProgress != nil {
		s.opts.OnProgress(types.CrawlProgressEvent{
			Type:     "fetch",
			URL:      item.url,
			Fetched:  s.fetched,
			Queued:   len(s.queue),
			MaxPages: s.opts.MaxPages,
		})
	}

	// Fetch the URL
	content, contentType, err := s.fetch(ctx, item.url)
	if err != nil {
		s.mu.Lock()
		s.failed++
		s.mu.Unlock()

		if s.opts.OnProgress != nil {
			s.opts.OnProgress(types.CrawlProgressEvent{
				Type:     "failed",
				URL:      item.url,
				Fetched:  s.fetched,
				Queued:   len(s.queue),
				MaxPages: s.opts.MaxPages,
			})
		}
		return
	}

	// Determine content type
	ct := contentTypeFromHeader(contentType)
	if ct == "" {
		s.mu.Lock()
		s.skipped++
		s.mu.Unlock()
		return
	}

	// Extract links for further crawling
	var discovered int
	if item.depth < s.opts.MaxDepth {
		links := extractLinks(content, item.url)
		for _, link := range links {
			canonical, err := util.CanonicalizeURL(link, item.url)
			if err != nil {
				continue
			}

			s.mu.Lock()
			if !s.queued[canonical] {
				s.queued[canonical] = true
				s.queue = append(s.queue, queueItem{url: canonical, depth: item.depth + 1})
				discovered++
			}
			s.mu.Unlock()
		}
	}

	// Normalize and store document
	raw := types.RawDocument{
		SourceID:     item.url,
		URL:          item.url,
		ContentType:  ct,
		Raw:          content,
		DiscoveredAt: time.Now(),
	}
	doc := normalize.NormalizeDocument(raw)

	s.mu.Lock()
	s.documents = append(s.documents, doc)
	s.fetched++
	s.mu.Unlock()

	if s.opts.OnProgress != nil {
		s.opts.OnProgress(types.CrawlProgressEvent{
			Type:       "fetched",
			URL:        item.url,
			Fetched:    s.fetched,
			Queued:     len(s.queue),
			Discovered: discovered,
			MaxPages:   s.opts.MaxPages,
		})
	}
}

func (s *crawlState) shouldVisit(urlStr string) bool {
	if !util.IsHTTPURL(urlStr) {
		return false
	}

	// Same origin check
	if s.opts.SameOrigin && !util.SameOrigin(urlStr, s.seed) {
		return false
	}

	// Private network check
	if !s.opts.AllowPrivateNetwork && util.IsPrivateNetworkURL(urlStr) {
		return false
	}

	// Include patterns
	if len(s.opts.Include) > 0 && !util.MatchesAnyPattern(urlStr, s.opts.Include) {
		return false
	}

	// Exclude patterns
	if util.MatchesAnyPattern(urlStr, s.opts.Exclude) {
		return false
	}

	// Robots.txt
	if s.robots != nil && !s.robots.TestAgent(urlStr, userAgent) {
		return false
	}

	return true
}

func (s *crawlState) fetch(ctx context.Context, urlStr string) (string, string, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects")
			}
			// Validate redirect target
			if !s.opts.AllowPrivateNetwork && util.IsPrivateNetworkURL(req.URL.String()) {
				return fmt.Errorf("redirect to private network rejected")
			}
			if s.opts.SameOrigin && !util.SameOrigin(req.URL.String(), s.seed) {
				return fmt.Errorf("cross-origin redirect rejected")
			}
			return nil
		},
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
		if err != nil {
			return "", "", err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "text/html,text/markdown,text/plain,*/*")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(250*(1<<attempt)) * time.Millisecond)
			continue
		}

		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			time.Sleep(time.Duration(250*(1<<attempt)) * time.Millisecond)
			continue
		}

		if resp.StatusCode >= 400 {
			_ = resp.Body.Close()
			return "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		// Read body with size limit
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		_ = resp.Body.Close()

		if err != nil {
			lastErr = err
			continue
		}

		if len(body) > maxResponseBytes {
			return "", "", fmt.Errorf("response too large")
		}

		return string(body), resp.Header.Get("Content-Type"), nil
	}

	return "", "", lastErr
}

func loadRobots(ctx context.Context, seedURL string) *robotstxt.RobotsData {
	parsed, err := url.Parse(seedURL)
	if err != nil {
		return nil
	}

	robotsURL := fmt.Sprintf("%s://%s/robots.txt", parsed.Scheme, parsed.Host)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil
	}

	robots, err := robotstxt.FromResponse(resp)
	if err != nil {
		return nil
	}

	return robots
}

func contentTypeFromHeader(header string) types.ContentType {
	lower := strings.ToLower(header)
	if strings.Contains(lower, "text/html") {
		return types.ContentTypeHTML
	}
	if strings.Contains(lower, "markdown") {
		return types.ContentTypeMarkdown
	}
	if strings.Contains(lower, "text/plain") {
		return types.ContentTypeText
	}
	if header == "" {
		return types.ContentTypeHTML
	}
	return ""
}

func extractLinks(html, baseURL string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	var links []string
	doc.Find("a[href]").Each(func(_ int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if !exists || href == "" {
			return
		}
		links = append(links, href)
	})

	return links
}
