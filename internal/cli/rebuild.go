package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/store"
)

var rebuildCmd = &cobra.Command{
	Use:   "rebuild [store]",
	Short: "Regenerate derived indexes from the Markdown source of truth",
	Long: `Rebuild reloads a store from its Markdown files and regenerates the
derived indexes (full-text search and the relationship graph). Derived indexes
are caches: they can be deleted and rebuilt without affecting the canonical
Markdown.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRebuild,
}

func init() {
	rootCmd.AddCommand(rebuildCmd)
}

func runRebuild(cmd *cobra.Command, args []string) error {
	storeRoot := "."
	if len(args) == 1 {
		storeRoot = args[0]
	}
	cfg, err := config.Load(storeRoot)
	if err != nil {
		return err
	}
	s, err := store.Load(storeRoot, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	if err := s.Rebuild(); err != nil {
		return err
	}
	color.Green("Rebuilt store indexes.")
	fmt.Printf("  Canon artifacts: %d\n", len(s.Canon))
	fmt.Printf("  Reference concepts: %d\n", len(s.Reference))
	return nil
}
