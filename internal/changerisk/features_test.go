package changerisk

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Ann", "GIT_AUTHOR_EMAIL=ann@e",
		"GIT_COMMITTER_NAME=Ann", "GIT_COMMITTER_EMAIL=ann@e")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeF(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseNumstat_DirsSubsystemsBinary(t *testing.T) {
	numstat := "10\t2\tpkg/a/x.go\n5\t0\tpkg/b/y.go\n-\t-\tassets/logo.png\n"
	changes := parseNumstat(numstat, nil)
	f := featuresFromChanges(changes, nil, "", "", "")
	if f.LA != 15 { // 10 + 5 + 0 (binary)
		t.Errorf("LA = %d, want 15", f.LA)
	}
	if f.LD != 2 {
		t.Errorf("LD = %d, want 2", f.LD)
	}
	if f.NF != 3 {
		t.Errorf("NF = %d, want 3", f.NF)
	}
	if f.ND != 3 { // pkg/a, pkg/b, assets
		t.Errorf("ND = %d, want 3", f.ND)
	}
	if f.NS != 2 { // pkg, assets
		t.Errorf("NS = %d, want 2", f.NS)
	}
}

func TestParseNumstat_ExtensionFilter(t *testing.T) {
	numstat := "10\t2\ta.go\n5\t0\tb.py\n"
	f := featuresFromChanges(parseNumstat(numstat, []string{".go"}), nil, "", "", "")
	if f.NF != 1 || f.LA != 10 {
		t.Errorf("ext filter failed: NF=%d LA=%d, want 1/10", f.NF, f.LA)
	}
}

func TestEntropy_EdgeCases(t *testing.T) {
	if got := entropy(nil); got != 0 {
		t.Errorf("entropy(nil) = %v, want 0", got)
	}
	if got := entropy([]int{5}); got != 0 { // <2 files
		t.Errorf("entropy(single) = %v, want 0", got)
	}
	if got := entropy([]int{0, 0}); got != 0 { // zero churn
		t.Errorf("entropy(zeros) = %v, want 0", got)
	}
	// Two equal buckets → max entropy of 1 bit.
	if got := entropy([]int{4, 4}); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("entropy(4,4) = %v, want 1.0", got)
	}
}

func TestExtractStaged_NoAuthorExp(t *testing.T) {
	root := t.TempDir()
	gitT(t, root, "init")
	writeF(t, root, "a.go", "package a\n")
	gitT(t, root, "add", "a.go")

	f := ExtractStaged(root, nil)
	if f.NF != 1 {
		t.Errorf("staged NF = %d, want 1", f.NF)
	}
	if f.Exp != nil {
		t.Errorf("staged Exp should be nil (no author context), got %v", *f.Exp)
	}
}

func TestExtractCommit_HasExp(t *testing.T) {
	root := t.TempDir()
	gitT(t, root, "init")
	writeF(t, root, "a.go", "package a\n")
	gitT(t, root, "add", ".")
	gitT(t, root, "commit", "-m", "one")
	writeF(t, root, "a.go", "package a\n\nvar X = 1\n")
	gitT(t, root, "add", ".")
	gitT(t, root, "commit", "-m", "two")

	f := ExtractCommit(root, "HEAD", nil)
	if f.NF != 1 {
		t.Errorf("commit NF = %d, want 1", f.NF)
	}
	if f.Exp == nil {
		t.Fatal("commit Exp should be known")
	}
	if *f.Exp != 1 { // one prior commit before HEAD by Ann
		t.Errorf("Exp = %d, want 1", *f.Exp)
	}
	if f.Author != "Ann" {
		t.Errorf("author = %q, want Ann", f.Author)
	}
}
