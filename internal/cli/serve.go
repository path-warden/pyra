package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/okfy/okf-mcp/internal/mcp"
)

var serveCmd = &cobra.Command{
	Use:   "serve <bundle>",
	Short: "Serve an OKF bundle via MCP",
	Long:  `Start an MCP (Model Context Protocol) server to serve an OKF bundle to AI agents.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().Bool("mcp", true, "Use MCP stdio transport (default)")
	serveCmd.Flags().String("name", "", "Server name")
	serveCmd.Flags().Int("max-result-chars", 12000, "Maximum characters in tool results")
}

func runServe(cmd *cobra.Command, args []string) error {
	bundleDir := args[0]
	useMCP, _ := cmd.Flags().GetBool("mcp")
	name, _ := cmd.Flags().GetString("name")
	maxResultChars, _ := cmd.Flags().GetInt("max-result-chars")

	if !useMCP {
		color.Red("Error: only MCP transport is supported")
		return fmt.Errorf("only MCP transport is supported")
	}

	server, err := mcp.NewServer(mcp.ServerOptions{
		BundleDir:      bundleDir,
		Name:           name,
		MaxResultChars: maxResultChars,
	})
	if err != nil {
		color.Red("Error: %s", err.Error())
		return err
	}

	return server.Serve()
}
