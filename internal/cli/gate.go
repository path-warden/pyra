package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/memphis/internal/canon/gate"
	"github.com/chasedputnam/memphis/internal/canon/model"
	"github.com/chasedputnam/memphis/internal/changegate"
	"github.com/chasedputnam/memphis/internal/codeintel"
	"github.com/chasedputnam/memphis/internal/config"
	"github.com/chasedputnam/memphis/internal/sarif"
	"github.com/chasedputnam/memphis/internal/store"
)

var gateCmd = &cobra.Command{
	Use:   "gate [store]",
	Short: "Run the Canon authority gate (validate + relationships + policy)",
	Long: `Run the unified Canon gate over a store: validate every artifact, check
relationship integrity, and classify findings as blocking or advisory per the
store's enforcement policy. Exits non-zero if any blocking finding exists.

Change-aware mode (--diff / --changed / --since) additionally reports which
Accepted Canon artifacts govern each changed file, so a change that touches
governed code is surfaced (and, per enforcement policy, can block). These
findings are advisory by default; set them blocking via the enforcement policy
rule codes "canon-governed-change" and "governed-symbol-unresolved".`,
	Args: cobra.MaximumNArgs(1),
	RunE: runGate,
}

func init() {
	rootCmd.AddCommand(gateCmd)
	gateCmd.Flags().Bool("json", false, "Output result as JSON")
	gateCmd.Flags().Bool("sarif", false, "Output result as SARIF 2.1.0")
	gateCmd.Flags().Bool("diff", false, "Change-aware: evaluate the git staged diff against Canon")
	gateCmd.Flags().String("changed", "", "Change-aware: evaluate this comma-separated file list (bypasses git)")
	gateCmd.Flags().String("since", "", "Change-aware: evaluate files changed since this git ref")
}

// changeSource builds a changegate.Source from the flags, and reports whether
// change-aware mode is enabled at all. An explicit list wins over --since, which
// wins over --diff (staged).
func changeSource(cmd *cobra.Command) (changegate.Source, bool) {
	diff, _ := cmd.Flags().GetBool("diff")
	changed, _ := cmd.Flags().GetString("changed")
	since, _ := cmd.Flags().GetString("since")
	switch {
	case cmd.Flags().Changed("changed"):
		return changegate.Source{Kind: changegate.SourceExplicit, Files: splitList(changed)}, true
	case since != "":
		return changegate.Source{Kind: changegate.SourceSince, Ref: since}, true
	case diff:
		return changegate.Source{Kind: changegate.SourceStaged}, true
	default:
		return changegate.Source{}, false
	}
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func runGate(cmd *cobra.Command, args []string) error {
	storeRoot := "."
	if len(args) == 1 {
		storeRoot = args[0]
	}
	jsonOut, _ := cmd.Flags().GetBool("json")
	sarifOut, _ := cmd.Flags().GetBool("sarif")

	cfg, err := config.Load(storeRoot)
	if err != nil {
		return err
	}
	src, on := changeSource(cmd)
	res, changedCount, err := computeGate(storeRoot, cfg, src, on)
	if err != nil {
		return err
	}

	switch {
	case sarifOut:
		doc := sarif.FromIssues("memphis", version, res.Issues)
		data, _ := json.MarshalIndent(doc, "", "  ")
		fmt.Println(string(data))
	case jsonOut:
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
	default:
		printGateText(res, changedCount)
	}

	if !res.Passed() {
		os.Exit(1)
	}
	return nil
}

// computeGate runs the corpus gate and, when change-aware mode is on, folds the
// governance findings into a single merged result. changedCount is the number of
// changed files (-1 when the mode is off). This is the testable core of runGate,
// separated from rendering and os.Exit.
func computeGate(storeRoot string, cfg config.Config, src changegate.Source, on bool) (gate.Result, int, error) {
	res, err := gate.Run(storeRoot, cfg)
	if err != nil {
		return gate.Result{}, -1, err
	}
	if !on {
		return res, -1, nil
	}
	files, err := changegate.ChangedFiles(storeRoot, src)
	if err != nil {
		return gate.Result{}, -1, err
	}
	st, err := store.Load(storeRoot, cfg)
	if err != nil {
		return gate.Result{}, -1, err
	}
	defer st.Close()
	ops := codeintel.NewOps(nil, storeRoot)
	raw := changegate.Evaluate(st, ops, files)
	return res.Merge(gate.ApplyPolicy(cfg, raw)), len(files), nil
}

func printGateText(res gate.Result, changedCount int) {
	fmt.Println("memphis gate")
	fmt.Printf("Artifacts: %d\n", res.ArtifactCount)
	if changedCount >= 0 {
		fmt.Printf("Changed files: %d\n", changedCount)
	}
	if res.Blocking > 0 {
		color.Red("Blocking: %d", res.Blocking)
	} else {
		fmt.Println("Blocking: 0")
	}
	if res.Advisory > 0 {
		color.Yellow("Advisory: %d", res.Advisory)
	} else {
		fmt.Println("Advisory: 0")
	}
	for _, iss := range res.Issues {
		loc := iss.Path
		if iss.Line > 0 {
			loc = fmt.Sprintf("%s:%d", iss.Path, iss.Line)
		}
		if iss.Severity == model.SeverityError {
			color.Red("  [%s] %s: %s", iss.Code, loc, iss.Message)
		} else {
			color.Yellow("  [%s] %s: %s", iss.Code, loc, iss.Message)
		}
	}
	if res.Passed() {
		color.Green("\nGate passed.")
	} else {
		color.Red("\nGate failed.")
	}
}
