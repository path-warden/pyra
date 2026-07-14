// Package cli implements the pyra command-line interface.
package cli

import (
	"github.com/spf13/cobra"
)

var version = "dev"

// SetVersion sets the version string for the CLI.
func SetVersion(v string) {
	version = v
}

var rootCmd = &cobra.Command{
	Use:   "pyra",
	Short: "Turn docs into agent memory with Open Knowledge Format and MCP",
	Long: `pyra converts documentation websites and local Markdown folders into
Open Knowledge Format (OKF) bundles. These bundles can be served via MCP
to AI agents like Claude, Codex, or Cursor.`,
}

func init() {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
