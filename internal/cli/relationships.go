package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/canon"
	"github.com/chasedputnam/pyra/internal/canon/model"
	"github.com/chasedputnam/pyra/internal/canon/relate"
	"github.com/chasedputnam/pyra/internal/config"
)

var relationshipsCmd = &cobra.Command{
	Use:   "relationships [store]",
	Short: "Report and validate the Canon relationship graph",
	Long: `Build the typed relationship graph over a store's Canon artifacts and
print edges and inbound counts. With --validate, also report relationship
integrity issues and exit non-zero on errors.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRelationships,
}

func init() {
	rootCmd.AddCommand(relationshipsCmd)
	relationshipsCmd.Flags().Bool("validate", false, "Report integrity issues and fail on errors")
	relationshipsCmd.Flags().Bool("summary", false, "Show relationship health summary (coverage, orphans, broken)")
	relationshipsCmd.Flags().Bool("json", false, "Output as JSON")
}

func runRelationships(cmd *cobra.Command, args []string) error {
	storeRoot := "."
	if len(args) == 1 {
		storeRoot = args[0]
	}
	doValidate, _ := cmd.Flags().GetBool("validate")
	doSummary, _ := cmd.Flags().GetBool("summary")
	jsonOut, _ := cmd.Flags().GetBool("json")

	cfg, err := config.Load(storeRoot)
	if err != nil {
		return err
	}
	arts, err := canon.LoadCorpus(storeRoot, cfg)
	if err != nil {
		return err
	}
	entries := make([]relate.Entry, 0, len(arts))
	for _, a := range arts {
		entries = append(entries, relate.Entry{
			ID: a.ID, Type: a.Type, Status: a.Status, Retired: a.Retired, Path: a.Path,
			Aliases: a.Aliases, Product: a.Product,
		})
	}
	graph, issues := relate.Build(entries, relate.DefaultSpecs())
	summary := relate.Summarize(entries)

	if jsonOut {
		out := map[string]any{
			"edges":          graph.Edges,
			"inbound_counts": graph.InboundCounts(),
		}
		if doValidate {
			out["issues"] = issues
		}
		if doSummary {
			out["summary"] = summary
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Println("pyra relationships")
		fmt.Printf("Artifacts: %d\n", len(arts))
		edgeCount := 0
		for _, es := range graph.Edges {
			edgeCount += len(es)
		}
		fmt.Printf("Edges: %d\n", edgeCount)
		for from, es := range graph.Edges {
			for _, e := range es {
				fmt.Printf("  %s --%s--> %s\n", from, e.Kind, e.To)
			}
		}
		if doSummary {
			fmt.Printf("\nSummary: %d references, %d valid, %d broken, %d orphaned, coverage %.0f%%\n",
				summary.Total, summary.Valid, summary.Broken, summary.Orphaned, summary.Coverage*100)
		}
		if doValidate {
			printRelationshipIssues(issues)
		}
	}

	if doValidate && hasErrors(issues) {
		os.Exit(1)
	}
	return nil
}

func printRelationshipIssues(issues []model.Issue) {
	errs := 0
	for _, iss := range issues {
		if iss.Severity == model.SeverityError {
			errs++
			color.Red("  [%s] %s: %s", iss.Code, iss.Path, iss.Message)
		} else {
			color.Yellow("  [%s] %s: %s", iss.Code, iss.Path, iss.Message)
		}
	}
	if errs == 0 {
		color.Green("\nRelationship integrity OK.")
	} else {
		color.Red("\nRelationship integrity failed (%d errors).", errs)
	}
}

func hasErrors(issues []model.Issue) bool {
	for _, iss := range issues {
		if iss.Severity == model.SeverityError {
			return true
		}
	}
	return false
}
