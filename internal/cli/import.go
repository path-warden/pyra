package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/okfy/okf-mcp/internal/importer"
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

	importCmd.MarkFlagRequired("out")
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

	printStatus(fmt.Sprintf("okf-cli import: reading %s", inputPath))
	printStatus(fmt.Sprintf("okf-cli import: writing bundle to %s", outDir))

	result, err := importer.Import(importer.ImportOptions{
		InputPath:                    inputPath,
		OutDir:                       outDir,
		SourceName:                   sourceName,
		Include:                      include,
		Exclude:                      exclude,
		Force:                        force,
		DangerouslyAllowUnsafeOutput: dangerouslyAllowUnsafeOutput,
		StableTimestamps:             stableTimestamps,
	})
	if err != nil {
		color.Red(err.Error())
		os.Exit(1)
	}

	fmt.Println("okf-cli import")
	fmt.Printf("Source: %s\n", inputPath)
	fmt.Printf("Concepts: %d written\n", len(result.Documents))
	fmt.Printf("Output: %s\n", outDir)
	printStatus(fmt.Sprintf("okf-cli import: done, wrote %d concepts", len(result.Documents)))

	return nil
}
