package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/chasedputnam/pyra/internal/gitint"
)

func gitAt(t *testing.T, root string, ts int64, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	date := "@" + strconv.FormatInt(ts, 10) + " +0000"
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Ann", "GIT_AUTHOR_EMAIL=ann@e",
		"GIT_COMMITTER_NAME=Ann", "GIT_COMMITTER_EMAIL=ann@e",
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func wf(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// hotRepo builds a repo where internal/hot.go is a clear churn hotspot.
func hotRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitAt(t, root, 1, "init")
	// Several quiet files so the churned file lands clearly in the top quartile.
	wf(t, root, "internal/cool.go", "package p\n")
	wf(t, root, "internal/util.go", "package p\n")
	wf(t, root, "cmd/main.go", "package main\n")
	wf(t, root, "docs/readme.md", "# docs\n")
	gitAt(t, root, 1000, "add", ".")
	gitAt(t, root, 1000, "commit", "-m", "seed")
	// Many recent commits to internal/hot.go.
	for i := 0; i < 8; i++ {
		wf(t, root, "internal/hot.go", "package p\n// v"+strconv.Itoa(i)+"\n")
		gitAt(t, root, int64(2000+i), "add", ".")
		gitAt(t, root, int64(2000+i), "commit", "-m", "hot "+strconv.Itoa(i))
	}
	return root
}

func TestGitIntIndex_HotspotsAndOwnership(t *testing.T) {
	root := hotRepo(t)
	h, ok := gitint.New(root, 100)
	if !ok {
		t.Fatal("index should build")
	}
	hot := h.Hotspots()
	if len(hot) == 0 || hot[0].Path != "internal/hot.go" {
		t.Fatalf("top hotspot = %+v, want internal/hot.go", hot)
	}
	own := h.OwnershipAt("internal/hot.go")
	if own.PrimaryOwner != "Ann" || own.BusFactor != 1 {
		t.Errorf("ownership = %+v, want owner Ann bus 1", own)
	}
	// Directory path → module rollup.
	mod := h.OwnershipAt("internal")
	if !mod.IsModule || mod.Module == nil || mod.Module.Name != "internal" {
		t.Errorf("module ownership = %+v, want internal module", mod)
	}
}

func TestRunHotspots_JSONAndNonGit(t *testing.T) {
	// JSON over a real repo parses to a slice.
	root := hotRepo(t)
	out := captureStdout(t, func() {
		cmd := hotspotsCmd
		if err := cmd.Flags().Set("json", "true"); err != nil {
			t.Fatal(err)
		}
		if err := runHotspots(cmd, []string{root}); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Flags().Set("json", "false"); err != nil {
			t.Fatal(err)
		}
	})
	var hot []gitint.FileHistory
	if err := json.Unmarshal([]byte(out), &hot); err != nil {
		t.Fatalf("hotspots JSON did not parse: %v\n%s", err, out)
	}
	if len(hot) == 0 {
		t.Error("expected at least one hotspot in JSON")
	}

	// Non-git dir → clean exit, no error.
	if err := runHotspots(hotspotsCmd, []string{t.TempDir()}); err != nil {
		t.Errorf("non-git hotspots should exit cleanly, got %v", err)
	}
}
