package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/codeintel"
	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/store"
)

// newOps builds a code-intelligence Ops rooted at the current directory, so
// symbol-ids are repo-relative like grove.
func newOps() *codeintel.Ops {
	return codeintel.NewOps(nil, ".")
}

func printJSON(v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}

var outlineCmd = &cobra.Command{
	Use:   "outline <file>",
	Short: "List the definitions in one source file as a compact skeleton",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		kind, _ := cmd.Flags().GetString("kind")
		detail, _ := cmd.Flags().GetInt("detail")
		jsonOut, _ := cmd.Flags().GetBool("json")
		rows, err := newOps().Outline(args[0], kind, detail)
		if err != nil {
			return err
		}
		if jsonOut {
			printJSON(rows)
			return nil
		}
		for _, r := range rows {
			parent := ""
			if p, ok := r["parent"]; ok {
				parent = fmt.Sprintf(" (%v)", p)
			}
			fmt.Printf("%-10v %v%s  :%v\n", r["kind"], r["name"], parent, r["line"])
		}
		return nil
	},
}

var symbolsCmd = &cobra.Command{
	Use:   "symbols <dir>",
	Short: "Find symbols across a directory (gitignore-aware)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		kind, _ := cmd.Flags().GetString("kind")
		name, _ := cmd.Flags().GetString("name")
		nameContains, _ := cmd.Flags().GetBool("name-contains")
		refs, _ := cmd.Flags().GetBool("refs")
		jsonOut, _ := cmd.Flags().GetBool("json")
		syms, err := newOps().Symbols(args[0], kind, name, refs, nameContains)
		if err != nil {
			return err
		}
		if jsonOut {
			printJSON(syms)
			return nil
		}
		for _, s := range syms {
			fmt.Printf("%-10s %s  %s\n", s.Kind, s.Name, s.ID)
		}
		return nil
	},
}

var sourceCmd = &cobra.Command{
	Use:   "source <symbol-id>",
	Short: "Return the exact source of one symbol (by id, or --file + --name)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		name, _ := cmd.Flags().GetString("name")
		jsonOut, _ := cmd.Flags().GetBool("json")
		var idOrFile string
		if len(args) == 1 {
			idOrFile = args[0]
		} else if file != "" {
			idOrFile = file
		} else {
			return fmt.Errorf("provide a symbol-id argument or --file with --name")
		}
		res, err := newOps().Source(idOrFile, name)
		if err != nil {
			return err
		}
		if jsonOut {
			printJSON(res)
			return nil
		}
		fmt.Println(res.Source)
		return nil
	},
}

var checkCmd = &cobra.Command{
	Use:   "check <file>",
	Short: "Report syntax errors (ERROR/MISSING nodes) in a source file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		defects, err := newOps().Check(args[0])
		if err != nil {
			return err
		}
		if jsonOut {
			printJSON(defects)
		} else {
			for _, d := range defects {
				color.Red("  %s at %d:%d  %s", d.Kind, d.Line, d.Col, d.Text)
			}
			if len(defects) == 0 {
				color.Green("no syntax errors")
			}
		}
		if len(defects) > 0 {
			os.Exit(1)
		}
		return nil
	},
}

var callersCmd = &cobra.Command{
	Use:   "callers <name>",
	Short: "Find references to a symbol by name (structural + textual)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		jsonOut, _ := cmd.Flags().GetBool("json")
		if dir == "" {
			dir = "."
		}
		sites, err := newOps().Callers(dir, args[0])
		if err != nil {
			return err
		}
		if jsonOut {
			printJSON(sites)
			return nil
		}
		for _, s := range sites {
			tag := "T"
			if s.Source == "structural" {
				tag = "S"
			}
			in := ""
			if s.InFunction != nil {
				in = " in " + *s.InFunction
			}
			fmt.Printf("[%s] %s:%d:%d%s  %s\n", tag, s.File, s.Line, s.Col, in, s.Text)
		}
		fmt.Fprintln(os.Stderr, "S=structural, T=textual")
		return nil
	},
}

var mapCmd = &cobra.Command{
	Use:   "map <dir>",
	Short: "Structural map of a directory: definitions and their references",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		kind, _ := cmd.Flags().GetString("kind")
		name, _ := cmd.Flags().GetString("name")
		nameContains, _ := cmd.Flags().GetBool("name-contains")
		jsonOut, _ := cmd.Flags().GetBool("json")
		maps, err := newOps().Map(args[0], kind, name, nameContains)
		if err != nil {
			return err
		}
		if jsonOut {
			printJSON(maps)
			return nil
		}
		for _, fm := range maps {
			fmt.Println(fm.File)
			for _, e := range fm.Entries {
				fmt.Printf("  %-10s %s  ->  %v\n", e.Kind, e.Name, e.References)
			}
		}
		return nil
	},
}

var definitionCmd = &cobra.Command{
	Use:   "definition [name]",
	Short: "Find where a symbol is defined (by name, or --at file:line:col)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		at, _ := cmd.Flags().GetString("at")
		dir, _ := cmd.Flags().GetString("dir")
		jsonOut, _ := cmd.Flags().GetBool("json")
		if dir == "" {
			dir = "."
		}
		var name string
		if len(args) == 1 {
			name = args[0]
		}
		res, err := newOps().Definition(name, at, dir)
		if err != nil {
			return err
		}
		if jsonOut {
			printJSON(res)
			return nil
		}
		for _, d := range res.Definitions {
			fmt.Printf("%-10s %s  %s\n", d.Kind, d.Name, d.ID)
		}
		return nil
	},
}

var cliSymbolIDRe = regexp.MustCompile(`[\w.-]+:[^#\s]+#[^@\s]+@\d+`)

var groundCmd = &cobra.Command{
	Use:   "ground <artifact-id | symbol-id>",
	Short: "Ground Canon in code: resolve an artifact's symbols, or find artifacts referencing a symbol",
	Long: `ground bridges the two tiers. Given a Canon artifact ID, it resolves the
code symbols (symbol-ids) that artifact references to their source. Given a
symbol-id, it finds the Canon artifacts that reference that symbol. Read-only.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		storeRoot, _ := cmd.Flags().GetString("store")
		jsonOut, _ := cmd.Flags().GetBool("json")
		cfg, err := config.Load(storeRoot)
		if err != nil {
			return err
		}
		st, err := store.Load(storeRoot, cfg)
		if err != nil {
			return err
		}
		defer st.Close()
		ops := codeintel.NewOps(nil, storeRoot)

		arg := args[0]
		// A symbol-id -> find referencing artifacts; otherwise treat as artifact id.
		if _, _, _, ok := codeintel.ParseID(arg); ok {
			out := artifactsForSymbol(st, arg)
			return renderGround(out, jsonOut, func() {
				for _, a := range out.Artifacts {
					fmt.Printf("%-14s %-11s %s\n", a.ID, a.Type, a.Title)
				}
			})
		}
		out, err := codeForArtifact(st, ops, arg)
		if err != nil {
			return err
		}
		return renderGround(out, jsonOut, func() {
			for _, r := range out.Resolved {
				fmt.Printf("%s\n%s\n\n", r.ID, r.Source)
			}
			for _, u := range out.Unresolved {
				color.Yellow("unresolved: %s", u)
			}
		})
	},
}

type groundArtifact struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
	Path  string `json:"path"`
}

type groundResolved struct {
	ID     string `json:"id"`
	Source string `json:"source"`
}

type groundResult struct {
	Artifact   *groundArtifact  `json:"artifact,omitempty"`
	Resolved   []groundResolved `json:"resolved,omitempty"`
	Unresolved []string         `json:"unresolved,omitempty"`
	Symbol     string           `json:"symbol,omitempty"`
	Artifacts  []groundArtifact `json:"artifacts,omitempty"`
}

func codeForArtifact(st *store.Store, ops *codeintel.Ops, id string) (groundResult, error) {
	item := st.ByID(id)
	if item == nil || item.Tier != store.TierCanon {
		return groundResult{}, fmt.Errorf("canon artifact not found: %s", id)
	}
	res := groundResult{Artifact: &groundArtifact{ID: item.ID, Title: item.Title, Type: item.Type, Path: item.Path}}
	seen := map[string]bool{}
	for _, sid := range cliSymbolIDRe.FindAllString(item.Body, -1) {
		if seen[sid] {
			continue
		}
		seen[sid] = true
		r, err := ops.Source(sid, "")
		if err != nil {
			res.Unresolved = append(res.Unresolved, sid)
			continue
		}
		res.Resolved = append(res.Resolved, groundResolved{ID: r.ID, Source: r.Source})
	}
	return res, nil
}

func artifactsForSymbol(st *store.Store, sid string) groundResult {
	res := groundResult{Symbol: sid}
	query := sid
	if path, name, _, ok := codeintel.ParseID(sid); ok {
		query = name + " " + path
	}
	for _, h := range st.Discover(query, 20) {
		if h.Item == nil || h.Item.Tier != store.TierCanon {
			continue
		}
		if !strings.Contains(h.Item.Body, sid) && query != sid {
			continue
		}
		res.Artifacts = append(res.Artifacts, groundArtifact{ID: h.Item.ID, Title: h.Item.Title, Type: h.Item.Type, Path: h.Item.Path})
	}
	return res
}

func renderGround(out groundResult, jsonOut bool, text func()) error {
	if jsonOut {
		printJSON(out)
		return nil
	}
	text()
	return nil
}

func init() {
	rootCmd.AddCommand(outlineCmd, symbolsCmd, sourceCmd, checkCmd, callersCmd, mapCmd, definitionCmd, groundCmd)

	groundCmd.Flags().String("store", ".", "Store root")
	groundCmd.Flags().Bool("json", false, "Output as JSON")

	outlineCmd.Flags().String("kind", "", "Only this kind (e.g. class, function, method)")
	outlineCmd.Flags().Int("detail", 1, "0 terse · 1 default · 2 full")
	outlineCmd.Flags().Bool("json", false, "Output as JSON")

	symbolsCmd.Flags().String("kind", "", "Only this kind")
	symbolsCmd.Flags().String("name", "", "Only definitions whose name equals this (case-insensitive)")
	symbolsCmd.Flags().Bool("name-contains", false, "Substring matching for --name")
	symbolsCmd.Flags().Bool("refs", false, "Include references, not just definitions")
	symbolsCmd.Flags().Bool("json", false, "Output as JSON")

	sourceCmd.Flags().String("file", "", "Source file (with --name)")
	sourceCmd.Flags().String("name", "", "Symbol name to find in --file")
	sourceCmd.Flags().Bool("json", false, "Output as JSON")

	checkCmd.Flags().Bool("json", false, "Output as JSON")

	callersCmd.Flags().String("dir", ".", "Directory to search")
	callersCmd.Flags().Bool("json", false, "Output as JSON")

	mapCmd.Flags().String("kind", "", "Only definitions of this kind")
	mapCmd.Flags().String("name", "", "Only definitions whose name equals this")
	mapCmd.Flags().Bool("name-contains", false, "Substring matching for --name")
	mapCmd.Flags().Bool("json", false, "Output as JSON")

	definitionCmd.Flags().String("at", "", "Usage site to resolve: file:line:col (1-based)")
	definitionCmd.Flags().String("dir", ".", "Directory to search")
	definitionCmd.Flags().Bool("json", false, "Output as JSON")
}
