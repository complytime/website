// SPDX-License-Identifier: Apache-2.0

// Command doctest extracts annotated code blocks from Markdown documentation
// and reports test coverage gaps.
//
// Fenced code blocks with a {test="<name>"} attribute in the info string are
// extracted to individual files. Hand-written Bats tests then verify them.
//
// Usage:
//
//	doctest extract --content-dir content/docs --output-dir /tmp/doctest-snippets
//	doctest coverage --content-dir content/docs
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
)

func main() { os.Exit(run()) }

func run() int {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: doctest <extract|coverage> [flags]")
		return 1
	}

	subcmd := os.Args[1]
	switch subcmd {
	case "extract":
		fs := flag.NewFlagSet("extract", flag.ExitOnError)
		contentDir := fs.String("content-dir", "", "Root directory of Markdown content (required)")
		outputDir := fs.String("output-dir", "", "Directory for extracted snippets (required)")
		if err := fs.Parse(os.Args[2:]); err != nil {
			return 1
		}
		if *contentDir == "" || *outputDir == "" {
			fmt.Fprintln(os.Stderr, "extract: --content-dir and --output-dir are required")
			return 1
		}
		if err := runExtract(*contentDir, *outputDir); err != nil {
			slog.Error("extract failed", "error", err)
			return 1
		}
	case "coverage":
		fs := flag.NewFlagSet("coverage", flag.ExitOnError)
		contentDir := fs.String("content-dir", "", "Root directory of Markdown content (required)")
		if err := fs.Parse(os.Args[2:]); err != nil {
			return 1
		}
		if *contentDir == "" {
			fmt.Fprintln(os.Stderr, "coverage: --content-dir is required")
			return 1
		}
		if err := runCoverage(*contentDir); err != nil {
			return 1 // coverage report already printed to stdout
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\nusage: doctest <extract|coverage> [flags]\n", subcmd)
		return 1
	}
	return 0
}
