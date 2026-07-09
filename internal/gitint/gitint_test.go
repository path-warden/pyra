package gitint

import (
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

func commit(t *testing.T, root, msg string, files map[string]string) {
	for rel, content := range files {
		writeF(t, root, rel, content)
	}
	gitT(t, root, "add", ".")
	gitT(t, root, "commit", "-m", msg)
}

func TestHistory_ChurnAndCoChange(t *testing.T) {
	root := t.TempDir()
	gitT(t, root, "init")
	// a.go & b.go change together twice; c.go changes alone once.
	commit(t, root, "c1", map[string]string{"a.go": "1", "b.go": "1"})
	commit(t, root, "c2", map[string]string{"a.go": "2", "b.go": "2"})
	commit(t, root, "c3", map[string]string{"c.go": "1"})

	h, ok := New(root, 100)
	if !ok {
		t.Fatal("New should succeed in a git repo")
	}
	if got := h.Churn("a.go"); got != 2 {
		t.Errorf("churn a.go = %d, want 2", got)
	}
	if got := h.Churn("c.go"); got != 1 {
		t.Errorf("churn c.go = %d, want 1", got)
	}
	partners := h.CoChangePartners("a.go")
	if len(partners) != 1 || partners[0].Path != "b.go" || partners[0].Count != 2 {
		t.Errorf("a.go partners = %+v, want [{b.go 2}]", partners)
	}
	if p := h.CoChangePartners("c.go"); len(p) != 0 {
		t.Errorf("c.go has no co-change partners, got %+v", p)
	}
}

func TestHistory_AuthorCommits(t *testing.T) {
	root := t.TempDir()
	gitT(t, root, "init")
	commit(t, root, "c1", map[string]string{"a.go": "1"})
	commit(t, root, "c2", map[string]string{"a.go": "2"})
	h, _ := New(root, 100)
	if got := h.AuthorCommits("Ann", "HEAD"); got != 2 {
		t.Errorf("AuthorCommits(Ann) = %d, want 2", got)
	}
	if got := h.AuthorCommits("", "HEAD"); got != 0 {
		t.Errorf("AuthorCommits(\"\") = %d, want 0", got)
	}
}

func TestHistory_Deterministic(t *testing.T) {
	root := t.TempDir()
	gitT(t, root, "init")
	commit(t, root, "c1", map[string]string{"a.go": "1", "b.go": "1"})
	commit(t, root, "c2", map[string]string{"a.go": "2", "c.go": "1"})

	h1, _ := New(root, 100)
	h2, _ := New(root, 100)
	for _, f := range []string{"a.go", "b.go", "c.go"} {
		if h1.Churn(f) != h2.Churn(f) {
			t.Errorf("churn nondeterministic for %s", f)
		}
		p1, p2 := h1.CoChangePartners(f), h2.CoChangePartners(f)
		if len(p1) != len(p2) {
			t.Fatalf("partner count differs for %s", f)
		}
		for i := range p1 {
			if p1[i] != p2[i] {
				t.Errorf("partner order differs for %s: %+v vs %+v", f, p1, p2)
			}
		}
	}
}

func TestHistory_NonGitDegrades(t *testing.T) {
	root := t.TempDir() // not a git repo
	h, ok := New(root, 100)
	if ok {
		t.Error("New should return ok=false outside a git repo")
	}
	if h.Churn("a.go") != 0 || len(h.CoChangePartners("a.go")) != 0 {
		t.Error("degraded history should be empty, not panic")
	}
}

func TestIsFixCommit_WordBoundary(t *testing.T) {
	fix := []string{"fix: cache bug", "Fix the thing", "revert bad change", "bug in parser", "hotfix deploy"}
	notFix := []string{"add prefix handling", "improve debug logging", "dispatch events", "refactor suffix parser", "new feature"}
	for _, s := range fix {
		if !isFixCommit(s) {
			t.Errorf("%q should be a fix commit", s)
		}
	}
	for _, s := range notFix {
		if isFixCommit(s) {
			t.Errorf("%q should NOT be a fix commit (substring false positive)", s)
		}
	}
}
