// Package scale provides scale ceiling detection and RAG graduation guidance for OKF bundles.
package scale

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chasedputnam/pyra/internal/tokens"
)

// Thresholds for scale ceiling detection (from article research).
const (
	ConceptWarning  = 100    // Approaching ceiling
	ConceptExceeded = 150    // Past ceiling
	TokenWarning    = 400000 // ~400K tokens
	TokenExceeded   = 600000 // ~600K tokens
)

// Status represents the scale ceiling status.
type Status string

const (
	StatusHealthy  Status = "healthy"
	StatusWarning  Status = "warning"
	StatusExceeded Status = "exceeded"
)

// Metrics holds scale-related measurements for a bundle.
type Metrics struct {
	ConceptCount        int     `json:"conceptCount"`
	TotalTokens         int     `json:"totalTokens"`
	AvgTokensPerConcept int     `json:"avgTokensPerConcept"`
	IndexTokens         int     `json:"indexTokens"`
	IndexRatio          float64 `json:"indexRatio"` // IndexTokens / TotalTokens
}

// Ceiling represents scale ceiling status and guidance.
type Ceiling struct {
	Status  Status `json:"status"`
	Message string `json:"message"`
}

// Analyze computes scale metrics and ceiling status for a bundle.
func Analyze(bundleDir string) (*Metrics, *Ceiling, error) {
	estimator := tokens.NewEstimator()

	// Count concepts and tokens
	var conceptCount int
	var totalTokens int

	err := filepath.Walk(bundleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-markdown files
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		// Skip index.md and log.md (reserved files)
		base := filepath.Base(path)
		if base == "index.md" || base == "log.md" {
			return nil
		}

		// Read and count tokens
		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip files we can't read
		}

		conceptCount++
		totalTokens += estimator.Count(string(content))
		return nil
	})

	if err != nil {
		return nil, nil, fmt.Errorf("walking bundle: %w", err)
	}

	// Calculate index tokens
	indexPath := filepath.Join(bundleDir, "index.md")
	var indexTokens int
	if content, err := os.ReadFile(indexPath); err == nil {
		indexTokens = estimator.Count(string(content))
	}

	// Calculate metrics
	var avgTokens int
	var indexRatio float64
	if conceptCount > 0 {
		avgTokens = totalTokens / conceptCount
	}
	if totalTokens > 0 {
		indexRatio = float64(indexTokens) / float64(totalTokens)
	}

	metrics := &Metrics{
		ConceptCount:        conceptCount,
		TotalTokens:         totalTokens,
		AvgTokensPerConcept: avgTokens,
		IndexTokens:         indexTokens,
		IndexRatio:          indexRatio,
	}

	// Determine ceiling status
	ceiling := determineCeiling(metrics)

	return metrics, ceiling, nil
}

// determineCeiling calculates the ceiling status from metrics.
func determineCeiling(m *Metrics) *Ceiling {
	// Check exceeded thresholds first
	if m.ConceptCount > ConceptExceeded || m.TotalTokens > TokenExceeded {
		return &Ceiling{
			Status: StatusExceeded,
			Message: fmt.Sprintf(
				"Bundle has exceeded filing cabinet scale ceiling (%d concepts, %dk tokens). Consider adding RAG.",
				m.ConceptCount, m.TotalTokens/1000,
			),
		}
	}

	// Check warning thresholds
	if m.ConceptCount > ConceptWarning || m.TotalTokens > TokenWarning {
		return &Ceiling{
			Status: StatusWarning,
			Message: fmt.Sprintf(
				"Bundle is approaching filing cabinet scale ceiling (%d concepts, %dk tokens).",
				m.ConceptCount, m.TotalTokens/1000,
			),
		}
	}

	return &Ceiling{
		Status:  StatusHealthy,
		Message: "Bundle is within filing cabinet scale limits.",
	}
}

// RAGGuidance returns the full RAG graduation guidance text.
func RAGGuidance() string {
	return `Your bundle has exceeded the filing cabinet scale ceiling.

WHY THIS MATTERS:
Below ~100 concepts, query cost grows logarithmically — adding articles barely 
changes token cost because agents read the index and summaries, not full bodies.
Above the ceiling, summary navigation produces too many false candidates and 
per-query cost climbs sharply.

RECOMMENDED NEXT STEPS:

1. Add a vector index ON TOP of the existing structure (don't replace it)

2. Use header-based chunking, not token-count chunking:
   - Split at H1, H2, H3 boundaries
   - Each section becomes a retrieval unit
   - Preserves document structure the agent already understands

3. Recommended local vector stores:
   - DuckDB with vss extension (single-file, no server)
   - ChromaDB (Python ecosystem, also serverless)
   - Avoid Pinecone/Milvus unless you need multi-user scale

4. Keep the wiki structure:
   - Summaries, backlinks, and index remain primary navigation
   - Vector search is for queries the structural path can't answer
   - Route synthesis questions to graph, lookup questions to vectors

5. Implementation pattern:
   - PostToolUse hook to re-index changed files on save
   - Embedding model: bge-m3 is consensus pick for markdown
   - Hybrid retrieval: structural navigation by default, vector for precision`
}
