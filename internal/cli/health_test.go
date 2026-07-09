package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chasedputnam/memphis/internal/codehealth"
)

func writeHealthFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func healthRepo(t *testing.T) string {
	root := t.TempDir()
	var b strings.Builder
	b.WriteString("package p\n\nfunc huge(a int) int {\n")
	for i := 0; i < 80; i++ {
		b.WriteString("\tif a>0 { if a>1 { if a>2 { if a>3 { if a>4 { a++ } } } } }\n")
	}
	b.WriteString("\treturn a\n}\n")
	writeHealthFile(t, root, "bad.go", b.String())
	writeHealthFile(t, root, "good.go", "package p\n\nfunc Ok() int { return 1 }\n")
	return root
}

func TestRunHealth_JSON(t *testing.T) {
	root := healthRepo(t)
	out := captureStdout(t, func() {
		healthCmd.Flags().Set("json", "true")
		if err := runHealth(healthCmd, []string{root}); err != nil {
			t.Fatal(err)
		}
		healthCmd.Flags().Set("json", "false")
	})
	var rep codehealth.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("health JSON did not parse: %v\n%s", err, out)
	}
	if rep.FileCount != 2 || len(rep.Files) == 0 {
		t.Errorf("expected 2 files scored, got %+v", rep)
	}
	if rep.Files[0].Path != "bad.go" || rep.Files[0].Defect >= 10 {
		t.Errorf("worst file should be bad.go with low defect, got %+v", rep.Files[0])
	}
}

func TestRunHealth_FileView(t *testing.T) {
	root := healthRepo(t)
	out := captureStdout(t, func() {
		healthCmd.Flags().Set("file", "bad.go")
		if err := runHealth(healthCmd, []string{root}); err != nil {
			t.Fatal(err)
		}
		healthCmd.Flags().Set("file", "")
	})
	if !strings.Contains(out, "bad.go") || !strings.Contains(out, "findings") {
		t.Errorf("--file view should show findings for bad.go:\n%s", out)
	}
}

func TestRunHealth_CoverageIngestion(t *testing.T) {
	root := healthRepo(t)
	cov := filepath.Join(root, "cov.info")
	os.WriteFile(cov, []byte("SF:bad.go\nDA:1,0\nDA:2,0\nDA:3,0\nend_of_record\n"), 0o644)
	out := captureStdout(t, func() {
		healthCmd.Flags().Set("json", "true")
		healthCmd.Flags().Set("coverage", cov)
		if err := runHealth(healthCmd, []string{root}); err != nil {
			t.Fatal(err)
		}
		healthCmd.Flags().Set("coverage", "")
		healthCmd.Flags().Set("json", "false")
	})
	if !strings.Contains(out, "coverage_gap") && !strings.Contains(out, "coverage_gradient") {
		t.Errorf("coverage ingestion should add coverage findings:\n%s", out)
	}
}
