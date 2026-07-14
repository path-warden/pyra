// Package cli implements the pyra command-line interface.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/pyra/internal/crawler"
	"github.com/chasedputnam/pyra/internal/types"
)

var crawlCmd = &cobra.Command{
	Use:   "crawl <url>",
	Short: "Crawl a documentation website and create an OKF bundle",
	Args:  cobra.ExactArgs(1),
	RunE:  runCrawl,
}

func init() {
	crawlCmd.Flags().StringP("out", "o", "", "Output OKF bundle directory (required)")
	crawlCmd.Flags().Int("max-pages", 100, "Maximum pages to crawl")
	crawlCmd.Flags().Int("max-depth", 4, "Maximum crawl depth")
	crawlCmd.Flags().StringSlice("include", nil, "Include glob or regex patterns")
	crawlCmd.Flags().StringSlice("exclude", nil, "Exclude glob or regex patterns")
	crawlCmd.Flags().Bool("same-origin", true, "Stay on same origin")
	crawlCmd.Flags().Bool("respect-robots", true, "Respect robots.txt")
	crawlCmd.Flags().Int("concurrency", 4, "Fetch concurrency")
	crawlCmd.Flags().String("title", "", "Bundle title")
	crawlCmd.Flags().Bool("force", false, "Overwrite output directory")
	crawlCmd.Flags().Bool("dry-run", false, "List pages that would be crawled")
	crawlCmd.Flags().Bool("allow-private-network", false, "Allow localhost/private IP crawl targets")
	crawlCmd.Flags().Bool("dangerously-allow-unsafe-output", false, "Allow unsafe output paths with --force")
	crawlCmd.Flags().Bool("stable-timestamps", false, "Use deterministic timestamps")

	_ = crawlCmd.MarkFlagRequired("out")

	rootCmd.AddCommand(crawlCmd)
}

func runCrawl(cmd *cobra.Command, args []string) error {
	url := args[0]
	outDir, _ := cmd.Flags().GetString("out")
	maxPages, _ := cmd.Flags().GetInt("max-pages")
	maxDepth, _ := cmd.Flags().GetInt("max-depth")
	include, _ := cmd.Flags().GetStringSlice("include")
	exclude, _ := cmd.Flags().GetStringSlice("exclude")
	sameOrigin, _ := cmd.Flags().GetBool("same-origin")
	respectRobots, _ := cmd.Flags().GetBool("respect-robots")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	title, _ := cmd.Flags().GetString("title")
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	allowPrivateNetwork, _ := cmd.Flags().GetBool("allow-private-network")
	dangerouslyAllowUnsafeOutput, _ := cmd.Flags().GetBool("dangerously-allow-unsafe-output")
	stableTimestamps, _ := cmd.Flags().GetBool("stable-timestamps")

	var timestamp string
	if stableTimestamps {
		timestamp = "2026-06-14T00:00:00.000Z"
	}

	isTTY := isTerminal()

	opts := crawler.CrawlOptions{
		SeedURL:                      url,
		OutDir:                       outDir,
		MaxPages:                     maxPages,
		MaxDepth:                     maxDepth,
		Include:                      include,
		Exclude:                      exclude,
		SameOrigin:                   sameOrigin,
		RespectRobots:                respectRobots,
		Concurrency:                  concurrency,
		Title:                        title,
		Force:                        force,
		DryRun:                       dryRun,
		AllowPrivateNetwork:          allowPrivateNetwork,
		DangerouslyAllowUnsafeOutput: dangerouslyAllowUnsafeOutput,
		Timestamp:                    timestamp,
		OnProgress:                   makeCrawlProgressHandler(isTTY),
	}

	result, err := crawler.Crawl(context.Background(), opts)
	if err != nil {
		color.Red(err.Error())
		return err
	}

	if dryRun {
		fmt.Println("pyra crawl dry run")
		for _, page := range result.DryRunPages {
			fmt.Println(page)
		}
		return nil
	}

	fmt.Println("pyra crawl")
	fmt.Printf("Seed: %s\n", url)
	fmt.Printf("Pages: %d fetched, %d skipped, %d failed\n", result.PagesFetched, result.Skipped, result.Failed)
	fmt.Printf("Concepts: %d written\n", len(result.Documents))
	fmt.Printf("Output: %s\n", outDir)
	fmt.Println("\nNext:")
	fmt.Printf("  pyra validate %s\n", outDir)
	fmt.Printf("  pyra serve %s --mcp\n", outDir)

	return nil
}

func makeCrawlProgressHandler(isTTY bool) func(types.CrawlProgressEvent) {
	return func(event types.CrawlProgressEvent) {
		clear := ""
		if isTTY {
			clear = "\r\033[K"
		}

		switch event.Type {
		case "start":
			fmt.Fprintf(os.Stderr, "pyra crawl: starting %s (max %d pages, depth %d)\n", event.Seed, event.MaxPages, event.MaxDepth)
		case "fetch":
			fmt.Fprintf(os.Stderr, "%spyra crawl: fetching %d/%d, queued %d: %s", clear, event.Fetched, event.MaxPages, event.Queued, event.URL)
			if !isTTY {
				fmt.Fprintln(os.Stderr)
			}
		case "fetched":
			fmt.Fprintf(os.Stderr, "%spyra crawl: fetched %d/%d, queued %d, discovered +%d: %s\n", clear, event.Fetched, event.MaxPages, event.Queued, event.Discovered, event.URL)
		case "skipped":
			fmt.Fprintf(os.Stderr, "%spyra crawl: skipped %d/%d, queued %d: %s\n", clear, event.Fetched, event.MaxPages, event.Queued, event.URL)
		case "failed":
			fmt.Fprintf(os.Stderr, "%spyra crawl: failed %d/%d, queued %d: %s\n", clear, event.Fetched, event.MaxPages, event.Queued, event.URL)
		case "writing":
			fmt.Fprintf(os.Stderr, "%spyra crawl: writing %d concepts to %s\n", clear, event.Concepts, event.OutDir)
		}
	}
}

func isTerminal() bool {
	fileInfo, _ := os.Stderr.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
