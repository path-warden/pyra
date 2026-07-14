package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/config"
)

func gitStore(t *testing.T) Context {
	t.Helper()
	store := t.TempDir()
	if err := os.MkdirAll(filepath.Join(store, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	return Context{StoreRoot: store, Config: config.Default()}
}

// runHookScript runs a generated hook with a stub `pyra` on PATH that exits
// with pyraExit, returning the script's exit code.
func runHookScript(t *testing.T, scriptPath string, pyraExit int) int {
	t.Helper()
	bin := t.TempDir()
	stub := filepath.Join(bin, "pyra")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit "+strconv.Itoa(pyraExit)+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", scriptPath)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
	err := cmd.Run()
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	t.Fatalf("run script: %v", err)
	return -1
}

func TestGit_FreshCreate(t *testing.T) {
	ctx := gitStore(t)
	g := gitInstaller{}
	if !g.Detect(ctx) {
		t.Fatal("Detect should be true when .git exists")
	}
	if _, err := g.Install(ctx); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"pre-commit", "post-merge"} {
		p := filepath.Join(ctx.StoreRoot, ".git", "hooks", name)
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("%s not created: %v", name, err)
		}
		if fi.Mode().Perm()&0o100 == 0 {
			t.Errorf("%s is not executable: %v", name, fi.Mode())
		}
		body, _ := os.ReadFile(p)
		if !strings.Contains(string(body), "command -v pyra") {
			t.Errorf("%s missing PATH check:\n%s", name, body)
		}
	}
	pre, _ := os.ReadFile(filepath.Join(ctx.StoreRoot, ".git", "hooks", "pre-commit"))
	if !strings.Contains(string(pre), "pyra gate") {
		t.Errorf("pre-commit should run gate:\n%s", pre)
	}
	post, _ := os.ReadFile(filepath.Join(ctx.StoreRoot, ".git", "hooks", "post-merge"))
	if !strings.Contains(string(post), "pyra rebuild") {
		t.Errorf("post-merge should run rebuild:\n%s", post)
	}
}

func TestGit_AppendPreservesUserScript(t *testing.T) {
	ctx := gitStore(t)
	pre := filepath.Join(ctx.StoreRoot, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(pre, []byte("#!/bin/sh\necho user-hook\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (gitInstaller{}).Install(ctx); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(pre)
	if !strings.Contains(string(body), "echo user-hook") {
		t.Errorf("user hook content lost:\n%s", body)
	}
	if !hasBlock(string(body)) {
		t.Errorf("pyra block not added:\n%s", body)
	}
}

func TestGit_Idempotent(t *testing.T) {
	ctx := gitStore(t)
	g := gitInstaller{}
	if _, err := g.Install(ctx); err != nil {
		t.Fatal(err)
	}
	pre := filepath.Join(ctx.StoreRoot, ".git", "hooks", "pre-commit")
	first, _ := os.ReadFile(pre)
	if _, err := g.Install(ctx); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(pre)
	if string(first) != string(second) {
		t.Errorf("re-install changed content:\n--1--\n%s\n--2--\n%s", first, second)
	}
	if n := strings.Count(string(second), BlockBegin); n != 1 {
		t.Errorf("expected exactly one block, got %d", n)
	}
}

func TestGit_Uninstall(t *testing.T) {
	ctx := gitStore(t)
	pre := filepath.Join(ctx.StoreRoot, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(pre, []byte("#!/bin/sh\necho user-hook\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	g := gitInstaller{}
	if _, err := g.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Uninstall(ctx); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(pre)
	if hasBlock(string(body)) {
		t.Errorf("uninstall left the pyra block:\n%s", body)
	}
	if !strings.Contains(string(body), "echo user-hook") {
		t.Errorf("uninstall removed user content:\n%s", body)
	}
}

func TestGit_PreCommitExitCodes(t *testing.T) {
	ctx := gitStore(t)
	if _, err := (gitInstaller{}).Install(ctx); err != nil {
		t.Fatal(err)
	}
	pre := filepath.Join(ctx.StoreRoot, ".git", "hooks", "pre-commit")
	if code := runHookScript(t, pre, 0); code != 0 {
		t.Errorf("pre-commit should exit 0 when gate passes, got %d", code)
	}
	if code := runHookScript(t, pre, 1); code != 1 {
		t.Errorf("pre-commit should exit 1 when gate fails, got %d", code)
	}
}

func TestGit_PostMergeExitsZeroOnFailure(t *testing.T) {
	ctx := gitStore(t)
	if _, err := (gitInstaller{}).Install(ctx); err != nil {
		t.Fatal(err)
	}
	post := filepath.Join(ctx.StoreRoot, ".git", "hooks", "post-merge")
	if code := runHookScript(t, post, 1); code != 0 {
		t.Errorf("post-merge must not abort on guard failure, got exit %d", code)
	}
}
