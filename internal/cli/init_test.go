package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/hooks"
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
	c.Flags().StringArray("agent", nil, "")
	c.Flags().Bool("agents-only", false, "")
	c.Flags().String("kiro-agent", "", "")
	c.Flags().Bool("list-agents", false, "")
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
	if _, statErr := os.Stat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(statErr) {
		t.Errorf("agent setup changed before existing-store refusal: %v", statErr)
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

func TestInit_AgentsOnlyPreservesStoreAndHooks(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".okf"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Deliberately malformed: agent-only mode must preserve this file without
	// even attempting to parse it.
	configBody := []byte("not: [valid\n")
	if err := os.WriteFile(config.Path(dir), configBody, 0o644); err != nil {
		t.Fatal(err)
	}
	canonPath := filepath.Join(dir, "authority", "keep.md")
	if err := os.MkdirAll(filepath.Dir(canonPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(canonPath, []byte("# keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho keep\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	codexHookPath := filepath.Join(dir, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(codexHookPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexHookPath, []byte(`{"hooks":"keep"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runInitT(t, []string{dir}, map[string]string{"agents-only": "true"}, map[string][]string{
		"agent": {"claude", "codex"},
	})
	if err != nil {
		t.Fatalf("agents-only init failed: %v", err)
	}

	for path, want := range map[string][]byte{
		config.Path(dir): configBody,
		canonPath:        []byte("# keep"),
		hookPath:         []byte("#!/bin/sh\necho keep\n"),
		codexHookPath:    []byte(`{"hooks":"keep"}`),
	} {
		got, readErr := os.ReadFile(path)
		if readErr != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("agents-only changed %s: got=%q want=%q err=%v", path, got, want, readErr)
		}
	}
	for _, rel := range []string{"AGENTS.md", ".mcp.json", filepath.Join(".codex", "config.toml")} {
		if _, statErr := os.Stat(filepath.Join(dir, rel)); statErr != nil {
			t.Errorf("agents-only did not create %s: %v", rel, statErr)
		}
	}
}

func TestInit_AgentsOnlyRequiresAgentAndRejectsStoreFlags(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags map[string]string
		multi map[string][]string
		want  string
	}{
		{name: "missing agent", flags: map[string]string{"agents-only": "true"}, want: "requires at least one --agent"},
		{name: "store flag", flags: map[string]string{"agents-only": "true", "force": "true"}, multi: map[string][]string{"agent": {"codex"}}, want: "--force cannot be used"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			err := runInitT(t, []string{dir}, tc.flags, tc.multi)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
			for _, rel := range []string{"AGENTS.md", ".okf", ".codex"} {
				if _, statErr := os.Stat(filepath.Join(dir, rel)); !os.IsNotExist(statErr) {
					t.Errorf("%s written despite validation error: %v", rel, statErr)
				}
			}
		})
	}
}

func TestInit_AgentsOnlyDoesNotCreateStoreOrHooks(t *testing.T) {
	dir := t.TempDir()
	if err := runInitT(t, []string{dir}, map[string]string{"agents-only": "true"}, map[string][]string{"agent": {"opencode"}}); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".okf", "canon", ".git"} {
		if _, statErr := os.Stat(filepath.Join(dir, rel)); !os.IsNotExist(statErr) {
			t.Errorf("agents-only created %s: %v", rel, statErr)
		}
	}
	for _, rel := range []string{"AGENTS.md", "opencode.json"} {
		if _, statErr := os.Stat(filepath.Join(dir, rel)); statErr != nil {
			t.Errorf("agents-only did not create %s: %v", rel, statErr)
		}
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

func TestInit_CreatesAgentsWithoutAgentSelection(t *testing.T) {
	dir := t.TempDir()
	if err := runInitT(t, []string{dir}, nil, nil); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"find_decisions", "get_context", "pyra gate .", "pyra relationships . --summary --validate"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("AGENTS.md missing %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("unselected MCP config created: %v", err)
	}
}

func TestInit_MultipleAgentsAndHooks(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := runInitT(t, []string{dir}, nil, map[string][]string{
		"agent": {"claude", "codex", "opencode", "pi", "kiro", "pi"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		".mcp.json", filepath.Join(".codex", "config.toml"), "opencode.json",
		filepath.Join(".pi", "settings.json"), filepath.Join(".kiro", "settings", "mcp.json"),
		filepath.Join(".git", "hooks", "pre-commit"), filepath.Join(".claude", "settings.json"),
		filepath.Join(".codex", "hooks.json"), filepath.Join(".kiro", "hooks", "pyra-gate.json"),
		filepath.Join(".kiro", "agents", "pyra.json"),
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("expected %s: %v", rel, err)
		}
	}
}

func TestInit_SummaryReportsSetupAndGuidance(t *testing.T) {
	dir := t.TempDir()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := runInitT(t, []string{dir}, nil, map[string][]string{"agent": {"claude", "codex", "opencode", "pi", "kiro"}})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Agent setup:", ".codex/config.toml", "Codex: trust", "Pi: trust", "restart Pi",
		"MCP tool: claude (Claude Code)", "MCP tool: codex (Codex)", "MCP tool: opencode (OpenCode)",
		"MCP tool: pi (Pi)", "MCP tool: kiro (Kiro)",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestInit_KiroAmbiguityFailsBeforeWritesAndCanBeSelected(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".kiro", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(agentsDir, name+".json"), []byte(`{"name":"`+name+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := runInitT(t, []string{dir}, nil, map[string][]string{"agent": {"kiro"}})
	if err == nil || !strings.Contains(err.Error(), "multiple agent configs") {
		t.Fatalf("expected Kiro ambiguity error, got %v", err)
	}
	if _, err := os.Stat(config.Path(dir)); !os.IsNotExist(err) {
		t.Fatalf("store written despite ambiguous Kiro hooks: %v", err)
	}

	if err := runInitT(t, []string{dir}, map[string]string{"kiro-agent": "a"}, map[string][]string{"agent": {"kiro"}}); err != nil {
		t.Fatalf("selected Kiro agent init failed: %v", err)
	}
	aBody, _ := os.ReadFile(filepath.Join(agentsDir, "a.json"))
	bBody, _ := os.ReadFile(filepath.Join(agentsDir, "b.json"))
	if !strings.Contains(string(aBody), hooks.ManagedMarker) || strings.Contains(string(bBody), hooks.ManagedMarker) {
		t.Fatalf("Kiro hook selection modified wrong files: a=%s b=%s", aBody, bBody)
	}
}

func TestInit_MalformedHookShapeFailsBeforeStoreWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"hooks":"keep"}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	err := runInitT(t, []string{dir}, nil, map[string][]string{"agent": {"claude"}})
	if err == nil || !strings.Contains(err.Error(), "hooks must be an object") {
		t.Fatalf("expected hook shape error, got %v", err)
	}
	if _, err := os.Stat(config.Path(dir)); !os.IsNotExist(err) {
		t.Fatalf("store written despite malformed hook shape: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Fatalf("malformed hook file changed: %s", got)
	}
}

type initTestInstaller struct {
	target hooks.Target
	result hooks.Result
	err    error
	calls  *int
}

func (i initTestInstaller) Target() hooks.Target                          { return i.target }
func (i initTestInstaller) Detect(hooks.Context) bool                     { return true }
func (i initTestInstaller) Uninstall(hooks.Context) (hooks.Result, error) { return hooks.Result{}, nil }
func (i initTestInstaller) Status(hooks.Context) (hooks.Result, error)    { return hooks.Result{}, nil }
func (i initTestInstaller) Install(hooks.Context) (hooks.Result, error) {
	(*i.calls)++
	return i.result, i.err
}

func TestApplyInitHooksReportsCompletedAndPending(t *testing.T) {
	calls := 0
	installers := []hooks.Installer{
		initTestInstaller{target: hooks.TargetGit, result: hooks.Result{Target: hooks.TargetGit, Paths: []string{"pre-commit"}}, calls: &calls},
		initTestInstaller{target: hooks.TargetCodex, err: errors.New("denied"), calls: &calls},
		initTestInstaller{target: hooks.TargetKiroIDE, calls: &calls},
	}
	_, completed, pending, err := applyInitHooks(hooks.Context{}, installers)
	if err == nil || len(completed) != 1 || completed[0] != "pre-commit" {
		t.Fatalf("unexpected hook progress: completed=%v pending=%v err=%v", completed, pending, err)
	}
	if strings.Join(pending, ",") != "hook:codex,hook:kiro-ide" || calls != 2 {
		t.Fatalf("pending hooks or call count wrong: pending=%v calls=%d", pending, calls)
	}
}

func TestInit_InvalidAgentWritesNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new-store")
	err := runInitT(t, []string{dir}, nil, map[string][]string{"agent": {"cursor"}})
	if err == nil || !strings.Contains(err.Error(), "supported") {
		t.Fatalf("expected supported-agent error, got %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("store path created after validation failure: %v", err)
	}
}

func TestInit_ListAgentsWritesNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new-store")
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := runInitT(t, []string{dir}, map[string]string{"list-agents": "true"}, nil)
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"claude", "codex", "opencode", "pi", "kiro"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("list output missing %q: %s", want, out)
		}
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("list-agents created store path: %v", err)
	}
}

func TestInit_MalformedAgentConfigFailsBeforeStoreWrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runInitT(t, []string{dir}, nil, map[string][]string{"agent": {"claude"}})
	if err == nil {
		t.Fatal("expected malformed MCP config error")
	}
	if _, err := os.Stat(config.Path(dir)); !os.IsNotExist(err) {
		t.Fatalf("store config written despite preflight failure: %v", err)
	}
}
