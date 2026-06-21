package tokens

import (
	"strings"
	"testing"
)

func TestEstimatorCount(t *testing.T) {
	est := NewEstimator()

	tests := []struct {
		name     string
		text     string
		minTokens int
		maxTokens int
	}{
		{"empty", "", 0, 0},
		{"single word", "hello", 1, 2},
		{"sentence", "The quick brown fox jumps over the lazy dog.", 8, 12},
		{"code block", "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```", 12, 25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := est.Count(tt.text)
			if count < tt.minTokens || count > tt.maxTokens {
				t.Errorf("Count(%q) = %d, want between %d and %d", tt.text, count, tt.minTokens, tt.maxTokens)
			}
		})
	}
}

func TestEstimatorCountJSON(t *testing.T) {
	est := NewEstimator()

	data := map[string]any{
		"title":       "Test Document",
		"description": "A test document for token counting",
		"tags":        []string{"test", "example"},
	}

	count := est.CountJSON(data)
	if count < 15 || count > 30 {
		t.Errorf("CountJSON returned %d, expected between 15 and 30", count)
	}
}

func TestEstimatorTruncate(t *testing.T) {
	est := NewEstimator()

	tests := []struct {
		name       string
		text       string
		budget     int
		wantTrunc  bool
		maxLen     int
	}{
		{"within budget", "hello world", 100, false, 11},
		{"exactly at budget", "hi", 1, false, 2},
		{"needs truncation", strings.Repeat("hello ", 100), 10, true, 100},
		{"zero budget returns original", "hello", 0, false, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, truncated := est.Truncate(tt.text, tt.budget)
			if truncated != tt.wantTrunc {
				t.Errorf("Truncate() truncated = %v, want %v", truncated, tt.wantTrunc)
			}
			if truncated && len(result) > tt.maxLen {
				t.Errorf("Truncate() result len = %d, want <= %d", len(result), tt.maxLen)
			}
			if !truncated && result != tt.text {
				t.Errorf("Truncate() modified text when not truncating")
			}
		})
	}
}

func TestEstimatorTruncateToSection(t *testing.T) {
	est := NewEstimator()

	text := `# Main Title

Introduction paragraph.

## Section One

Content for section one with some details.

## Section Two

Content for section two with more details.

## Section Three

Final section content.
`

	// Should truncate at a section boundary
	result, truncated := est.TruncateToSection(text, 30)
	if !truncated {
		t.Error("Expected truncation for small budget")
	}
	
	// Result should end cleanly (not mid-word)
	if strings.Contains(result, "Section Three") && !strings.Contains(result, "Final") {
		t.Error("Expected clean truncation at section boundary")
	}

	// No truncation needed for large budget
	result2, truncated2 := est.TruncateToSection(text, 1000)
	if truncated2 {
		t.Error("Should not truncate with large budget")
	}
	if result2 != text {
		t.Error("Should return original text when no truncation needed")
	}
}

func TestEstimatorFallback(t *testing.T) {
	// Test the fallback calculation directly
	est := &Estimator{} // Don't initialize tiktoken
	
	// Fallback uses 4 chars per token
	text := "12345678" // 8 chars = 2 tokens
	count := est.fallbackCount(text)
	if count != 2 {
		t.Errorf("fallbackCount(%q) = %d, want 2", text, count)
	}

	// Test fallback truncation
	longText := strings.Repeat("a", 100)
	result, truncated := est.fallbackTruncate(longText, 10) // 10 tokens = 40 chars
	if !truncated {
		t.Error("Expected fallback truncation")
	}
	if len(result) != 40 {
		t.Errorf("fallbackTruncate result len = %d, want 40", len(result))
	}
}

func TestEstimatorAvailable(t *testing.T) {
	est := NewEstimator()
	
	// Should be available after initialization
	available := est.Available()
	// We don't fail the test if tiktoken isn't available,
	// just verify the method works
	t.Logf("Tiktoken available: %v", available)
}

func TestEstimatorThreadSafe(t *testing.T) {
	est := NewEstimator()
	text := "The quick brown fox"

	// Run concurrent counts
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				est.Count(text)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
