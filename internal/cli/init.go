package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/canon/validate"
	"github.com/chasedputnam/pyra/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Scaffold a new store (.okf/config.yaml + canon roots)",
	Long: `Initialize a pyra store: write a self-documenting .okf/config.yaml and
create the canon root directories so you can start authoring Canon immediately.

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
}

func runInit(cmd *cobra.Command, args []string) error {
	storeRoot := "."
	if len(args) == 1 {
		storeRoot = args[0]
	}
	force, _ := cmd.Flags().GetBool("force")
	quiet, _ := cmd.Flags().GetBool("quiet")

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

	if err := writeStore(storeRoot, cfg); err != nil {
		printStatus("Error: " + err.Error())
		return err
	}

	if !quiet {
		printInitSummary(storeRoot, cfgPath, cfg)
	}
	return nil
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

func printInitSummary(storeRoot, cfgPath string, cfg config.Config) {
	color.Green("Initialized pyra store at %s", storeRoot)
	fmt.Printf("  Config: %s\n", cfgPath)
	for _, root := range cfg.CanonRoots {
		fmt.Printf("  Canon root: %s\n", filepath.Join(storeRoot, root))
	}
	firstRoot := "canon"
	if len(cfg.CanonRoots) > 0 {
		firstRoot = cfg.CanonRoots[0]
	}
	example := filepath.Join(firstRoot, "adr-001-example.md")
	fmt.Println("\nNext: author your first Canon artifact")
	fmt.Printf("  pyra new decision %s --title \"My first decision\"\n", example)
}
