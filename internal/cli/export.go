package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/scale"
	"github.com/chasedputnam/pyra/internal/store"
)

var exportCmd = &cobra.Command{
	Use:   "export [store]",
	Short: "Export Reference knowledge for scale-out (documents/graph)",
	Long: `Export the Reference tier for graduation to an external RAG or graph
backend when it outgrows the in-repo filing cabinet. Canon stays in the repo as
the authoritative source of truth and is never exported as documents; only the
relationship graph structure (ids and typed edges) is included in --graph.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runExport,
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().Bool("documents", false, "Export Reference concepts as JSONL (for RAG)")
	exportCmd.Flags().Bool("graph", false, "Export the relationship graph as JSON")
	exportCmd.Flags().String("out", "", "Write to a file instead of stdout")
}

func runExport(cmd *cobra.Command, args []string) error {
	storeRoot := "."
	if len(args) == 1 {
		storeRoot = args[0]
	}
	documents, _ := cmd.Flags().GetBool("documents")
	graph, _ := cmd.Flags().GetBool("graph")
	outPath, _ := cmd.Flags().GetString("out")

	if !documents && !graph {
		return fmt.Errorf("specify --documents and/or --graph")
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

	var out strings.Builder
	if documents {
		out.WriteString(exportDocuments(s))
	}
	if graph {
		data, err := exportGraph(s)
		if err != nil {
			return err
		}
		out.Write(data)
		out.WriteString("\n")
	}

	if outPath != "" {
		if err := os.WriteFile(outPath, []byte(out.String()), 0o644); err != nil {
			return err
		}
		color.Green("Exported to %s", outPath)
	} else {
		fmt.Print(out.String())
	}

	// Scale-out guidance: if Reference has outgrown the filing-cabinet ceiling,
	// surface RAG graduation advice (Requirement 13.3).
	if _, ceiling, aerr := scale.Analyze(storeRoot); aerr == nil && ceiling.Status != scale.StatusHealthy {
		fmt.Fprintln(os.Stderr, "\n"+color.YellowString("Scale notice: ")+ceiling.Message)
		fmt.Fprintln(os.Stderr, scale.RAGGuidance())
	}
	return nil
}

// exportDocuments emits Reference concepts as JSONL. Canon is intentionally
// excluded: it remains the in-repo source of truth (Requirement 13.2).
func exportDocuments(s *store.Store) string {
	var b strings.Builder
	for _, it := range s.Reference {
		rec := map[string]any{
			"id": it.ID, "title": it.Title, "type": it.Type,
			"path": it.Path, "tags": it.Tags, "body": it.Body,
			"tier": "reference",
		}
		data, _ := json.Marshal(rec)
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String()
}

// exportGraph emits the relationship graph structure (nodes + typed edges). This
// is a derived projection; the canonical content is not included.
func exportGraph(s *store.Store) ([]byte, error) {
	type node struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Type  string `json:"type"`
		Tier  string `json:"tier"`
	}
	var nodes []node
	for _, it := range s.Canon {
		nodes = append(nodes, node{it.ID, it.Title, it.Type, "canon"})
	}
	for _, it := range s.Reference {
		nodes = append(nodes, node{it.ID, it.Title, it.Type, "reference"})
	}
	var edges []map[string]string
	if s.Graph != nil {
		for _, es := range s.Graph.Edges {
			for _, e := range es {
				edges = append(edges, map[string]string{"from": e.From, "to": e.To, "kind": e.Kind})
			}
		}
	}
	return json.MarshalIndent(map[string]any{"nodes": nodes, "edges": edges}, "", "  ")
}
