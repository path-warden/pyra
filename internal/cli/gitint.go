package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/gitint"
)

var hotspotsCmd = &cobra.Command{
	Use:   "hotspots [store]",
	Short: "Rank files by git churn (deterministic, offline)",
	Long: `List the repository's hotspots — files in the top quartile of temporally
decayed churn that also clear absolute activity floors — ranked by hotspot score.
Derived from git history alone; read-only, offline, no LLM. Anchored to HEAD's
commit time so identical repository state yields identical output.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHotspots,
}

var ownershipCmd = &cobra.Command{
	Use:   "ownership [path]",
	Short: "Show ownership, bus factor, and contributors for a file or module",
	Long: `Report git ownership for a file (primary owner and their commit share,
recent owner, contributor count, bus factor) or, for a directory or no path, the
top-level module rollups. Read-only, offline, no LLM.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runOwnership,
}

func init() {
	rootCmd.AddCommand(hotspotsCmd, ownershipCmd)
	hotspotsCmd.Flags().Bool("json", false, "Output as JSON")
	hotspotsCmd.Flags().Int("limit", 20, "Maximum hotspots to show")
	hotspotsCmd.Flags().Int("window", 0, "Commits of history to walk (0 = default)")

	ownershipCmd.Flags().Bool("json", false, "Output as JSON")
	ownershipCmd.Flags().Int("window", 0, "Commits of history to walk (0 = default)")
	ownershipCmd.Flags().String("store", ".", "Store/repository root")
}

func runHotspots(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) == 1 {
		root = args[0]
	}
	jsonOut, _ := cmd.Flags().GetBool("json")
	limit, _ := cmd.Flags().GetInt("limit")
	window, _ := cmd.Flags().GetInt("window")

	h, ok := gitint.New(root, window)
	if !ok {
		return gitUnavailable(jsonOut)
	}
	hot := h.Hotspots()
	if limit > 0 && len(hot) > limit {
		hot = hot[:limit]
	}
	if jsonOut {
		printJSON(hot)
		return nil
	}
	fmt.Printf("pyra hotspots (%d)\n", len(hot))
	if len(hot) == 0 {
		fmt.Println("No hotspots (repository activity is below the hotspot floors).")
		return nil
	}
	fmt.Printf("%-48s %6s %8s %5s %-16s %4s\n", "file", "churn%", "commits", "90d", "owner", "bus")
	for _, f := range hot {
		fmt.Printf("%-48s %5.0f%% %8d %5d %-16s %4d\n",
			trunc(f.Path, 48), f.ChurnPercentile*100, f.CommitsTotal, f.Commits90d, trunc(f.PrimaryOwner, 16), f.BusFactor)
	}
	return nil
}

func runOwnership(cmd *cobra.Command, args []string) error {
	root, _ := cmd.Flags().GetString("store")
	jsonOut, _ := cmd.Flags().GetBool("json")
	window, _ := cmd.Flags().GetInt("window")

	h, ok := gitint.New(root, window)
	if !ok {
		return gitUnavailable(jsonOut)
	}

	// No path → the module rollups.
	if len(args) == 0 {
		mods := h.Modules()
		if jsonOut {
			printJSON(mods)
			return nil
		}
		fmt.Printf("%-24s %5s %8s %8s %-16s %4s\n", "module", "files", "hot", "avgchurn", "owner", "bus")
		for _, m := range mods {
			fmt.Printf("%-24s %5d %8d %8.1f %-16s %4d\n",
				trunc(m.Name, 24), m.FileCount, m.HotspotCount, m.AvgChurn, trunc(m.PrimaryOwner, 16), m.MedianBusFactor)
		}
		return nil
	}

	own := h.OwnershipAt(args[0])
	if jsonOut {
		printJSON(own)
		return nil
	}
	if own.IsModule {
		if own.Module == nil {
			fmt.Printf("%s: no history in the window\n", args[0])
			return nil
		}
		m := own.Module
		fmt.Printf("module %s: %d files, %d hotspots (%.0f%%), owner %s, median bus %d\n",
			m.Name, m.FileCount, m.HotspotCount, m.HotspotDensity*100, m.PrimaryOwner, m.MedianBusFactor)
		return nil
	}
	fmt.Printf("%s\n", own.Path)
	fmt.Printf("  owner:        %s (%.0f%%)\n", own.PrimaryOwner, own.PrimaryOwnerPct*100)
	if own.RecentOwner != "" {
		fmt.Printf("  recent owner: %s\n", own.RecentOwner)
	}
	fmt.Printf("  contributors: %d\n", own.ContributorCount)
	fmt.Printf("  bus factor:   %d\n", own.BusFactor)
	return nil
}

func gitUnavailable(jsonOut bool) error {
	if jsonOut {
		printJSON(map[string]any{"available": false, "reason": "not a git repository or no history"})
		return nil
	}
	fmt.Println("git history unavailable (not a git repository)")
	return nil
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
