package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/changegate"
	"github.com/chasedputnam/pyra/internal/changerisk"
	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/sarif"
)

func TestRiskOutput_JSONAndSARIF(t *testing.T) {
	root := stageRiskRepo(t)
	res, _, err := computeGate(root, config.Default(),
		changegate.Source{Kind: changegate.SourceStaged}, true, true)
	if err != nil {
		t.Fatal(err)
	}

	// JSON carries the risk headline code and a directive.
	data, _ := json.Marshal(res)
	out := string(data)
	if !strings.Contains(out, changerisk.CodeRisk) {
		t.Errorf("JSON missing %s headline\n%s", changerisk.CodeRisk, out)
	}
	if !strings.Contains(out, changerisk.CodeGovernanceRisk) {
		t.Errorf("JSON missing %s directive", changerisk.CodeGovernanceRisk)
	}

	// SARIF: each risk directive rule id equals its code and its location is the
	// changed file (directives carry a Path).
	doc := sarif.FromIssues("pyra", "test", res.Issues)
	var sawDirectiveLoc bool
	for _, run := range doc.Runs {
		for _, r := range run.Results {
			if r.RuleID == changerisk.CodeGovernanceRisk {
				if len(r.Locations) == 0 || r.Locations[0].PhysicalLocation.ArtifactLocation.URI == "" {
					t.Errorf("%s should carry a changed-file location", r.RuleID)
				} else {
					sawDirectiveLoc = true
				}
			}
		}
	}
	if !sawDirectiveLoc {
		t.Error("expected a governance-risk SARIF result with a location")
	}
}

func TestRiskOutput_StableAcrossRuns(t *testing.T) {
	root := stageRiskRepo(t)
	run := func() string {
		res, _, err := computeGate(root, config.Default(),
			changegate.Source{Kind: changegate.SourceStaged}, true, true)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(res)
		return string(b)
	}
	first := run()
	for i := 0; i < 3; i++ {
		if again := run(); again != first {
			t.Fatalf("risk gate output not stable:\n%s\nvs\n%s", first, again)
		}
	}
}
