package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/config"
)

func hooksStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".okf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.Path(dir), []byte(config.Render(config.Default())), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newHooksSubCmd(run func(*cobra.Command, []string) error) *cobra.Command {
	c := &cobra.Command{Args: cobra.NoArgs, RunE: run, SilenceUsage: true, SilenceErrors: true}
	c.Flags().String("store", ".", "")
	c.Flags().Bool("git", false, "")
	c.Flags().Bool("claude", false, "")
	c.Flags().Bool("kiro-ide", false, "")
	c.Flags().Bool("kiro-cli", false, "")
	c.Flags().Bool("kiro", false, "")
	c.Flags().Bool("all", false, "")
	c.Flags().String("kiro-agent", "", "")
	return c
}

func runHooksT(t *testing.T, run func(*cobra.Command, []string) error, store string, flags map[string]string) (string, error) {
	t.Helper()
	c := newHooksSubCmd(run)
	if store != "" {
		flags["store"] = store
	}
	for k, v := range flags {
		if err := c.Flags().Set(k, v); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}
	old := os.Stdout
	oldColor := color.Output
	r, w, _ := os.Pipe()
	os.Stdout = w
	color.Output = w
	err := run(c, nil)
	_ = w.Close()
	os.Stdout = old
	color.Output = oldColor
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String(), err
}

func TestHooksInstall_NotAStore(t *testing.T) {
	dir := t.TempDir() // no .okf/config.yaml
	_, err := runHooksT(t, runHooksInstall, dir, map[string]string{})
	if err == nil {
		t.Fatal("expected error when not in a store")
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".git", "hooks", "pre-commit")); !os.IsNotExist(statErr) {
		t.Error("nothing should be written outside a store")
	}
}

func TestHooksInstall_DefaultGitAlwaysPlusDetected(t *testing.T) {
	dir := hooksStore(t)
	_ = os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755) // git present
	_ = os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)       // claude detected
	// no .kiro

	if _, err := runHooksT(t, runHooksInstall, dir, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "hooks", "pre-commit")); err != nil {
		t.Error("git pre-commit should always be installed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); err != nil {
		t.Error("detected Claude target should be installed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".kiro", "hooks", "pyra-gate.json")); !os.IsNotExist(err) {
		t.Error("undetected Kiro IDE target should be skipped")
	}
}

func TestHooksInstall_ExplicitFlagOverridesDetection(t *testing.T) {
	dir := hooksStore(t)
	_ = os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755)

	if _, err := runHooksT(t, runHooksInstall, dir, map[string]string{"claude": "true"}); err != nil {
		t.Fatal(err)
	}
	// --claude selected explicitly even though no .claude dir existed.
	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); err != nil {
		t.Error("explicit --claude should install Claude even without detection")
	}
	// git not selected → not installed.
	if _, err := os.Stat(filepath.Join(dir, ".git", "hooks", "pre-commit")); !os.IsNotExist(err) {
		t.Error("git should not be installed when an explicit non-git target is chosen")
	}
}

func TestHooksUninstall_Reverses(t *testing.T) {
	dir := hooksStore(t)
	_ = os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755)
	if _, err := runHooksT(t, runHooksInstall, dir, map[string]string{"git": "true"}); err != nil {
		t.Fatal(err)
	}
	if _, err := runHooksT(t, runHooksUninstall, dir, map[string]string{"git": "true"}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".git", "hooks", "pre-commit"))
	if strings.Contains(string(body), "pyra gate") {
		t.Errorf("uninstall should remove the pyra hook:\n%s", body)
	}
}

func TestHooksStatus_Reports(t *testing.T) {
	dir := hooksStore(t)
	_ = os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755)
	if _, err := runHooksT(t, runHooksInstall, dir, map[string]string{"git": "true"}); err != nil {
		t.Fatal(err)
	}
	out, err := runHooksT(t, runHooksStatus, dir, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "git") || !strings.Contains(strings.ToLower(out), "present") {
		t.Errorf("status should report git present:\n%s", out)
	}
}
