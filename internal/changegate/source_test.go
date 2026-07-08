package changegate

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

func TestChangedFiles_Staged(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	git(t, root, "init")
	writeFile(t, root, "a.go", "package a\n")
	writeFile(t, root, "b.go", "package b\n")
	git(t, root, "add", "a.go") // stage only a.go

	got, err := ChangedFiles(root, Source{Kind: SourceStaged})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("staged = %v, want %v", got, want)
	}
}

func TestChangedFiles_Since(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	git(t, root, "init")
	writeFile(t, root, "a.go", "package a\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "init")
	// Modify a tracked file after the commit.
	writeFile(t, root, "a.go", "package a\n\nvar X = 1\n")

	got, err := ChangedFiles(root, Source{Kind: SourceSince, Ref: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("since HEAD = %v, want %v", got, want)
	}
}

func TestChangedFiles_Explicit_NoGit(t *testing.T) {
	// Explicit source must work in a non-git directory and never invoke git.
	root := t.TempDir()
	got, err := ChangedFiles(root, Source{
		Kind:  SourceExplicit,
		Files: []string{"b.go", "a.go", "a.go", "  ", "pkg/c.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Sorted + de-duplicated + blanks dropped.
	want := []string{"a.go", "b.go", "pkg/c.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("explicit = %v, want %v", got, want)
	}
}

func TestChangedFiles_NoGit_Staged_Errors(t *testing.T) {
	root := t.TempDir() // not a git repo
	_, err := ChangedFiles(root, Source{Kind: SourceStaged})
	if err != ErrNoChangeSource {
		t.Errorf("err = %v, want ErrNoChangeSource", err)
	}
}

func TestChangedFiles_NestedStoreRoot_Normalizes(t *testing.T) {
	requireGit(t)
	repo := t.TempDir()
	git(t, repo, "init")
	// Store lives in a subdirectory of the git repo.
	store := filepath.Join(repo, "store")
	writeFile(t, repo, "store/canon/x.md", "# x\n")
	writeFile(t, repo, "outside.go", "package o\n") // outside the store root
	git(t, repo, "add", ".")

	got, err := ChangedFiles(store, Source{Kind: SourceStaged})
	if err != nil {
		t.Fatal(err)
	}
	// Only the in-store path survives, relative to the store root.
	if want := []string{"canon/x.md"}; !reflect.DeepEqual(got, want) {
		t.Errorf("nested = %v, want %v (outside.go must be dropped)", got, want)
	}
}
