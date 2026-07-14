package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/canon/artifacts"
	"github.com/chasedputnam/pyra/internal/canon/frontmatter"
	"github.com/chasedputnam/pyra/internal/canon/identity"
	"github.com/chasedputnam/pyra/internal/config"
)

var newCmd = &cobra.Command{
	Use:   "new <type> <path>",
	Short: "Scaffold a typed Canon artifact with a minted ID",
	Long: `Create a new Canon artifact of the given type (requirement, decision,
design, roadmap, prompt) at the given path. A stable opaque ID is minted into
the frontmatter and the type's required and recommended sections are scaffolded.`,
	Args: cobra.ExactArgs(2),
	RunE: runNew,
}

func init() {
	rootCmd.AddCommand(newCmd)
	newCmd.Flags().String("store", ".", "Store root used to load config (repository_key)")
	newCmd.Flags().String("title", "", "Artifact title (defaults to a humanized filename)")
}

func runNew(cmd *cobra.Command, args []string) error {
	typ := strings.ToLower(args[0])
	path := args[1]
	storeRoot, _ := cmd.Flags().GetString("store")
	title, _ := cmd.Flags().GetString("title")

	reg := artifacts.Default()
	spec, ok := reg[typ]
	if !ok {
		color.Red("Error: unknown type %q. Valid types: %s", typ, strings.Join(reg.Types(), ", "))
		return fmt.Errorf("unknown type %q", typ)
	}

	if _, err := os.Stat(path); err == nil {
		color.Red("Error: file already exists: %s", path)
		return fmt.Errorf("file exists: %s", path)
	}

	cfg, err := config.Load(storeRoot)
	if err != nil {
		return err
	}
	id := identity.Mint(cfg.RepositoryKey, identity.NewEntropy())

	if title == "" {
		title = humanizeFilename(path)
	}

	content := scaffold(id, typ, title, spec)
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}

	color.Green("Created %s artifact %s", typ, id)
	fmt.Printf("  %s\n", path)
	return nil
}

func scaffold(id, typ, title string, spec artifacts.ArtifactSpec) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "schema_version: %d\n", frontmatter.CurrentSchemaVersion)
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "type: %s\n", typ)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", title)

	emit := func(s artifacts.Section) {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", titleize(s.Name), sectionPlaceholder(spec, s.Name))
	}
	for _, s := range spec.Required {
		emit(s)
	}
	for _, s := range spec.Recommended {
		emit(s)
	}
	return b.String()
}

// sectionPlaceholder returns a valid starter value for a scaffolded section:
// the first allowed value for a constrained metadata field, a sample requirement
// line, or a TODO marker otherwise.
func sectionPlaceholder(spec artifacts.ArtifactSpec, name string) string {
	if allowed, ok := spec.Metadata[name]; ok && len(allowed) > 0 {
		return allowed[0]
	}
	if name == "requirements" {
		return "[REQ-001] The system SHALL do a specific, testable thing."
	}
	return "TODO"
}

func titleize(normalized string) string {
	words := strings.Fields(normalized)
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func humanizeFilename(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	return titleize(base)
}
