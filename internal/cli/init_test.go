package cli

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/store"
)

// newInitCmd builds a fresh command with init's flags so each test gets isolated
// flag state (the production initCmd is a shared global).
func newInitCmd() *cobra.Command {
	c := &cobra.Command{Use: "init [path]", Args: cobra.MaximumNArgs(1), RunE: runInit,
		SilenceUsage: true, SilenceErrors: true}
	c.Flags().String("repository-key", "", "")
	c.Flags().StringArray("canon-root", nil, "")
	c.Flags().String("ticketing", "github", "")
	c.Flags().Bool("force", false, "")
	c.Flags().Bool("quiet", false, "")
	return c
}

func runInitT(t *testing.T, args []string, flags map[string]string, multi map[string][]string) error {
	t.Helper()
	c := newInitCmd()
	for k, v := range flags {
		if err := c.Flags().Set(k, v); err != nil {
			t.Fatalf("set flag %s: %v", k, err)
		}
	}
	for k, vs := range multi {
		for _, v := range vs {
			if err := c.Flags().Set(k, v); err != nil {
				t.Fatalf("set flag %s: %v", k, err)
			}
		}
	}
	return runInit(c, args)
}

func TestInit_HappyPath(t *testing.T) {
	dir := t.TempDir()
	if err := runInitT(t, []string{dir}, nil, nil); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(cfg, config.Default()) {
		t.Errorf("loaded config %+v != Default() %+v", cfg, config.Default())
	}
	if fi, err := os.Stat(filepath.Join(dir, "canon")); err != nil || !fi.IsDir() {
		t.Errorf("canon dir not created: %v", err)
	}

	// Downstream commands operate without a missing-config error (Requirement 1).
	s, err := store.Load(dir, cfg)
	if err != nil {
		t.Fatalf("store.Load after init: %v", err)
	}
	defer func() { _ = s.Close() }()
	if s.HasCanon() {
		t.Errorf("fresh store should have no Canon, got %d artifacts", len(s.Canon))
	}
}

func TestInit_CreatesNestedParents(t *testing.T) {
	root := filepath.Join(t.TempDir(), "a", "b", "store")
	if err := runInitT(t, []string{root}, nil, nil); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if _, err := os.Stat(config.Path(root)); err != nil {
		t.Errorf("config not created at nested path: %v", err)
	}
}

func TestInit_Overrides(t *testing.T) {
	dir := t.TempDir()
	err := runInitT(t, []string{dir},
		map[string]string{"repository-key": "PROJ", "ticketing": "jira"},
		map[string][]string{"canon-root": {"rac", "decisions"}})
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RepositoryKey != "PROJ" {
		t.Errorf("RepositoryKey=%q", cfg.RepositoryKey)
	}
	if !reflect.DeepEqual(cfg.CanonRoots, []string{"rac", "decisions"}) {
		t.Errorf("CanonRoots=%v", cfg.CanonRoots)
	}
	if cfg.Ticketing.Provider != "jira" {
		t.Errorf("Provider=%q", cfg.Ticketing.Provider)
	}
	for _, r := range []string{"rac", "decisions"} {
		if fi, err := os.Stat(filepath.Join(dir, r)); err != nil || !fi.IsDir() {
			t.Errorf("canon root %q not created: %v", r, err)
		}
	}
}

func TestInit_DedupesCanonRoots(t *testing.T) {
	dir := t.TempDir()
	if err := runInitT(t, []string{dir}, nil, map[string][]string{"canon-root": {"canon", "canon"}}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(dir)
	if !reflect.DeepEqual(cfg.CanonRoots, []string{"canon"}) {
		t.Errorf("expected deduped [canon], got %v", cfg.CanonRoots)
	}
}

func TestInit_ValidationRejects(t *testing.T) {
	cases := []struct {
		name  string
		flags map[string]string
		multi map[string][]string
	}{
		{"unknown provider", map[string]string{"ticketing": "bogus"}, nil},
		{"empty key", map[string]string{"repository-key": ""}, nil},
		{"absolute canon root", nil, map[string][]string{"canon-root": {string(os.PathSeparator) + "etc"}}},
		{"escaping canon root", nil, map[string][]string{"canon-root": {filepath.Join("..", "evil")}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			err := runInitT(t, []string{dir}, tc.flags, tc.multi)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			// No partial store: .okf must not exist.
			if _, statErr := os.Stat(filepath.Join(dir, ".okf")); !os.IsNotExist(statErr) {
				t.Errorf(".okf created despite validation failure (stat err: %v)", statErr)
			}
		})
	}
}

func TestInit_RefusesToClobberWithoutForce(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".okf"), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := []byte("repository_key: CUSTOM\ncanon_roots: [mine]\n")
	if err := os.WriteFile(config.Path(dir), custom, 0o644); err != nil {
		t.Fatal(err)
	}

	err := runInitT(t, []string{dir}, nil, nil)
	if err == nil {
		t.Fatal("expected error when store exists without --force")
	}
	got, _ := os.ReadFile(config.Path(dir))
	if !reflect.DeepEqual(got, custom) {
		t.Errorf("existing config modified without --force:\n%s", got)
	}
}

func TestInit_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".okf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.Path(dir), []byte("repository_key: CUSTOM\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runInitT(t, []string{dir}, map[string]string{"force": "true"}, nil); err != nil {
		t.Fatalf("forced init failed: %v", err)
	}
	cfg, _ := config.Load(dir)
	if cfg.RepositoryKey != "OKF" {
		t.Errorf("expected overwritten config (OKF), got %q", cfg.RepositoryKey)
	}
}

func TestInit_PreservesExistingCanonContents(t *testing.T) {
	dir := t.TempDir()
	canonDir := filepath.Join(dir, "canon")
	if err := os.MkdirAll(canonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(canonDir, "keep.md")
	if err := os.WriteFile(keep, []byte("# keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runInitT(t, []string{dir}, map[string]string{"force": "true"}, nil); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	got, err := os.ReadFile(keep)
	if err != nil || string(got) != "# keep me" {
		t.Errorf("existing canon file not preserved: %q err=%v", got, err)
	}
}

func TestInit_QuietSuppressesOutput(t *testing.T) {
	dir := t.TempDir()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := runInitT(t, []string{dir}, map[string]string{"quiet": "true"}, nil)
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("quiet init failed: %v", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("expected no stdout with --quiet, got: %q", out)
	}
	// Side effect still happened.
	if _, err := os.Stat(config.Path(dir)); err != nil {
		t.Errorf("config not written under --quiet: %v", err)
	}
}
