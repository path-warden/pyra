// Command pyra converts documentation into Open Knowledge Format bundles.
package main

import (
	"os"

	"github.com/chasedputnam/pyra/internal/cli"
)

// Version is set at build time via ldflags.
var version = "dev"

func main() {
	cli.SetVersion(version)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
