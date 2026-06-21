package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/okfy/okf-mcp/internal/differ"
	"github.com/okfy/okf-mcp/internal/updater"
)

var updateCmd = &cobra.Command{
	Use:   "update <bundle>",
	Short: "Update an existing OKF bundle from its source",
	Long: `Update an OKF bundle by fetching new content from its original source.

The source is read from the bundle's changelog.txt file, or can be overridden
with the --source flag. The command will:
  1. Fetch current content from the source (URL or local path)
  2. Compare against existing bundle files
  3. Show changes and prompt for confirmation (unless --force)
  4. Apply approved changes and update the changelog`,
	Args: cobra.ExactArgs(1),
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().StringP("source", "s", "", "Override source URL or path from changelog")
	updateCmd.Flags().Bool("force", false, "Apply all changes without prompting")
	updateCmd.Flags().Bool("dry-run", false, "Show changes without applying them")
	updateCmd.Flags().Int("max-pages", 100, "Maximum pages to crawl (for URL sources)")
	updateCmd.Flags().Int("max-depth", 4, "Maximum crawl depth (for URL sources)")
	updateCmd.Flags().Int("concurrency", 4, "Fetch concurrency (for URL sources)")
	updateCmd.Flags().StringSlice("include", nil, "Include glob or regex patterns")
	updateCmd.Flags().StringSlice("exclude", nil, "Exclude glob or regex patterns")

	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	bundlePath := args[0]
	source, _ := cmd.Flags().GetString("source")
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	maxPages, _ := cmd.Flags().GetInt("max-pages")
	maxDepth, _ := cmd.Flags().GetInt("max-depth")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	include, _ := cmd.Flags().GetStringSlice("include")
	exclude, _ := cmd.Flags().GetStringSlice("exclude")

	isTTY := isTerminal()

	opts := updater.UpdateOptions{
		BundlePath:  bundlePath,
		Source:      source,
		Force:       force,
		DryRun:      dryRun,
		MaxPages:    maxPages,
		MaxDepth:    maxDepth,
		Concurrency: concurrency,
		Include:     include,
		Exclude:     exclude,
		OnProgress:  makeUpdateProgressHandler(isTTY),
	}

	// Set up interactive prompts if not force mode and TTY
	if !force && !dryRun && isTTY {
		opts.OnPrompt = makeUpdatePromptHandler()
	}

	result, err := updater.Update(context.Background(), opts)
	if err != nil {
		return err
	}

	// Print results
	fmt.Println()
	if dryRun {
		color.Cyan("okf-cli update (dry run)")
		fmt.Printf("Bundle: %s\n", bundlePath)
		fmt.Printf("Would add: %d files\n", result.Added)
		fmt.Printf("Would modify: %d files\n", result.Modified)
		fmt.Printf("Would delete: %d files\n", result.Deleted)
		return nil
	}

	color.Green("okf-cli update")
	fmt.Printf("Bundle: %s\n", bundlePath)
	fmt.Printf("Added: %d files\n", result.Added)
	fmt.Printf("Modified: %d files\n", result.Modified)
	fmt.Printf("Deleted: %d files\n", result.Deleted)
	if result.Skipped > 0 {
		fmt.Printf("Skipped: %d files\n", result.Skipped)
	}

	if result.Added == 0 && result.Modified == 0 && result.Deleted == 0 {
		color.Yellow("No changes detected")
	}

	return nil
}

func makeUpdateProgressHandler(isTTY bool) func(phase string, message string) {
	return func(phase string, message string) {
		clear := ""
		if isTTY {
			clear = "\r\033[K"
		}

		switch phase {
		case "fetching":
			fmt.Fprintf(os.Stderr, "%sokf-cli update: %s\n", clear, message)
		case "diffing":
			fmt.Fprintf(os.Stderr, "%sokf-cli update: %s\n", clear, message)
		case "applying":
			fmt.Fprintf(os.Stderr, "%sokf-cli update: %s\n", clear, message)
		case "warning":
			color.Yellow("%sokf-cli update: warning: %s", clear, message)
		}
	}
}

func makeUpdatePromptHandler() func(changeType differ.ChangeType, files []differ.FileChange) (apply bool, applyAll bool, cancel bool) {
	reader := bufio.NewReader(os.Stdin)

	return func(changeType differ.ChangeType, files []differ.FileChange) (apply bool, applyAll bool, cancel bool) {
		var action string
		switch changeType {
		case differ.ChangeModified:
			action = "Modify"
		case differ.ChangeDeleted:
			action = "Delete"
		default:
			action = "Change"
		}

		for _, f := range files {
			color.Yellow("\n%s: %s", action, f.Path)
		}

		fmt.Print("\nApply this change? [y]es / [n]o / [a]ll / [c]ancel: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return false, false, true
		}

		input = strings.TrimSpace(strings.ToLower(input))
		switch input {
		case "y", "yes":
			return true, false, false
		case "n", "no":
			return false, false, false
		case "a", "all":
			return true, true, false
		case "c", "cancel":
			return false, false, true
		default:
			// Default to no
			return false, false, false
		}
	}
}
