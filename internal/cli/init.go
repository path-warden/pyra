package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/agents"
	"github.com/chasedputnam/pyra/internal/canon/validate"
	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/hooks"
)

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Initialize a store and repository-local agent integrations",
	Long: `Initialize a pyra store: write a self-documenting .okf/config.yaml and
create the canon root directories, AGENTS.md authority workflow, selected agent
MCP configuration, and applicable repository-local gate hooks.

The defaults match pyra's implicit configuration, so an initialized store and
a config-less store behave identically. init is safe by default: it refuses to
overwrite an existing store unless --force is given.`,
	Args: cobra.MaximumNArgs(1),
	// init owns its own error/usage output so it can route messages to stderr.
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().String("repository-key", "", "Repository key for minted Canon IDs (default: OKF)")
	initCmd.Flags().StringArray("canon-root", nil, "Canon root directory (repeatable; default: canon)")
	initCmd.Flags().String("ticketing", "github", "Ticketing provider (github, jira, linear, azure-devops, servicenow, none)")
	initCmd.Flags().Bool("force", false, "Overwrite an existing .okf/config.yaml")
	initCmd.Flags().Bool("quiet", false, "Suppress success output")
	initCmd.Flags().StringArray("agent", nil, "Agent tool to configure locally (repeatable; use --list-agents)")
	initCmd.Flags().String("kiro-agent", "", "Kiro CLI agent config to update when multiple agents exist")
	initCmd.Flags().Bool("list-agents", false, "List supported agent identifiers without modifying the repository")
}

func runInit(cmd *cobra.Command, args []string) error {
	storeRoot := "."
	if len(args) == 1 {
		storeRoot = args[0]
	}
	listAgents, _ := cmd.Flags().GetBool("list-agents")
	if listAgents {
		printSupportedAgents()
		return nil
	}
	force, _ := cmd.Flags().GetBool("force")
	quiet, _ := cmd.Flags().GetBool("quiet")
	rawAgents, _ := cmd.Flags().GetStringArray("agent")
	selected, err := agents.ParseIDs(rawAgents)
	if err != nil {
		printStatus("Error: " + err.Error())
		return err
	}

	cfg, err := resolveInitConfig(cmd)
	if err != nil {
		printStatus("Error: " + err.Error())
		return err
	}

	// Safety: never silently clobber an existing store.
	cfgPath := config.Path(storeRoot)
	if _, statErr := os.Stat(cfgPath); statErr == nil && !force {
		err := fmt.Errorf("store already exists at %s (use --force to overwrite)", cfgPath)
		printStatus("Error: " + err.Error())
		return err
	}

	plan, err := agents.BuildPlan(storeRoot, selected)
	if err != nil {
		printStatus("Error: " + err.Error())
		return err
	}

	kiroAgent, _ := cmd.Flags().GetString("kiro-agent")
	hookCtx := hooks.Context{StoreRoot: storeRoot, Config: cfg, KiroAgent: kiroAgent}
	installers := selectInitInstallers(selected, hookCtx)
	// Parse every existing structured hook file before the first write. This
	// preserves init's no-write guarantee for validation failures.
	for _, installer := range installers {
		status, statusErr := installer.Status(hookCtx)
		if statusErr != nil {
			err = fmt.Errorf("preflight %s hooks: %w", installer.Target(), statusErr)
			printStatus("Error: " + err.Error())
			return err
		}
		if status.Action == hooks.ActionAmbiguous {
			err = fmt.Errorf("preflight %s hooks: %s", installer.Target(), status.Detail)
			printStatus("Error: " + err.Error())
			return err
		}
	}

	if err := writeStore(storeRoot, cfg); err != nil {
		printStatus("Error: " + err.Error())
		return err
	}
	completed := []string{cfgPath}
	applyResult, err := agents.ApplyPlan(plan)
	if err != nil {
		printStatus("Error: " + err.Error())
		reportInitProgress(append(completed, applyResult.Completed...), append(applyResult.Pending, initHookLabels(installers)...))
		return err
	}
	completed = append(completed, applyResult.Completed...)

	hookResults, hookCompleted, hookPending, err := applyInitHooks(hookCtx, installers)
	if err != nil {
		printStatus("Error: " + err.Error())
		reportInitProgress(append(completed, hookCompleted...), hookPending)
		return err
	}

	if !quiet {
		printInitSummary(storeRoot, cfgPath, cfg, plan, hookResults)
	}
	return nil
}

func initHookLabels(installers []hooks.Installer) []string {
	labels := make([]string, 0, len(installers))
	for _, installer := range installers {
		labels = append(labels, "hook:"+string(installer.Target()))
	}
	return labels
}

func applyInitHooks(ctx hooks.Context, installers []hooks.Installer) ([]hooks.Result, []string, []string, error) {
	results := make([]hooks.Result, 0, len(installers))
	var completed []string
	for i, installer := range installers {
		res, err := installer.Install(ctx)
		if err != nil {
			return results, completed, initHookLabels(installers[i:]), fmt.Errorf("install %s hooks: %w", installer.Target(), err)
		}
		if res.Action == hooks.ActionAmbiguous {
			return results, completed, initHookLabels(installers[i:]), fmt.Errorf("install %s hooks: %s", installer.Target(), res.Detail)
		}
		results = append(results, res)
		if len(res.Paths) == 0 {
			completed = append(completed, "hook:"+string(installer.Target()))
		} else {
			completed = append(completed, res.Paths...)
		}
	}
	return results, completed, nil, nil
}

func reportInitProgress(completed, pending []string) {
	if len(completed) > 0 {
		printStatus("Completed initialization changes: " + strings.Join(completed, ", "))
	}
	if len(pending) > 0 {
		printStatus("Pending initialization changes: " + strings.Join(pending, ", "))
	}
}

func printSupportedAgents() {
	fmt.Println("Supported agents:")
	for _, d := range agents.Definitions() {
		fmt.Printf("  %-10s %s\n", d.ID, d.Name)
	}
}

func selectInitInstallers(selected []agents.ID, ctx hooks.Context) []hooks.Installer {
	want := map[hooks.Target]bool{}
	for _, id := range selected {
		switch id {
		case agents.Claude:
			want[hooks.TargetClaude] = true
		case agents.Codex:
			want[hooks.TargetCodex] = true
		case agents.Kiro:
			want[hooks.TargetKiroIDE] = true
			want[hooks.TargetKiroCLI] = true
		}
	}
	var out []hooks.Installer
	for _, installer := range hooks.Installers() {
		if installer.Target() == hooks.TargetGit {
			if installer.Detect(ctx) {
				out = append(out, installer)
			}
			continue
		}
		if want[installer.Target()] {
			out = append(out, installer)
		}
	}
	return out
}

// resolveInitConfig builds the store Config from Default() plus flags, and
// validates every input before any filesystem write occurs.
func resolveInitConfig(cmd *cobra.Command) (config.Config, error) {
	cfg := config.Default()

	if cmd.Flags().Changed("repository-key") {
		key, _ := cmd.Flags().GetString("repository-key")
		if key == "" {
			return config.Config{}, fmt.Errorf("repository key must be non-empty")
		}
		cfg.RepositoryKey = key
	}

	if cmd.Flags().Changed("canon-root") {
		roots, _ := cmd.Flags().GetStringArray("canon-root")
		deduped := dedupeStrings(roots)
		for _, r := range deduped {
			if err := validateCanonRoot(r); err != nil {
				return config.Config{}, err
			}
		}
		if len(deduped) > 0 {
			cfg.CanonRoots = deduped
		}
	}

	providers := validate.KnownProviders()
	ticketing, _ := cmd.Flags().GetString("ticketing")
	if !isKnownTicketing(ticketing, providers) {
		return config.Config{}, fmt.Errorf(
			"unknown ticketing provider %q; allowed: %s, none",
			ticketing, strings.Join(providers, ", "))
	}
	cfg.Ticketing.Provider = ticketing

	return cfg, nil
}

func isKnownTicketing(provider string, known []string) bool {
	if provider == "" || provider == "none" {
		return true
	}
	for _, p := range known {
		if p == provider {
			return true
		}
	}
	return false
}

// validateCanonRoot rejects absolute paths and paths that escape the store root.
func validateCanonRoot(root string) error {
	if filepath.IsAbs(root) {
		return fmt.Errorf("canon root %q must be relative to the store, not absolute", root)
	}
	cleaned := filepath.Clean(root)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("canon root %q escapes the store root", root)
	}
	return nil
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// writeStore performs the filesystem effects: store root, .okf/config.yaml, and
// each canon root. Creating directories is idempotent and never deletes content.
func writeStore(storeRoot string, cfg config.Config) error {
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		return err
	}
	cfgPath := config.Path(storeRoot)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, []byte(config.Render(cfg)), 0o644); err != nil {
		return err
	}
	for _, root := range cfg.CanonRoots {
		if err := os.MkdirAll(filepath.Join(storeRoot, root), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func printInitSummary(storeRoot, cfgPath string, cfg config.Config, plan agents.Plan, hookResults []hooks.Result) {
	color.Green("Initialized pyra store at %s", storeRoot)
	fmt.Printf("  Config: %s\n", cfgPath)
	for _, root := range cfg.CanonRoots {
		fmt.Printf("  Canon root: %s\n", filepath.Join(storeRoot, root))
	}
	for _, change := range plan.Changes {
		fmt.Printf("  Agent setup: %s (%s)\n", change.Path, change.Action)
	}
	for _, selected := range plan.Agents {
		for _, definition := range agents.Definitions() {
			if definition.ID == selected {
				fmt.Printf("  MCP tool: %s (%s)\n", definition.ID, definition.Name)
				break
			}
		}
	}
	for _, res := range hookResults {
		fmt.Printf("  Hook: %s (%s)\n", res.Target, res.Action.String())
		for _, path := range res.Paths {
			fmt.Printf("    %s\n", path)
		}
	}
	if len(plan.Agents) == 0 {
		fmt.Println("\nNo MCP clients selected. Re-run with --force --agent <id> to enable one or more agents.")
	}
	for _, id := range plan.Agents {
		switch id {
		case agents.Codex:
			fmt.Println("  Codex: trust this project and review the Pyra hook with /hooks.")
		case agents.Pi:
			fmt.Println("  Pi: trust the project, allow the project-scoped pi-mcp-adapter install, then restart Pi.")
		}
	}
	firstRoot := "canon"
	if len(cfg.CanonRoots) > 0 {
		firstRoot = cfg.CanonRoots[0]
	}
	example := filepath.Join(firstRoot, "adr-001-example.md")
	fmt.Println("\nNext: author your first Canon artifact")
	fmt.Printf("  pyra new decision %s --title \"My first decision\"\n", example)
}
