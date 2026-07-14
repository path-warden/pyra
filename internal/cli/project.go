package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/canon/project"
	"github.com/chasedputnam/pyra/internal/config"
)

var projectCmd = &cobra.Command{
	Use:   "project <spec-doc-or-dir>",
	Short: "Project an approved spec document into a typed Canon artifact",
	Long: `Project turns an approved spec document (requirements.md or design.md, from
the local specs/ layout or Kiro's .kiro/specs/ layout) into a typed Canon
artifact: a stable ID is reused or minted, the type's sections are filled from
the source prose, and relationships are inferred from literal ID references.

Projection is ratify-or-correct: a new artifact is created, but an existing one
is only overwritten with --write (or interactive confirmation). Use --dry-run to
preview without touching the filesystem.`,
	Args:          cobra.ExactArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runProject,
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.Flags().String("store", ".", "Store root")
	projectCmd.Flags().String("type", "", "Canon type override (else inferred from the document name)")
	projectCmd.Flags().Bool("dry-run", false, "Preview the projection without writing")
	projectCmd.Flags().Bool("write", false, "Apply changes to an existing artifact")
	projectCmd.Flags().Bool("force", false, "Alias for --write")
	projectCmd.Flags().String("kiro-agent", "", "Reserved (parity with hooks); unused by project")
	projectCmd.Flags().Bool("json", false, "Output results as JSON")
	projectCmd.Flags().Bool("quiet", false, "Suppress success output")
}

func runProject(cmd *cobra.Command, args []string) error {
	path := args[0]
	storeRoot, _ := cmd.Flags().GetString("store")
	typ, _ := cmd.Flags().GetString("type")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	write, _ := cmd.Flags().GetBool("write")
	force, _ := cmd.Flags().GetBool("force")
	kiroAgent, _ := cmd.Flags().GetString("kiro-agent")
	jsonOut, _ := cmd.Flags().GetBool("json")
	quiet, _ := cmd.Flags().GetBool("quiet")

	cfg, err := config.Load(storeRoot)
	if err != nil {
		printStatus("Error: " + err.Error())
		return err
	}

	opts := project.Options{
		Store:     storeRoot,
		Type:      typ,
		DryRun:    dryRun,
		Write:     write || force,
		KiroAgent: kiroAgent,
	}

	info, err := os.Stat(path)
	if err != nil {
		printStatus("Error: " + err.Error())
		return err
	}

	var results []project.Result
	if info.IsDir() {
		results, err = project.ProjectDir(cfg, path, opts)
	} else {
		var r project.Result
		r, err = project.Project(cfg, path, opts)
		results = []project.Result{r}
	}
	if err != nil {
		printStatus("Error: " + err.Error())
		return err
	}

	// Ratify changes to existing artifacts that were not applied. Interactively
	// prompt when attached to a terminal; otherwise require --write.
	needsWrite := false
	for i := range results {
		r := &results[i]
		if r.Created || !r.Changed || opts.Write || opts.DryRun {
			continue
		}
		if isInteractive() && confirmApply(r) {
			applied, aerr := project.Project(cfg, r.SourcePath, withWrite(opts))
			if aerr != nil {
				printStatus("Error: " + aerr.Error())
				return aerr
			}
			results[i] = applied
			continue
		}
		needsWrite = true
	}

	if jsonOut {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
	} else if !quiet {
		for i := range results {
			printProjectResult(&results[i], opts)
		}
	}

	// Exit non-zero if any artifact has blocking issues or an unratified change.
	blocking := false
	for i := range results {
		if len(results[i].BlockingIssues) > 0 {
			blocking = true
		}
	}
	switch {
	case blocking:
		err := fmt.Errorf("projection produced blocking issues; resolve them before treating the artifact as Canon")
		printStatus("Error: " + err.Error())
		return err
	case needsWrite:
		err := fmt.Errorf("existing artifact would change; re-run with --write to apply")
		printStatus("Error: " + err.Error())
		return err
	}
	return nil
}

func withWrite(o project.Options) project.Options {
	o.Write = true
	return o
}

// isInteractive reports whether both stdin and stdout are terminals, so the
// command may prompt. In tests and pipelines this is false (non-interactive).
func isInteractive() bool {
	return isCharDevice(os.Stdin) && isCharDevice(os.Stdout)
}

func isCharDevice(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func confirmApply(r *project.Result) bool {
	if r.Diff != "" {
		fmt.Println(r.Diff)
	}
	fmt.Printf("Apply changes to %s? [y/N] ", r.ArtifactPath)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}

func printProjectResult(r *project.Result, opts project.Options) {
	action := "Updated"
	switch {
	case opts.DryRun && r.Created:
		action = "Would create"
	case opts.DryRun:
		action = "Would update"
	case r.Created:
		action = "Created"
	case !r.Changed:
		action = "Unchanged"
	case !opts.Write:
		action = "Pending (needs --write)"
	}
	color.Green("%s %s artifact %s", action, r.Type, r.ID)
	fmt.Printf("  %s\n", r.ArtifactPath)

	if len(r.IncompleteSections) > 0 {
		color.Yellow("  Incomplete sections (fill before relying on them): %s", strings.Join(r.IncompleteSections, ", "))
	}
	for _, e := range r.InferredEdges {
		fmt.Printf("  + %s: %s\n", titleize(e.Section), e.Target)
	}
	for _, ref := range r.UnresolvedRefs {
		color.Yellow("  ? unresolved reference (not linked): %s", ref)
	}
	for _, iss := range r.BlockingIssues {
		color.Red("  [%s] %s", iss.Code, iss.Message)
	}
	if r.Diff != "" && (opts.DryRun || !opts.Write) {
		fmt.Println(r.Diff)
	}
}
