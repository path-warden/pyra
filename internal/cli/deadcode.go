package cli

import (
	"fmt"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/memphis/internal/codegraph"
	"github.com/chasedputnam/memphis/internal/codeintel"
	"github.com/chasedputnam/memphis/internal/config"
	"github.com/chasedputnam/memphis/internal/deadcode"
	"github.com/chasedputnam/memphis/internal/store"
)

var deadCodeCmd = &cobra.Command{
	Use:   "dead-code [store]",
	Short: "Report likely-unreachable code by confidence tier (deterministic, offline)",
	Long: `Report unreachable definitions — code with no path from the repository's
entry points (main + exported/public symbols) in the dependency graph — ranked by
cleanup impact. Each candidate carries a confidence tier: high (no textual
references, safe to delete), medium (has textual/possibly-dynamic references), or
low (a test-file helper). A "[governed]" marker means Accepted Canon still cites
that now-unreachable code (drift).

Read-only, offline, no LLM. Note: memphis has no framework route→handler edges, so
a handler reachable only via a route may read as unreachable — the high tier is
conservative to mitigate this.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDeadCode,
}

func init() {
	rootCmd.AddCommand(deadCodeCmd)
	deadCodeCmd.Flags().String("tier", "", "Only this tier: high, medium, or low")
	deadCodeCmd.Flags().Int("limit", 0, "Max candidates to show (0 = all)")
	deadCodeCmd.Flags().Bool("json", false, "Output as JSON")
}

func runDeadCode(cmd *cobra.Command, args []string) error {
	storeRoot := "."
	if len(args) == 1 {
		storeRoot = args[0]
	}
	tier, _ := cmd.Flags().GetString("tier")
	limit, _ := cmd.Flags().GetInt("limit")
	jsonOut, _ := cmd.Flags().GetBool("json")

	cfg, err := config.Load(storeRoot)
	if err != nil {
		return err
	}
	ops := codeintel.NewOps(nil, storeRoot)
	var roots []string
	for _, r := range cfg.CodeRoots {
		roots = append(roots, filepath.Join(storeRoot, r))
	}
	g, err := codegraph.Build(ops, roots, codegraph.Options{})
	if err != nil {
		return err
	}

	var canonBodies []string
	if s, err := store.Load(storeRoot, cfg); err == nil {
		for _, it := range s.Canon {
			canonBodies = append(canonBodies, it.Body)
		}
		defer s.Close()
	}

	rep := deadcode.Analyze(g, ops, storeRoot, canonBodies)
	cands := filterTier(rep.Candidates, tier)
	if limit > 0 && len(cands) > limit {
		cands = cands[:limit]
	}

	if jsonOut {
		printJSON(map[string]any{"candidates": cands, "total_impact": rep.TotalImpact})
		return nil
	}
	fmt.Printf("memphis dead-code — %d candidate(s), %d lines of cleanup impact\n", len(cands), rep.TotalImpact)
	if len(cands) == 0 {
		color.Green("  no unreachable code found")
		return nil
	}
	fmt.Printf("  %6s  %-8s %-40s %s\n", "impact", "tier", "file", "symbol")
	for _, c := range cands {
		marker := ""
		if c.Governed {
			marker = " [governed]"
		}
		line := fmt.Sprintf("  %6d  %-8s %-40s %s%s", c.Impact, c.Tier, trunc(c.File, 40), c.Name, marker)
		if c.Tier == deadcode.TierHigh {
			color.Red(line)
		} else {
			color.Yellow(line)
		}
	}
	return nil
}

func filterTier(cands []deadcode.Candidate, tier string) []deadcode.Candidate {
	if tier == "" {
		return cands
	}
	var out []deadcode.Candidate
	for _, c := range cands {
		if c.Tier == tier {
			out = append(out, c)
		}
	}
	return out
}
