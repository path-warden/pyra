package cli

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/changerisk"
	"github.com/chasedputnam/pyra/internal/codeintel"
	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/store"
)

var riskCmd = &cobra.Command{
	Use:   "risk [commit | base..head]",
	Short: "Score a change for defect risk (deterministic, offline)",
	Long: `Score a change for defect risk from the shape of its diff (Kamei
just-in-time metrics), ranked against this repository's own recent commits, with
PR directives (missing tests, absent co-change partners, structural dependents,
and governed Canon). No argument scores the staged diff; a single sha scores that
commit; "base..head" scores the range as one change.

The headline is repo-relative (Below typical / Typical / Elevated + percentile);
the raw 0–10 score is a secondary, uncalibrated ordering number.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRisk,
}

func init() {
	rootCmd.AddCommand(riskCmd)
	riskCmd.Flags().String("store", ".", "Store root (for governance directives)")
	riskCmd.Flags().Int("baseline", changerisk.DefaultBaseline, "Recent commits sampled for the repo-relative ranking")
	riskCmd.Flags().String("ext", "", "Restrict counted files to these comma-separated suffixes (e.g. .go,.py)")
	riskCmd.Flags().Bool("json", false, "Output the report as JSON")
}

func runRisk(cmd *cobra.Command, args []string) error {
	storeRoot, _ := cmd.Flags().GetString("store")
	baseline, _ := cmd.Flags().GetInt("baseline")
	extStr, _ := cmd.Flags().GetString("ext")
	jsonOut, _ := cmd.Flags().GetBool("json")

	ch := changerisk.Change{Mode: changerisk.ModeStaged}
	if len(args) == 1 {
		if base, head, ok := strings.Cut(args[0], ".."); ok {
			ch = changerisk.Change{Mode: changerisk.ModeRange, Base: base, Head: head}
		} else {
			ch = changerisk.Change{Mode: changerisk.ModeCommit, SHA: args[0]}
		}
	}

	cfg, err := config.Load(storeRoot)
	if err != nil {
		return err
	}
	// Load the store best-effort: risk works without Canon (governance simply
	// yields nothing).
	var st *store.Store
	if s, err := store.Load(storeRoot, cfg); err == nil {
		st = s
		defer st.Close()
	}
	ops := codeintel.NewOps(nil, storeRoot)

	rep, err := changerisk.Assess(storeRoot, storeRoot, ch, st, ops, changerisk.Options{
		Baseline: baseline,
		Exts:     splitList(extStr),
	})
	if err != nil {
		return err
	}

	if jsonOut {
		printJSON(rep)
		return nil
	}
	printRiskText(rep)
	return nil
}

func printRiskText(rep changerisk.Report) {
	fmt.Printf("pyra risk (%s)\n", rep.Ref)
	switch rep.Priority {
	case changerisk.PriorityElevated:
		color.Red("Review priority: %s", rep.HeadlineText())
	case changerisk.PriorityTypical:
		color.Yellow("Review priority: %s", rep.HeadlineText())
	default:
		fmt.Printf("Review priority: %s\n", rep.HeadlineText())
	}

	fmt.Println("Drivers:")
	for _, d := range rep.TopDriversText() {
		fmt.Printf("  %s\n", d)
	}

	if len(rep.Directives) == 0 {
		fmt.Println("Directives: none")
		return
	}
	fmt.Println("Directives:")
	for _, d := range rep.Directives {
		color.Yellow("  [%s] %s", d.Code, d.Message)
	}
}
