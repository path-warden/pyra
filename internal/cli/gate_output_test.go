package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chasedputnam/memphis/internal/changegate"
	"github.com/chasedputnam/memphis/internal/config"
	"github.com/chasedputnam/memphis/internal/sarif"
)

func TestGateOutput_JSONIncludesGovernanceFields(t *testing.T) {
	root := t.TempDir()
	writeGateFile(t, root, "canon/d.md", governedDecision("OKF-000000000AAA"))

	res, _, err := computeGate(root, config.Default(),
		changegate.Source{Kind: changegate.SourceExplicit, Files: []string{"internal/cache/store.go"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	for _, want := range []string{
		changegate.CodeGovernedChange, // code
		"internal/cache/store.go",     // changed file (path)
		"OKF-000000000AAA",            // governing artifact id (in message)
		"warning",                     // severity
	} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON missing %q\n%s", want, out)
		}
	}
}

func TestGateOutput_SARIFLocationAndRuleID(t *testing.T) {
	root := t.TempDir()
	writeGateFile(t, root, "canon/d.md", governedDecision("OKF-000000000AAA"))

	res, _, err := computeGate(root, config.Default(),
		changegate.Source{Kind: changegate.SourceExplicit, Files: []string{"internal/cache/store.go"}}, true)
	if err != nil {
		t.Fatal(err)
	}

	doc := sarif.FromIssues("memphis", "test", res.Issues)
	var found bool
	for _, run := range doc.Runs {
		for _, r := range run.Results {
			if r.RuleID != changegate.CodeGovernedChange {
				continue
			}
			found = true
			if len(r.Locations) == 0 {
				t.Fatal("governance result has no location")
			}
			if uri := r.Locations[0].PhysicalLocation.ArtifactLocation.URI; uri != "internal/cache/store.go" {
				t.Errorf("SARIF location URI = %q, want the changed file", uri)
			}
		}
	}
	if !found {
		t.Errorf("no SARIF result with rule id %q", changegate.CodeGovernedChange)
	}
}

func TestGateOutput_StableAcrossRuns(t *testing.T) {
	root := t.TempDir()
	writeGateFile(t, root, "canon/a.md", governedDecision("OKF-000000000AAA"))

	src := changegate.Source{Kind: changegate.SourceExplicit, Files: []string{"internal/cache/store.go"}}
	res1, _, err := computeGate(root, config.Default(), src, true)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := json.Marshal(res1)
	for i := 0; i < 3; i++ {
		res2, _, err := computeGate(root, config.Default(), src, true)
		if err != nil {
			t.Fatal(err)
		}
		again, _ := json.Marshal(res2)
		if string(first) != string(again) {
			t.Fatalf("gate output not stable across runs:\n%s\nvs\n%s", first, again)
		}
	}
}
