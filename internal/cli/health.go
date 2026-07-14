package cli

import (
	"fmt"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/codegraph"
	"github.com/chasedputnam/pyra/internal/codehealth"
	"github.com/chasedputnam/pyra/internal/codeintel"
	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/gitint"
	"github.com/chasedputnam/pyra/internal/store"
)

var healthCmd = &cobra.Command{
	Use:   "health [store]",
	Short: "Score files for code health (deterministic, offline)",
	Long: `Score every source file across three signals — defect risk,
maintainability, and performance — from deterministic biomarkers over code
structure, git history, the dependency graph, and Canon governance, then rank the
lowest-scoring files and roll up repo KPIs. The scoring kernel is a faithful port
of repowise (AGPL-3.0); the performance dimension's detectors are deferred (clean
files score 10.0 on it). No LLM, no network.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
	healthCmd.Flags().Bool("json", false, "Output as JSON")
	healthCmd.Flags().String("file", "", "Show findings for one file")
	healthCmd.Flags().String("coverage", "", "Coverage report (LCOV or Cobertura) to ingest")
	healthCmd.Flags().Int("limit", 15, "Max lowest-scoring files to list")
}

func runHealth(cmd *cobra.Command, args []string) error {
	storeRoot := "."
	if len(args) == 1 {
		storeRoot = args[0]
	}
	jsonOut, _ := cmd.Flags().GetBool("json")
	fileArg, _ := cmd.Flags().GetString("file")
	covPath, _ := cmd.Flags().GetString("coverage")
	limit, _ := cmd.Flags().GetInt("limit")

	cfg, err := config.Load(storeRoot)
	if err != nil {
		return err
	}
	ops := codeintel.NewOps(nil, storeRoot)
	var roots []string
	for _, r := range cfg.CodeRoots {
		roots = append(roots, filepath.Join(storeRoot, r))
	}

	in := codehealth.Inputs{Ops: ops, Roots: roots, Root: storeRoot}
	if h, ok := gitint.New(storeRoot, gitint.DefaultWindow); ok {
		in.History = h
	}
	if g, err := codegraph.Build(ops, roots, codegraph.Options{}); err == nil {
		in.Graph = g
	}
	if s, err := store.Load(storeRoot, cfg); err == nil {
		in.Store = s
		defer s.Close()
	}
	if covPath != "" {
		if cov, err := codehealth.ParseCoverage(covPath); err == nil {
			in.Coverage = cov
		}
	}

	rep, err := codehealth.Analyze(in)
	if err != nil {
		return err
	}

	if fileArg != "" {
		return renderHealthFile(rep, fileArg, jsonOut)
	}
	if jsonOut {
		printJSON(rep)
		return nil
	}
	printHealthText(rep, limit)
	return nil
}

func printHealthText(rep codehealth.Report, limit int) {
	fmt.Println("pyra health")
	fmt.Printf("Files: %d · average health: %.2f · hotspot health: %.2f\n",
		rep.FileCount, rep.AverageHealth, rep.HotspotHealth)
	if rep.Worst != nil {
		fmt.Printf("Worst: %s (%.1f)\n", rep.Worst.Path, rep.Worst.Defect)
	}
	for _, d := range rep.Contradictions {
		color.Yellow("  [contradictory_decision] %s", d)
	}
	fmt.Println("Lowest-scoring files:")
	fmt.Printf("  %-44s %6s %6s %6s  %s\n", "file", "defect", "maint", "perf", "top marker")
	shown := 0
	for _, f := range rep.Files {
		if f.Defect >= 10.0 {
			continue
		}
		if limit > 0 && shown >= limit {
			break
		}
		shown++
		line := fmt.Sprintf("  %-44s %6.1f %6.1f %6.1f  %s", trunc(f.Path, 44), f.Defect, f.Maintainability, f.Performance, f.TopMarker)
		if f.Defect < 5.0 {
			color.Red(line)
		} else {
			color.Yellow(line)
		}
	}
	if shown == 0 {
		color.Green("  all files healthy")
	}
}

func renderHealthFile(rep codehealth.Report, path string, jsonOut bool) error {
	for _, f := range rep.Files {
		if f.Path != path {
			continue
		}
		if jsonOut {
			printJSON(f)
			return nil
		}
		fmt.Printf("%s\n  defect %.1f · maintainability %.1f · performance %.1f\n",
			f.Path, f.Defect, f.Maintainability, f.Performance)
		if f.Suggestion != "" {
			fmt.Printf("  suggestion: %s (%s)\n", f.Suggestion, f.TopMarker)
		}
		fmt.Println("  findings:")
		for _, fi := range f.Findings {
			fmt.Printf("    [%s] %s (impact %.2f) %s\n", fi.Severity, fi.Biomarker, fi.Impact, fi.Details)
		}
		return nil
	}
	fmt.Printf("%s: not found (or has no definitions)\n", path)
	return nil
}
