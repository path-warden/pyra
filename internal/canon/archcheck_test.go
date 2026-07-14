package canon

import (
	"os/exec"
	"strings"
	"testing"
)

// TestCanonHasNoAIorNetworkDependencies enforces the architectural invariant
// (Requirement 9): the Canon authority path must be a pure function of repo
// state. No package under internal/canon may transitively depend on the
// summarizer, an HTTP client, or an on-device LLM bridge.
func TestCanonHasNoAIorNetworkDependencies(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps",
		"github.com/chasedputnam/pyra/internal/canon/...").Output()
	if err != nil {
		t.Skipf("go list unavailable: %v", err)
	}
	forbidden := []string{
		"net/http",
		"github.com/chasedputnam/pyra/internal/summarize",
		"github.com/blacktop/go-foundationmodels",
	}
	deps := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, dep := range deps {
		for _, bad := range forbidden {
			if dep == bad {
				t.Errorf("internal/canon must not depend on %q (AI/network in the authority path)", bad)
			}
		}
	}
}
