package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/types"
	"github.com/chasedputnam/pyra/internal/validate"
)

var validateCmd = &cobra.Command{
	Use:   "validate <bundle>",
	Short: "Validate an OKF bundle",
	Long:  `Validate an OKF bundle for structural and content issues.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)

	validateCmd.Flags().Bool("json", false, "Output as JSON")
}

func runValidate(cmd *cobra.Command, args []string) error {
	bundleDir := args[0]
	jsonOutput, _ := cmd.Flags().GetBool("json")

	report, err := validate.ValidateBundle(bundleDir)
	if err != nil {
		if jsonOutput {
			printValidationJSON(report, err)
		} else {
			color.Red("Error: %s", err.Error())
		}
		return err
	}

	if jsonOutput {
		printValidationJSON(report, nil)
	} else {
		printValidationText(report)
	}

	if !report.Valid {
		os.Exit(1)
	}

	return nil
}

func printValidationJSON(report *types.ValidationReport, err error) {
	if report == nil {
		report = &types.ValidationReport{
			Valid:  false,
			Issues: []types.ValidationIssue{},
		}
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(data))
}

func printValidationText(report *types.ValidationReport) {
	fmt.Println("pyra validate")
	fmt.Printf("Concepts: %d\n", report.ConceptCount)
	fmt.Printf("Reserved files: %d\n", report.ReservedFileCount)

	errorCount := 0
	warningCount := 0
	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			errorCount++
		} else {
			warningCount++
		}
	}

	if errorCount > 0 {
		color.Red("Errors: %d", errorCount)
	} else {
		fmt.Println("Errors: 0")
	}

	if warningCount > 0 {
		color.Yellow("Warnings: %d", warningCount)
	} else {
		fmt.Println("Warnings: 0")
	}

	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			color.Red("  [%s] %s: %s", issue.Code, issue.Path, issue.Message)
		} else {
			color.Yellow("  [%s] %s: %s", issue.Code, issue.Path, issue.Message)
		}
	}

	if report.Valid {
		color.Green("\nBundle is valid.")
	} else {
		color.Red("\nBundle has validation errors.")
	}
}
