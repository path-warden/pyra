package cli

import (
	"fmt"
	"sort"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/scale"
	"github.com/chasedputnam/pyra/internal/validate"
)

var showRecommendations bool

var inspectCmd = &cobra.Command{
	Use:   "inspect <bundle>",
	Short: "Inspect an OKF bundle and show statistics",
	Long:  `Inspect an OKF bundle and display statistics about its contents.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runInspect,
}

func init() {
	inspectCmd.Flags().BoolVar(&showRecommendations, "recommendations", false, "Show RAG graduation guidance if scale ceiling is exceeded")
	rootCmd.AddCommand(inspectCmd)
}

func runInspect(cmd *cobra.Command, args []string) error {
	bundleDir := args[0]

	stats, err := validate.InspectBundle(bundleDir)
	if err != nil {
		color.Red("Error: %s", err.Error())
		return err
	}

	// Get scale metrics
	metrics, ceiling, scaleErr := scale.Analyze(bundleDir)

	fmt.Println("pyra inspect")
	fmt.Println()

	if stats.Title != "" {
		fmt.Printf("Title: %s\n", stats.Title)
	}
	fmt.Printf("Concepts: %d\n", stats.ConceptCount)
	fmt.Printf("Links: %d\n", stats.LinkCount)
	fmt.Printf("Broken links: %d\n", stats.BrokenLinks)
	fmt.Printf("Orphan concepts: %d\n", len(stats.OrphanConcepts))

	// Scale metrics section
	if scaleErr == nil {
		fmt.Println("\nScale Metrics:")
		fmt.Printf("  Total tokens:     %d\n", metrics.TotalTokens)
		fmt.Printf("  Avg tokens/concept: %d\n", metrics.AvgTokensPerConcept)
		fmt.Printf("  Index tokens:     %d\n", metrics.IndexTokens)
		if metrics.TotalTokens > 0 {
			fmt.Printf("  Index ratio:      %.2f%%\n", metrics.IndexRatio*100)
		}

		// Scale status with color
		fmt.Print("  Scale status:     ")
		switch ceiling.Status {
		case scale.StatusHealthy:
			color.Green("%s\n", ceiling.Message)
		case scale.StatusWarning:
			color.Yellow("%s\n", ceiling.Message)
		case scale.StatusExceeded:
			color.Red("%s\n", ceiling.Message)
		}
	}

	if len(stats.TypeDistribution) > 0 {
		fmt.Println("\nType distribution:")
		types := sortedKeys(stats.TypeDistribution)
		for _, t := range types {
			fmt.Printf("  %s: %d\n", t, stats.TypeDistribution[t])
		}
	}

	if len(stats.TopLinkedConcepts) > 0 {
		fmt.Println("\nTop linked concepts:")
		for _, c := range stats.TopLinkedConcepts {
			title := c.Title
			if title == "" {
				title = c.ID
			}
			fmt.Printf("  %s (%d links)\n", title, c.Count)
		}
	}

	if len(stats.SourceDomains) > 0 {
		fmt.Println("\nSource domains:")
		domains := sortedKeys(stats.SourceDomains)
		for _, d := range domains {
			fmt.Printf("  %s: %d\n", d, stats.SourceDomains[d])
		}
	}

	// Show RAG guidance if requested and ceiling exceeded
	if showRecommendations && scaleErr == nil && ceiling.Status == scale.StatusExceeded {
		fmt.Println("\n" + color.YellowString("RAG Graduation Guidance:"))
		fmt.Println(scale.RAGGuidance())
	}

	return nil
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
