package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/deadcode"
)

func writeDC(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func deadCodeRepo(t *testing.T) string {
	root := t.TempDir()
	writeDC(t, root, "app.go", "package p\n\nfunc Used() { helper() }\n\nfunc helper() {}\n\nfunc orphan() { return }\n")
	return root
}

func TestRunDeadCode_JSON(t *testing.T) {
	root := deadCodeRepo(t)
	out := captureStdout(t, func() {
		deadCodeCmd.Flags().Set("json", "true")
		if err := runDeadCode(deadCodeCmd, []string{root}); err != nil {
			t.Fatal(err)
		}
		deadCodeCmd.Flags().Set("json", "false")
	})
	var payload struct {
		Candidates  []deadcode.Candidate `json:"candidates"`
		TotalImpact int                  `json:"total_impact"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("dead-code JSON did not parse: %v\n%s", err, out)
	}
	found := false
	for _, c := range payload.Candidates {
		if c.Name == "orphan" {
			found = true
		}
		if c.Name == "helper" || c.Name == "Used" {
			t.Errorf("reachable/exported symbol %s should not be dead", c.Name)
		}
	}
	if !found {
		t.Errorf("orphan should be reported dead: %+v", payload.Candidates)
	}
}

func TestRunDeadCode_TierFilter(t *testing.T) {
	root := deadCodeRepo(t)
	out := captureStdout(t, func() {
		deadCodeCmd.Flags().Set("tier", "low")
		deadCodeCmd.Flags().Set("json", "true")
		if err := runDeadCode(deadCodeCmd, []string{root}); err != nil {
			t.Fatal(err)
		}
		deadCodeCmd.Flags().Set("tier", "")
		deadCodeCmd.Flags().Set("json", "false")
	})
	var payload struct {
		Candidates []deadcode.Candidate `json:"candidates"`
	}
	json.Unmarshal([]byte(out), &payload)
	// orphan is high-tier, so a low filter excludes it.
	for _, c := range payload.Candidates {
		if c.Tier != "low" {
			t.Errorf("--tier low returned a %s candidate", c.Tier)
		}
	}
}

func TestRunDeadCode_Text(t *testing.T) {
	root := deadCodeRepo(t)
	out := captureStdout(t, func() {
		if err := runDeadCode(deadCodeCmd, []string{root}); err != nil {
			t.Fatal(err)
		}
	})
	// The candidate rows are colorized (fatih/color bypasses the stdout capture);
	// assert on the plain header + summary, which the JSON test corroborates.
	if !strings.Contains(out, "dead-code") || !strings.Contains(out, "cleanup impact") {
		t.Errorf("text output should show the header + impact summary:\n%s", out)
	}
	if !strings.Contains(out, "candidate") {
		t.Errorf("text output should report a candidate count:\n%s", out)
	}
}
