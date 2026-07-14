package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/importer"
	"github.com/chasedputnam/pyra/internal/types"
)

var importCmd = &cobra.Command{
	Use:   "import <path>",
	Short: "Import local docs into an OKF bundle",
	Long:  `Import local Markdown, MDX, HTML, or text files into an Open Knowledge Format bundle.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runImport,
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().String("out", "", "Output OKF bundle directory (required)")
	importCmd.Flags().String("source-name", "", "Source name for the bundle")
	importCmd.Flags().StringSlice("include", nil, "Include glob patterns")
	importCmd.Flags().StringSlice("exclude", nil, "Exclude glob patterns")
	importCmd.Flags().Bool("force", false, "Overwrite output directory")
	importCmd.Flags().Bool("dangerously-allow-unsafe-output", false, "Allow unsafe output paths with --force")
	importCmd.Flags().Bool("stable-timestamps", false, "Use deterministic timestamps")
	importCmd.Flags().String("summarize", "extractive", "Summarization mode: extractive (default) or llm")
	importCmd.Flags().String("summarize-algorithm", "lsa", "Extractive algorithm: lsa, lexrank, textrank, luhn, edmundson, sumbasic, kl, reduction, random")
	importCmd.Flags().String("language", "english", "Language for summarization")
	importCmd.Flags().String("edmundson-config", "", "Path to edmundson.config YAML (defaults to bundle/edmundson.config or ~/.config/pyra/edmundson.config)")

	_ = importCmd.MarkFlagRequired("out")
}

func runImport(cmd *cobra.Command, args []string) error {
	inputPath := args[0]
	outDir, _ := cmd.Flags().GetString("out")
	sourceName, _ := cmd.Flags().GetString("source-name")
	include, _ := cmd.Flags().GetStringSlice("include")
	exclude, _ := cmd.Flags().GetStringSlice("exclude")
	force, _ := cmd.Flags().GetBool("force")
	dangerouslyAllowUnsafeOutput, _ := cmd.Flags().GetBool("dangerously-allow-unsafe-output")
	stableTimestamps, _ := cmd.Flags().GetBool("stable-timestamps")
	summarizeMode, _ := cmd.Flags().GetString("summarize")
	summarizeAlgorithm, _ := cmd.Flags().GetString("summarize-algorithm")
	language, _ := cmd.Flags().GetString("language")
	edmundsonConfig, _ := cmd.Flags().GetString("edmundson-config")

	printStatus(fmt.Sprintf("pyra import: reading %s", inputPath))
	printStatus(fmt.Sprintf("pyra import: writing bundle to %s", outDir))

	result, err := importer.Import(importer.ImportOptions{
		InputPath:                    inputPath,
		OutDir:                       outDir,
		SourceName:                   sourceName,
		Include:                      include,
		Exclude:                      exclude,
		Force:                        force,
		DangerouslyAllowUnsafeOutput: dangerouslyAllowUnsafeOutput,
		StableTimestamps:             stableTimestamps,
		SummarizeMode:                summarizeMode,
		SummarizeAlgorithm:           summarizeAlgorithm,
		Language:                     language,
		EdmundsonConfigPath:          edmundsonConfig,
		OnProgress:                   makeImportProgressHandler(),
		OnSummarizeWarning:           makeImportWarningHandler(),
	})
	if err != nil {
		color.Red(err.Error())
		os.Exit(1)
	}

	fmt.Println("pyra import")
	fmt.Printf("Source: %s\n", inputPath)
	fmt.Printf("Concepts: %d written\n", len(result.Documents))
	fmt.Printf("Output: %s\n", outDir)
	if result.Stats != nil {
		printSummaryStats(result.Stats)
	}
	printStatus(fmt.Sprintf("pyra import: done, wrote %d concepts", len(result.Documents)))

	return nil
}

// makeImportProgressHandler returns a callback that prints progress to stderr.
func makeImportProgressHandler() func(int, int, string) {
	return func(index, total int, source string) {
		fmt.Fprintf(os.Stderr, "\rpyra import: summarizing %d/%d", index, total)
		if index == total {
			fmt.Fprintln(os.Stderr)
		}
	}
}

// makeImportWarningHandler returns a callback that logs summarization warnings.
func makeImportWarningHandler() func(string, string) {
	return func(path, message string) {
		color.Yellow("pyra import: warning: %s: %s", path, message)
	}
}

// printSummaryStats prints summary statistics in a human-readable form.
func printSummaryStats(stats *types.SummaryStats) {
	if stats == nil || stats.Total == 0 {
		return
	}
	fmt.Printf("Summaries: %d total", stats.Total)
	if len(stats.BySource) > 0 {
		var parts []string
		for source, count := range stats.BySource {
			parts = append(parts, fmt.Sprintf("%s=%d", source, count))
		}
		fmt.Printf(" (%s)", strings.Join(parts, ", "))
	}
	fmt.Println()
	if stats.Fallbacks > 0 {
		fmt.Printf("Fallbacks: %d\n", stats.Fallbacks)
	}
	if stats.Failed > 0 {
		fmt.Printf("Failed: %d\n", stats.Failed)
	}
}
