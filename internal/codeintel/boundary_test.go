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
		// The change-aware gate depends on codeintel + store; the authority path
		// must never depend on it, or the gate's determinism/offline guarantee
		// would be at risk.
		"github.com/chasedputnam/memphis/internal/changegate",
		// Change-risk and its git-intelligence substrate likewise depend on
		// codeintel/store and must stay outside the authority path.
		"github.com/chasedputnam/memphis/internal/changerisk",
		"github.com/chasedputnam/memphis/internal/gitint",
		// The code dependency graph depends on codeintel and must stay outside
		// the authority path.
		"github.com/chasedputnam/memphis/internal/codegraph",
		// The code-health layer composes codeintel/gitint/codegraph/store and
		// must stay outside the authority path.
		"github.com/chasedputnam/memphis/internal/codehealth",
		// Dead-code consumes codegraph reachability and must stay outside the
		// authority path.
		"github.com/chasedputnam/memphis/internal/deadcode",
	}
	for _, dep := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		for _, bad := range forbidden {
			if dep == bad || strings.HasPrefix(dep, bad+"/") {
				t.Errorf("internal/canon must not depend on %q (code-intelligence in the authority path)", dep)
			}
		}
	}
}
