package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/canon/gate"
	"github.com/chasedputnam/pyra/internal/canon/model"
	"github.com/chasedputnam/pyra/internal/changegate"
	"github.com/chasedputnam/pyra/internal/changerisk"
	"github.com/chasedputnam/pyra/internal/codeintel"
	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/sarif"
	"github.com/chasedputnam/pyra/internal/store"
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
rule codes "canon-governed-change" and "governed-symbol-unresolved".

--risk additionally scores the change for defect risk (repo-relative ranking +
directives: missing_tests, missing_cochanges, will_break, governance_risk) and
merges those findings into the same result and exit code. Rule codes:
"change-risk", "risk-missing-tests", "risk-missing-cochanges", "risk-will-break",
"risk-governance". See "pyra risk" for the standalone report.`,
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
	gateCmd.Flags().Bool("risk", false, "Also score the change for defect risk (implies change-aware; staged by default)")
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
	risk, _ := cmd.Flags().GetBool("risk")
	if risk && !on {
		// --risk with no explicit source defaults to the staged diff.
		src, on = changegate.Source{Kind: changegate.SourceStaged}, true
	}
	res, changedCount, err := computeGate(storeRoot, cfg, src, on, risk)
	if err != nil {
		return err
	}

	switch {
	case sarifOut:
		doc := sarif.FromIssues("pyra", version, res.Issues)
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
func computeGate(storeRoot string, cfg config.Config, src changegate.Source, on, risk bool) (gate.Result, int, error) {
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
	res = res.Merge(gate.ApplyPolicy(cfg, changegate.Evaluate(st, ops, files)))

	if risk {
		rep, err := changerisk.Assess(storeRoot, storeRoot, riskChange(src), st, ops, changerisk.Options{})
		if err != nil {
			return gate.Result{}, -1, err
		}
		res = res.Merge(gate.ApplyPolicy(cfg, rep.Issues()))
	}
	return res, len(files), nil
}

// riskChange maps the gate's change source to a changerisk.Change.
func riskChange(src changegate.Source) changerisk.Change {
	switch src.Kind {
	case changegate.SourceSince:
		return changerisk.Change{Mode: changerisk.ModeSince, Ref: src.Ref}
	case changegate.SourceExplicit:
		return changerisk.Change{Mode: changerisk.ModeFiles, Files: src.Files}
	default:
		return changerisk.Change{Mode: changerisk.ModeStaged}
	}
}

func printGateText(res gate.Result, changedCount int) {
	fmt.Println("pyra gate")
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
		// Repo-level findings (e.g. the change-risk headline) carry no path.
		prefix := fmt.Sprintf("  [%s] %s: ", iss.Code, loc)
		if loc == "" {
			prefix = fmt.Sprintf("  [%s] ", iss.Code)
		}
		if iss.Severity == model.SeverityError {
			color.Red("%s%s", prefix, iss.Message)
		} else {
			color.Yellow("%s%s", prefix, iss.Message)
		}
	}
	if res.Passed() {
		color.Green("\nGate passed.")
	} else {
		color.Red("\nGate failed.")
	}
}
