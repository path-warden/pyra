package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/codegraph"
	"github.com/chasedputnam/pyra/internal/codeintel"
	"github.com/chasedputnam/pyra/internal/config"
)

var graphCmd = &cobra.Command{
	Use:   "graph [store]",
	Short: "Build and query the code dependency graph (deterministic, offline)",
	Long: `Build a two-tier (file + symbol) dependency graph from code intelligence
over the configured code roots, then query it. Subviews (choose one; --centrality
is the default):

  --centrality     rank symbols by PageRank (the hubs)
  --communities    logical modules via label propagation
  --cycles         dependency cycles (strongly-connected components)
  --reachability   symbols reachable from entry points vs. the unreachable rest

Read-only, offline, no LLM; deterministic (identical repo state → identical
output). Algorithms are standard and self-contained (PageRank, label propagation,
Tarjan SCC); betweenness centrality and Leiden are intentionally not used.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runGraph,
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.Flags().Bool("centrality", false, "Rank symbols by PageRank (default view)")
	graphCmd.Flags().Bool("communities", false, "Show label-propagation communities")
	graphCmd.Flags().Bool("cycles", false, "Show dependency cycles (SCCs)")
	graphCmd.Flags().Bool("reachability", false, "Show reachable vs. unreachable symbols")
	graphCmd.Flags().Int("limit", 20, "Max rows for the centrality view")
	graphCmd.Flags().String("scope", "", "Restrict the graph to a subdirectory")
	graphCmd.Flags().Int("node-cap", 0, "Cap the number of symbol nodes (0 = no cap)")
	graphCmd.Flags().Bool("json", false, "Output as JSON")
}

func runGraph(cmd *cobra.Command, args []string) error {
	storeRoot := "."
	if len(args) == 1 {
		storeRoot = args[0]
	}
	jsonOut, _ := cmd.Flags().GetBool("json")
	limit, _ := cmd.Flags().GetInt("limit")
	scope, _ := cmd.Flags().GetString("scope")
	nodeCap, _ := cmd.Flags().GetInt("node-cap")

	cfg, err := config.Load(storeRoot)
	if err != nil {
		return err
	}
	ops := codeintel.NewOps(nil, storeRoot)
	var roots []string
	for _, r := range cfg.CodeRoots {
		roots = append(roots, filepath.Join(storeRoot, r))
	}
	opts := codegraph.Options{NodeCap: nodeCap}
	if scope != "" {
		opts.Scope = filepath.Join(storeRoot, scope)
	}
	g, err := codegraph.Build(ops, roots, opts)
	if err != nil {
		return err
	}

	comm, _ := cmd.Flags().GetBool("communities")
	cyc, _ := cmd.Flags().GetBool("cycles")
	reach, _ := cmd.Flags().GetBool("reachability")

	switch {
	case comm:
		return renderCommunities(g, jsonOut)
	case cyc:
		return renderCycles(g, jsonOut)
	case reach:
		return renderReachability(g, jsonOut)
	default: // centrality
		return renderCentrality(g, limit, jsonOut)
	}
}

func graphHeader(g *codegraph.Graph) {
	fmt.Printf("pyra graph — %d symbols\n", g.NodeCount())
	if g.Truncated {
		fmt.Println("(graph truncated by --node-cap)")
	}
}

func renderCentrality(g *codegraph.Graph, limit int, jsonOut bool) error {
	top := g.TopCentral(limit)
	if jsonOut {
		printJSON(map[string]any{"total": g.NodeCount(), "truncated": g.Truncated, "centrality": top})
		return nil
	}
	graphHeader(g)
	fmt.Printf("Top %d by PageRank:\n", len(top))
	for _, c := range top {
		fmt.Printf("  %.5f  %s\n", c.Score, c.ID)
	}
	return nil
}

func renderCommunities(g *codegraph.Graph, jsonOut bool) error {
	cs := g.Communities()
	if jsonOut {
		printJSON(map[string]any{"total": g.NodeCount(), "communities": cs})
		return nil
	}
	graphHeader(g)
	fmt.Printf("%d communities:\n", len(cs))
	for _, c := range cs {
		fmt.Printf("  community %d (%d symbols)\n", c.ID, len(c.Members))
		for _, m := range c.Members {
			fmt.Printf("    %s\n", m)
		}
	}
	return nil
}

func renderCycles(g *codegraph.Graph, jsonOut bool) error {
	cycles := g.Cycles()
	if jsonOut {
		printJSON(map[string]any{"cycles": cycles})
		return nil
	}
	graphHeader(g)
	if len(cycles) == 0 {
		fmt.Println("No dependency cycles.")
		return nil
	}
	fmt.Printf("%d cycle(s):\n", len(cycles))
	for i, c := range cycles {
		fmt.Printf("  cycle %d (%d symbols): %v\n", i, len(c), c)
	}
	return nil
}

func renderReachability(g *codegraph.Graph, jsonOut bool) error {
	r := g.Reachability()
	if jsonOut {
		printJSON(r)
		return nil
	}
	graphHeader(g)
	fmt.Printf("Entry points: %d · reachable: %d · unreachable: %d\n",
		len(r.EntryPoints), len(r.Reachable), len(r.Unreachable))
	for _, id := range r.Unreachable {
		fmt.Printf("  unreachable: %s\n", id)
	}
	return nil
}
