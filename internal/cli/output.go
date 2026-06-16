package cli

import (
	"fmt"
	"os"
)

// printStatus prints a status message to stderr.
func printStatus(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}
