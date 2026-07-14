package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/canon/artifacts"
	"github.com/chasedputnam/pyra/internal/canon/classify"
	"github.com/chasedputnam/pyra/internal/canon/frontmatter"
	"github.com/chasedputnam/pyra/internal/canon/identity"
	"github.com/chasedputnam/pyra/internal/canon/model"
	"github.com/chasedputnam/pyra/internal/canon/parse"
	"github.com/chasedputnam/pyra/internal/canon/validate"
	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/store"
)

var promoteCmd = &cobra.Command{
	Use:   "promote <concept-id-or-path>",
	Short: "Promote a Reference concept into a typed Canon artifact",
	Long: `Promote takes an ingested Reference concept and scaffolds a typed Canon
artifact from its content: a stable ID is minted, the type's sections are
created, and the concept body is seeded into the artifact. The new artifact is
then validated; any blocking findings are reported so the draft can be corrected
before it is treated as authoritative Canon. Promotion is a deliberate, reviewed
act — content that is not promoted stays in the Reference tier.`,
	Args: cobra.ExactArgs(1),
	RunE: runPromote,
}

func init() {
	rootCmd.AddCommand(promoteCmd)
	promoteCmd.Flags().String("store", ".", "Store root")
	promoteCmd.Flags().String("type", "", "Canon type to promote to (requirement, decision, design, roadmap, prompt)")
	promoteCmd.Flags().String("out", "", "Output path (defaults to <first canon root>/<slug>.md)")
}

func runPromote(cmd *cobra.Command, args []string) error {
	ref := args[0]
	storeRoot, _ := cmd.Flags().GetString("store")
	typ := strings.ToLower(mustFlag(cmd, "type"))
	outPath, _ := cmd.Flags().GetString("out")

	reg := artifacts.Default()
	spec, ok := reg[typ]
	if !ok {
		color.Red("Error: unknown or missing --type. Valid types: %s", strings.Join(reg.Types(), ", "))
		return fmt.Errorf("invalid type %q", typ)
	}

	cfg, err := config.Load(storeRoot)
	if err != nil {
		return err
	}
	s, err := store.Load(storeRoot, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	item := findReference(s, ref)
	if item == nil {
		color.Red("Error: reference concept not found: %s", ref)
		return fmt.Errorf("reference not found: %s", ref)
	}

	if outPath == "" {
		canonRoot := "canon"
		if len(cfg.CanonRoots) > 0 {
			canonRoot = cfg.CanonRoots[0]
		}
		outPath = filepath.Join(storeRoot, canonRoot, slug(item.Title, item.ID)+".md")
	}
	if _, err := os.Stat(outPath); err == nil {
		color.Red("Error: output file already exists: %s", outPath)
		return fmt.Errorf("file exists: %s", outPath)
	}

	id := identity.Mint(cfg.RepositoryKey, identity.NewEntropy())
	title := item.Title
	if title == "" {
		title = humanizeFilename(outPath)
	}
	content := promoteScaffold(id, typ, title, item.Body, spec)

	if dir := filepath.Dir(outPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return err
	}
	color.Green("Promoted %s -> %s artifact %s", item.ID, typ, id)
	fmt.Printf("  %s\n", outPath)

	// Validate the freshly promoted artifact and surface blocking findings so it
	// is not silently treated as valid Canon.
	p := parse.Parse([]byte(content))
	c := classify.Classify(p, reg)
	issues := validate.Validate(p, c, validate.Options{})
	blocking := blockingIssues(issues)
	if len(blocking) > 0 {
		color.Yellow("\nThe promoted draft has %d blocking finding(s) to resolve before it is valid Canon:", len(blocking))
		for _, iss := range blocking {
			color.Yellow("  [%s] %s", iss.Code, iss.Message)
		}
		fmt.Println("\nEdit the artifact to resolve these, then run `pyra gate`.")
	} else {
		color.Green("Promoted draft passes validation.")
	}
	return nil
}

func mustFlag(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

func findReference(s *store.Store, ref string) *store.Item {
	if it := s.ByID(ref); it != nil && it.Tier == store.TierReference {
		return it
	}
	for i := range s.Reference {
		if s.Reference[i].Path == ref || s.Reference[i].ID == ref {
			return &s.Reference[i]
		}
	}
	return nil
}

func blockingIssues(issues []model.Issue) []model.Issue {
	var out []model.Issue
	for _, iss := range issues {
		if iss.Severity == model.SeverityError {
			out = append(out, iss)
		}
	}
	return out
}

// promoteScaffold builds a typed Canon artifact seeded with the source body
// placed into the artifact's primary prose section.
func promoteScaffold(id, typ, title, sourceBody string, spec artifacts.ArtifactSpec) string {
	target := primaryProseSection(spec)

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "schema_version: %d\n", frontmatter.CurrentSchemaVersion)
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "type: %s\n", typ)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", title)

	emit := func(name string) {
		body := sectionPlaceholder(spec, name)
		if name == target {
			body = stripConceptChrome(sourceBody)
		}
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", titleize(name), body)
	}
	for _, sec := range spec.Required {
		emit(sec.Name)
	}
	for _, sec := range spec.Recommended {
		emit(sec.Name)
	}
	return b.String()
}

// primaryProseSection chooses the section to seed the source body into: the
// first required section that is neither status nor requirements, else the first
// recommended section, else "context".
func primaryProseSection(spec artifacts.ArtifactSpec) string {
	for _, s := range spec.Required {
		if s.Name != "status" && s.Name != "requirements" {
			return s.Name
		}
	}
	if len(spec.Recommended) > 0 {
		return spec.Recommended[0].Name
	}
	return "context"
}

// stripConceptChrome removes a Reference concept's leading "# " title and its
// "> [!summary]" callout so the seeded Canon body does not introduce a second
// title or stray callout.
func stripConceptChrome(body string) string {
	lines := strings.Split(body, "\n")
	var out []string
	droppedTitle := false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if !droppedTitle && strings.HasPrefix(trimmed, "# ") {
			droppedTitle = true
			continue
		}
		if strings.HasPrefix(trimmed, ">") {
			continue // summary callout / blockquote chrome
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func slug(title, fallback string) string {
	t := strings.ToLower(strings.TrimSpace(title))
	if t == "" {
		return strings.ToLower(fallback)
	}
	var b strings.Builder
	prevDash := false
	for _, r := range t {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
