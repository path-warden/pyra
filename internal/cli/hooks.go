package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/hooks"
)

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Install event hooks that run the gate automatically",
	Long: `Manage pyra event hooks across the supported toolchains: git (pre-commit
and post-merge), Claude Code and Codex (PostToolUse), the Kiro IDE (.kiro/hooks),
and the Kiro CLI (.kiro/agents). Hooks run the deterministic gate when artifacts
change, so malformed authority is caught without a manual command.`,
}

var hooksInstallCmd = &cobra.Command{
	Use:           "install",
	Short:         "Install pyra hooks (git always; agent targets auto-detected)",
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runHooksInstall,
}

var hooksUninstallCmd = &cobra.Command{
	Use:           "uninstall",
	Short:         "Remove pyra-managed hooks (leaves other content intact)",
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runHooksUninstall,
}

var hooksStatusCmd = &cobra.Command{
	Use:           "status",
	Short:         "Show which pyra hooks are installed per target",
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runHooksStatus,
}

func init() {
	rootCmd.AddCommand(hooksCmd)
	hooksCmd.AddCommand(hooksInstallCmd, hooksUninstallCmd, hooksStatusCmd)

	for _, c := range []*cobra.Command{hooksInstallCmd, hooksUninstallCmd} {
		c.Flags().String("store", ".", "Store root")
		c.Flags().Bool("git", false, "Target git hooks")
		c.Flags().Bool("claude", false, "Target Claude Code")
		c.Flags().Bool("codex", false, "Target Codex")
		c.Flags().Bool("kiro-ide", false, "Target Kiro IDE hooks")
		c.Flags().Bool("kiro-cli", false, "Target Kiro CLI hooks")
		c.Flags().Bool("kiro", false, "Target both Kiro surfaces")
		c.Flags().Bool("all", false, "Target every surface")
		c.Flags().String("kiro-agent", "", "Kiro CLI agent config to edit (.kiro/agents/<name>.json)")
	}
	hooksStatusCmd.Flags().String("store", ".", "Store root")
}

// hooksContext validates the store and builds the installer context, or returns
// an error if the path is not a pyra store (Requirement 5.2).
func hooksContext(cmd *cobra.Command) (hooks.Context, error) {
	storeRoot, _ := cmd.Flags().GetString("store")
	if _, err := os.Stat(config.Path(storeRoot)); err != nil {
		return hooks.Context{}, fmt.Errorf("not a pyra store: %s has no .okf/config.yaml (run `pyra init` first)", storeRoot)
	}
	cfg, err := config.Load(storeRoot)
	if err != nil {
		return hooks.Context{}, err
	}
	kiroAgent, _ := cmd.Flags().GetString("kiro-agent")
	return hooks.Context{StoreRoot: storeRoot, Config: cfg, KiroAgent: kiroAgent}, nil
}

// selectInstallers resolves which installers to act on from the target flags. With
// no flags, the default set is git (always) plus every detected agent toolchain.
func selectInstallers(cmd *cobra.Command, ctx hooks.Context) []hooks.Installer {
	all, _ := cmd.Flags().GetBool("all")
	git, _ := cmd.Flags().GetBool("git")
	claude, _ := cmd.Flags().GetBool("claude")
	codex, _ := cmd.Flags().GetBool("codex")
	kiroIDE, _ := cmd.Flags().GetBool("kiro-ide")
	kiroCLI, _ := cmd.Flags().GetBool("kiro-cli")
	kiro, _ := cmd.Flags().GetBool("kiro")

	installers := hooks.Installers()
	explicit := all || git || claude || codex || kiroIDE || kiroCLI || kiro
	if !explicit {
		var out []hooks.Installer
		for _, ins := range installers {
			if ins.Target() == hooks.TargetGit || ins.Detect(ctx) {
				out = append(out, ins)
			}
		}
		return out
	}

	want := map[hooks.Target]bool{}
	if all {
		want[hooks.TargetGit] = true
		want[hooks.TargetClaude] = true
		want[hooks.TargetCodex] = true
		want[hooks.TargetKiroIDE] = true
		want[hooks.TargetKiroCLI] = true
	}
	if git {
		want[hooks.TargetGit] = true
	}
	if claude {
		want[hooks.TargetClaude] = true
	}
	if codex {
		want[hooks.TargetCodex] = true
	}
	if kiroIDE || kiro {
		want[hooks.TargetKiroIDE] = true
	}
	if kiroCLI || kiro {
		want[hooks.TargetKiroCLI] = true
	}
	var out []hooks.Installer
	for _, ins := range installers {
		if want[ins.Target()] {
			out = append(out, ins)
		}
	}
	return out
}

func runHooksInstall(cmd *cobra.Command, _ []string) error {
	ctx, err := hooksContext(cmd)
	if err != nil {
		printStatus("Error: " + err.Error())
		return err
	}
	for _, ins := range selectInstallers(cmd, ctx) {
		res, ierr := ins.Install(ctx)
		if ierr != nil {
			printStatus("Error: " + ierr.Error())
			return ierr
		}
		printHookResult(res)
	}
	return nil
}

func runHooksUninstall(cmd *cobra.Command, _ []string) error {
	ctx, err := hooksContext(cmd)
	if err != nil {
		printStatus("Error: " + err.Error())
		return err
	}
	for _, ins := range selectInstallers(cmd, ctx) {
		res, ierr := ins.Uninstall(ctx)
		if ierr != nil {
			printStatus("Error: " + ierr.Error())
			return ierr
		}
		printHookResult(res)
	}
	return nil
}

func runHooksStatus(cmd *cobra.Command, _ []string) error {
	ctx, err := hooksContext(cmd)
	if err != nil {
		printStatus("Error: " + err.Error())
		return err
	}
	for _, ins := range hooks.Installers() {
		res, serr := ins.Status(ctx)
		if serr != nil {
			printStatus("Error: " + serr.Error())
			return serr
		}
		printHookResult(res)
	}
	return nil
}

func printHookResult(res hooks.Result) {
	line := fmt.Sprintf("%-9s %s", string(res.Target), res.Action.String())
	switch res.Action {
	case hooks.ActionAmbiguous:
		color.Yellow("%s — %s", line, res.Detail)
	case hooks.ActionPresent, hooks.ActionCreated, hooks.ActionUpdated, hooks.ActionRemoved:
		color.Green(line)
	default:
		fmt.Println(line)
	}
	for _, p := range res.Paths {
		fmt.Printf("  %s\n", p)
	}
	if res.Detail != "" && res.Action != hooks.ActionAmbiguous {
		fmt.Printf("  %s\n", res.Detail)
	}
}
