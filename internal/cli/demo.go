package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/okfy/okf-mcp/internal/embed"
	"github.com/okfy/okf-mcp/internal/mcp"
	"github.com/okfy/okf-mcp/internal/validate"
)

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Run an offline demo with a bundled example",
	Long:  `Run an offline demo against a bundled example OKF bundle.`,
	Args:  cobra.NoArgs,
	RunE:  runDemo,
}

func init() {
	rootCmd.AddCommand(demoCmd)

	demoCmd.Flags().Bool("serve", false, "Start MCP server after demo output")
}

func runDemo(cmd *cobra.Command, args []string) error {
	serve, _ := cmd.Flags().GetBool("serve")

	var bundleDir string
	var cleanup func()

	// Try embedded bundle first
	if embed.HasDemoBundle() {
		tmpDir, err := embed.ExtractDemoBundle()
		if err != nil {
			color.Red("Failed to extract embedded demo bundle: %s", err.Error())
			return err
		}
		bundleDir = tmpDir
		cleanup = func() { os.RemoveAll(tmpDir) }
	} else {
		// Fall back to filesystem locations
		demoPaths := []string{
			"examples/bundles/okf-cli-docs",
			"../examples/bundles/okf-cli-docs",
			filepath.Join(os.Getenv("HOME"), ".okf-cli/demo-bundle"),
		}

		for _, p := range demoPaths {
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				bundleDir = p
				break
			}
		}

		if bundleDir == "" {
			color.Red("Demo bundle not found.")
			fmt.Println("\nTo use the demo, place an OKF bundle at one of these locations:")
			for _, p := range demoPaths {
				fmt.Printf("  %s\n", p)
			}
			fmt.Println("\nOr create one with:")
			fmt.Println("  okf-cli crawl https://example.com/docs --out examples/bundles/okf-cli-docs")
			return fmt.Errorf("demo bundle not found")
		}
	}

	// Cleanup temp dir on exit if not serving, or if server fails to start
	if cleanup != nil {
		if !serve {
			defer cleanup()
		}
	}

	fmt.Println("okf-cli demo")
	fmt.Printf("Bundle: %s\n", bundleDir)
	if cleanup != nil {
		fmt.Println("(temporary - extracted from embedded bundle)")
	}
	fmt.Println()

	// Validate the bundle
	report, err := validate.ValidateBundle(bundleDir)
	if err != nil {
		color.Red("Validation error: %s", err.Error())
		return err
	}

	if report.Valid {
		color.Green("Bundle is valid.")
	} else {
		color.Yellow("Bundle has %d issues.", len(report.Issues))
	}

	// Inspect the bundle
	stats, err := validate.InspectBundle(bundleDir)
	if err != nil {
		color.Red("Inspection error: %s", err.Error())
		return err
	}

	fmt.Println()
	fmt.Printf("Concepts: %d\n", stats.ConceptCount)
	fmt.Printf("Links: %d\n", stats.LinkCount)

	// Print MCP config
	fmt.Println()
	fmt.Println("MCP configuration (add to your client config):")
	fmt.Println()
	fmt.Printf(`{
  "mcpServers": {
    "okf-cli-demo": {
      "command": "okf-cli",
      "args": ["serve", "%s", "--mcp"]
    }
  }
}
`, bundleDir)

	// Print example prompts
	fmt.Println()
	fmt.Println("Example prompts:")
	fmt.Println("  - Search for concepts about MCP")
	fmt.Println("  - Read the getting started guide")
	fmt.Println("  - What are the main topics in this documentation?")

	if serve {
		fmt.Println()
		fmt.Println("Starting MCP server...")
		server, err := mcp.NewServer(mcp.ServerOptions{
			BundleDir: bundleDir,
			Name:      "okf-cli-demo",
		})
		if err != nil {
			if cleanup != nil {
				cleanup()
			}
			color.Red("Error: %s", err.Error())
			return err
		}
		// Note: cleanup happens when server/process exits
		return server.Serve()
	}

	return nil
}
