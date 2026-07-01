package codeintel_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestCanonDoesNotDependOnCodeIntel enforces the design boundary: the Canon
// authority path must stay a pure, offline, deterministic function of repo
// state, so no package under internal/canon may depend on the code-intelligence
// package or the tree-sitter runtime it uses. Together with the existing
// archcheck test, this keeps `memphis gate` unchanged by this feature.
func TestCanonDoesNotDependOnCodeIntel(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps",
		"github.com/chasedputnam/memphis/internal/canon/...").Output()
	if err != nil {
		t.Skipf("go list unavailable: %v", err)
	}
	forbidden := []string{
		"github.com/chasedputnam/memphis/internal/codeintel",
		"github.com/odvcencio/gotreesitter",
	}
	for _, dep := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		for _, bad := range forbidden {
			if dep == bad || strings.HasPrefix(dep, bad+"/") {
				t.Errorf("internal/canon must not depend on %q (code-intelligence in the authority path)", dep)
			}
		}
	}
}
