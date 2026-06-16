// Package types defines the core data structures used throughout okf-cli.
package types

import "time"

// ContentType represents supported input formats.
type ContentType string

const (
	ContentTypeHTML     ContentType = "html"
	ContentTypeMarkdown ContentType = "markdown"
	ContentTypeMDX      ContentType = "mdx"
	ContentTypeText     ContentType = "text"
)

// RawDocument is an unprocessed document from crawling or import.
type RawDocument struct {
	SourceID     string
	URL          string // set if from web crawl
	FilePath     string // set if from local import
	ContentType  ContentType
	Raw          string
	DiscoveredAt time.Time
}

// Heading represents a Markdown heading.
type Heading struct {
	Depth int
	Text  string
	Slug  string
}

// Link represents a Markdown link.
type Link struct {
	Href string
	Text string
}

// NormalizedDocument is processed and ready for writing.
type NormalizedDocument struct {
	SourceID   string
	Title      string
	Markdown   string
	Resource   string // original URL or path
	SourcePath string
	OutputPath string
	Headings   []Heading
	Links      []Link
	Tags       []string
	Type       string
}

// Concept is a parsed OKF concept from a bundle.
type Concept struct {
	ID          string
	Path        string
	Frontmatter map[string]any
	Type        string
	Title       string
	Description string
	Resource    string
	Tags        []string
	Body        string
}

// KnowledgeGraph holds concepts and their relationships.
type KnowledgeGraph struct {
	Concepts  map[string]*Concept
	Outbound  map[string][]string // concept ID -> linked concept IDs
	Backlinks map[string][]string // concept ID -> concepts that link to it
}

// ValidationIssue represents a validation error or warning.
type ValidationIssue struct {
	Severity string `json:"severity"` // "error" or "warning"
	Code     string `json:"code"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
}

// ValidationReport is the result of bundle validation.
type ValidationReport struct {
	Valid             bool              `json:"valid"`
	Issues            []ValidationIssue `json:"issues"`
	ConceptCount      int               `json:"conceptCount"`
	ReservedFileCount int               `json:"reservedFileCount"`
	WarningCount      int               `json:"warningCount"`
}

// LinkedConcept represents a concept with its link count.
type LinkedConcept struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
	Count int    `json:"count"`
}

// BundleStats contains bundle inspection results.
type BundleStats struct {
	Title             string            `json:"title"`
	ConceptCount      int               `json:"conceptCount"`
	ReservedFileCount int               `json:"reservedFileCount"`
	WarningCount      int               `json:"warningCount"`
	LinkCount         int               `json:"linkCount"`
	BrokenLinks       int               `json:"brokenLinks"`
	OrphanConcepts    []string          `json:"orphanConcepts"`
	TypeDistribution  map[string]int    `json:"typeDistribution"`
	TagDistribution   map[string]int    `json:"tagDistribution"`
	TopLinkedConcepts []LinkedConcept   `json:"topLinkedConcepts"`
	SourceDomains     map[string]int    `json:"sourceDomains"`
}

// SearchResult represents a search hit.
type SearchResult struct {
	ID          string   `json:"id"`
	Title       string   `json:"title,omitempty"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags"`
	Resource    string   `json:"resource,omitempty"`
	Snippet     string   `json:"snippet"`
	Score       float64  `json:"score"`
}

// CrawlProgressEvent represents a progress update during crawling.
type CrawlProgressEvent struct {
	Type       string // "start", "fetch", "fetched", "skipped", "failed", "writing"
	URL        string
	Seed       string
	Fetched    int
	Queued     int
	MaxPages   int
	MaxDepth   int
	Discovered int
	Concepts   int
	OutDir     string
}

// CrawlResult is the result of a crawl operation.
type CrawlResult struct {
	PagesFetched int
	Skipped      int
	Failed       int
	Written      []string
	Documents    []NormalizedDocument
	DryRunPages  []string
}

// ImportResult is the result of an import operation.
type ImportResult struct {
	Written   []string
	Documents []NormalizedDocument
}
