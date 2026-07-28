# Testable Documentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build testable documentation infrastructure so fenced code blocks in Markdown can be extracted, tested with Bats, and coverage-reported.

**Architecture:** A Go extraction tool (`cmd/doctest/`) parses Markdown with goldmark, finds fenced code blocks annotated with `{test="..."}`, writes them to disk. Hand-written Bats tests run those snippets. Coverage reporting flags untested executable blocks. Everything integrates into existing Makefile targets and CI workflows.

**Tech Stack:** Go 1.25+ (goldmark, go-yaml), Bats-core + bats-support + bats-assert (git submodules), Hugo 0.155.1 extended (existing), GNU Make.

**Spec:** `specs/015-testable-documentation/spec.md`

---

## File Map

### New files

| Path | Purpose |
|------|---------|
| `cmd/doctest/main.go` | CLI entry point: `extract` and `coverage` subcommands |
| `cmd/doctest/extract.go` | Markdown walker, attribute parser, snippet writer, manifest generator |
| `cmd/doctest/extract_test.go` | Table-driven unit tests for extraction and coverage |
| `tests/docs/setup_suite.bash` | Bats suite setup: sets `SNIPPETS_DIR` |
| `tests/docs/helpers/bash.bash` | `run_snippet()` helper function |
| `tests/docs/getting-started.bats` | Skeleton Bats test file (placeholder for future snippet tests) |
| `tests/libs/bats-core/` | Git submodule |
| `tests/libs/bats-support/` | Git submodule |
| `tests/libs/bats-assert/` | Git submodule |

### Modified files

| Path | Change |
|------|--------|
| `go.mod` | Add `github.com/yuin/goldmark` dependency |
| `go.sum` | Updated by `go mod tidy` |
| `Makefile` | Add `test-docs-extract`, `test-docs`, `test-docs-coverage` targets; update `check` |
| `.gitignore` | Add `/doctest` binary and `.test-output/` directory |
| `.github/workflows/ci.yml` | Add `checkout submodules`, doc tests step after Hugo build |
| `.github/workflows/deploy-gh-pages.yml` | Add `checkout submodules`, doc tests step before artifact upload |
| `CONTRIBUTING.md` | New "Testing Documentation" section |
| `README.md` | Add `cmd/doctest/` and `tests/docs/` to project structure; mention `make test-docs` |

---

## Task Dependency Graph

```
Task 1 (Go extractor core + coverage)
  └─> Task 2 (Go extractor tests)
        └─> Task 3 (Bats submodules + harness)
              └─> Task 4 (Makefile targets)
                    └─> Task 5 (CI integration)
                          └─> Task 6 (Documentation updates)
                                └─> Task 7 (.gitignore + housekeeping)
                                      └─> Task 8 (End-to-end verification)
```

Tasks 3 and 7 can be parallelized with adjacent tasks if using subagent-driven-development (they have no code dependencies on each other beyond ordering).

---

### Task 1: Go Extraction Tool — Core Implementation

**Files:**
- Create: `cmd/doctest/main.go`
- Create: `cmd/doctest/extract.go`
- Modify: `go.mod` (add goldmark dependency)

**Context:** The existing `cmd/sync-content/` tool is a `package main` with all files in one directory. Follow the same pattern. The project already uses `github.com/goccy/go-yaml` for YAML parsing. Use goldmark (`github.com/yuin/goldmark`) for Markdown AST parsing — the same parser Hugo uses internally.

- [ ] **Step 1: Add goldmark dependency**

```bash
go get github.com/yuin/goldmark@latest
go mod tidy
```

Verify `go.mod` now contains `github.com/yuin/goldmark`.

- [ ] **Step 2: Create `cmd/doctest/main.go`**

Create `cmd/doctest/main.go` with the CLI entry point:

```go
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
			slog.Error("coverage failed", "error", err)
			return 1
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\nusage: doctest <extract|coverage> [flags]\n", subcmd)
		return 1
	}
	return 0
}
```

- [ ] **Step 3: Create `cmd/doctest/extract.go`**

Create `cmd/doctest/extract.go` with the extraction and coverage logic:

```go
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	goyaml "github.com/goccy/go-yaml"
)

// testableLangs are languages whose untested blocks produce coverage warnings.
var testableLangs = map[string]bool{
	"bash":  true,
	"sh":    true,
	"shell": true,
	"zsh":   true,
}

// nonTestableLangs are explicitly silenced in coverage reports.
var nonTestableLangs = map[string]bool{
	"text": true, "plaintext": true, "console": true,
	"yaml": true, "toml": true, "json": true,
	"xml": true, "csv": true, "markdown": true,
	"go": true, "python": true,
}

var testIDPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// snippet represents a single extracted code block.
type snippet struct {
	Test       string `json:"test"`
	File       string `json:"file"`
	SourceLine int    `json:"source_line"`
	Language   string `json:"language"`
}

// manifest represents the JSON manifest for a single page.
type manifest struct {
	Page     string    `json:"page"`
	Snippets []snippet `json:"snippets"`
}

// codeBlock is an intermediate representation of a parsed fenced code block.
type codeBlock struct {
	lang     string
	testName string
	content  []byte
	line     int // 1-based line number in the source file
}

// frontmatter holds the subset of YAML frontmatter we care about.
type frontmatter struct {
	TestableDocs *bool `yaml:"testable_docs"`
}

// parseFrontmatter extracts YAML frontmatter from Markdown source.
// Returns nil (not opted-out) if no frontmatter is found.
func parseFrontmatter(source []byte) (*frontmatter, error) {
	// Find --- delimiters
	if !bytes.HasPrefix(source, []byte("---")) {
		return nil, nil
	}
	end := bytes.Index(source[3:], []byte("\n---"))
	if end == -1 {
		return nil, nil
	}
	yamlBytes := source[3 : end+3]

	var fm frontmatter
	if err := goyaml.Unmarshal(yamlBytes, &fm); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}
	return &fm, nil
}

// isOptedOut returns true if the page has testable_docs: false.
func isOptedOut(fm *frontmatter) bool {
	return fm != nil && fm.TestableDocs != nil && !*fm.TestableDocs
}

// parseInfoString extracts the language and test attribute from a fenced code
// block info string. Examples:
//
//	"bash {test=\"install\"}"  -> ("bash", "install")
//	"bash"                     -> ("bash", "")
//	""                         -> ("", "")
func parseInfoString(info string) (lang string, testName string, err error) {
	info = strings.TrimSpace(info)
	if info == "" {
		return "", "", nil
	}

	// Split on '{' to separate language from attributes
	langPart, attrPart, hasAttrs := strings.Cut(info, "{")
	lang = strings.TrimSpace(langPart)

	if !hasAttrs {
		return lang, "", nil
	}

	// Re-add the '{' for parser.ParseAttributes
	attrStr := "{" + attrPart
	reader := text.NewReader([]byte(attrStr))
	attrs, ok := parser.ParseAttributes(reader)
	if !ok {
		return lang, "", nil
	}

	for _, attr := range attrs {
		if string(attr.Name) == "test" {
			if v, ok := attr.Value.([]byte); ok {
				testName = string(v)
			}
			break
		}
	}

	if testName != "" && !testIDPattern.MatchString(testName) {
		return "", "", fmt.Errorf("invalid test identifier %q: must match [a-z0-9-]+", testName)
	}

	return lang, testName, nil
}

// extractBlocks parses a Markdown file and returns all fenced code blocks.
func extractBlocks(source []byte) ([]codeBlock, error) {
	md := goldmark.New()
	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)

	var blocks []codeBlock
	err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || node.Kind() != ast.KindFencedCodeBlock {
			return ast.WalkContinue, nil
		}
		fcb := node.(*ast.FencedCodeBlock)

		// Get the full info string
		var info string
		if fcb.Info != nil {
			info = string(fcb.Info.Segment.Value(source))
		}

		lang, testName, err := parseInfoString(info)
		if err != nil {
			return ast.WalkStop, fmt.Errorf("line %d: %w", fcb.Lines().At(0).Start, err)
		}

		// Collect code content
		var content []byte
		for i := 0; i < fcb.Lines().Len(); i++ {
			line := fcb.Lines().At(i)
			content = append(content, line.Value(source)...)
		}

		blocks = append(blocks, codeBlock{
			lang:     lang,
			testName: testName,
			content:  content,
			line:     lineNumber(source, node),
		})

		return ast.WalkContinue, nil
	})

	return blocks, err
}

// lineNumber computes the 1-based line number for an AST node's position.
func lineNumber(source []byte, node ast.Node) int {
	// Use the first line of the code block to find the position,
	// then scan backwards to find the fence line.
	// The node itself doesn't track the fence line, but we can use
	// the text segment positions.
	pos := 0
	if fcb, ok := node.(*ast.FencedCodeBlock); ok && fcb.Lines().Len() > 0 {
		seg := fcb.Lines().At(0)
		pos = seg.Start
	}
	// Count newlines before pos to get line number, then subtract 1
	// for the fence line itself.
	line := 1
	for i := 0; i < pos && i < len(source); i++ {
		if source[i] == '\n' {
			line++
		}
	}
	// The fence line (```) is one line before the first content line
	if line > 1 {
		line--
	}
	return line
}

// langExtension returns the file extension for a language identifier.
func langExtension(lang string) string {
	switch lang {
	case "bash", "sh", "shell", "zsh":
		return lang
	case "yaml", "yml":
		return "yaml"
	case "json":
		return "json"
	case "toml":
		return "toml"
	case "go":
		return "go"
	case "python", "py":
		return "py"
	default:
		if lang == "" {
			return "txt"
		}
		return lang
	}
}

// pageSlug computes the output directory name from a file path relative to
// the content directory. For "getting-started/_index.md" -> "getting-started".
// For "guides/advanced/_index.md" -> "guides-advanced".
// For "_index.md" at the root -> "root".
func pageSlug(relPath string) string {
	dir := filepath.Dir(relPath)
	if dir == "." || dir == "" {
		return "root"
	}
	// Replace path separators with hyphens
	return strings.ReplaceAll(filepath.ToSlash(dir), "/", "-")
}

// runExtract walks the content directory, extracts annotated code blocks,
// and writes them to the output directory.
func runExtract(contentDir, outputDir string) error {
	return filepath.WalkDir(contentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		source, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		// Check frontmatter opt-out
		fm, err := parseFrontmatter(source)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if isOptedOut(fm) {
			return nil
		}

		blocks, err := extractBlocks(source)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		// Filter to annotated blocks only
		var annotated []codeBlock
		for _, b := range blocks {
			if b.testName != "" {
				annotated = append(annotated, b)
			}
		}
		if len(annotated) == 0 {
			return nil
		}

		// Check for duplicate test names
		seen := make(map[string]int) // testName -> line number
		for _, b := range annotated {
			if prevLine, ok := seen[b.testName]; ok {
				return fmt.Errorf("%s: duplicate test name %q at lines %d and %d",
					path, b.testName, prevLine, b.line)
			}
			seen[b.testName] = b.line
		}

		// Compute output paths and write
		relPath, err := filepath.Rel(contentDir, path)
		if err != nil {
			return fmt.Errorf("computing relative path for %s: %w", path, err)
		}
		slug := pageSlug(relPath)
		pageDir := filepath.Join(outputDir, slug)
		if err := os.MkdirAll(pageDir, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", pageDir, err)
		}

		var snippets []snippet
		for i, b := range annotated {
			ext := langExtension(b.lang)
			filename := fmt.Sprintf("%02d-%s.%s", i+1, b.testName, ext)
			outPath := filepath.Join(pageDir, filename)

			if err := os.WriteFile(outPath, b.content, 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", outPath, err)
			}

			snippets = append(snippets, snippet{
				Test:       b.testName,
				File:       filename,
				SourceLine: b.line,
				Language:   b.lang,
			})
		}

		// Write manifest
		m := manifest{
			Page:     path,
			Snippets: snippets,
		}
		manifestData, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling manifest for %s: %w", path, err)
		}
		manifestPath := filepath.Join(pageDir, "manifest.json")
		if err := os.WriteFile(manifestPath, append(manifestData, '\n'), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", manifestPath, err)
		}

		return nil
	})
}

// coverageEntry represents an untested executable code block.
type coverageEntry struct {
	File     string
	Line     int
	Language string
}

// runCoverage walks the content directory and reports untested executable blocks.
func runCoverage(contentDir string) error {
	var entries []coverageEntry

	err := filepath.WalkDir(contentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		source, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		fm, err := parseFrontmatter(source)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if isOptedOut(fm) {
			return nil
		}

		blocks, err := extractBlocks(source)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		for _, b := range blocks {
			if b.testName != "" {
				continue // annotated = tested
			}
			if !testableLangs[b.lang] {
				continue // not a testable language
			}
			entries = append(entries, coverageEntry{
				File:     path,
				Line:     b.line,
				Language: b.lang,
			})
		}

		return nil
	})
	if err != nil {
		return err
	}

	if len(entries) > 0 {
		fmt.Printf("Untested executable code blocks (%d):\n", len(entries))
		for _, e := range entries {
			fmt.Printf("  %s:%d [%s]\n", e.File, e.Line, e.Language)
		}
	} else {
		fmt.Println("All executable code blocks are tested.")
	}

	return nil
}
```

- [ ] **Step 4: Verify compilation**

```bash
go build ./cmd/doctest/
```

Expected: no errors, no output. A `doctest` binary appears in the workspace root.

```bash
rm -f doctest
```

- [ ] **Step 5: Smoke test against existing content**

```bash
go run ./cmd/doctest extract --content-dir content/docs --output-dir /tmp/doctest-snippets
```

Expected: completes with exit 0. Since no blocks have `{test="..."}` yet, `/tmp/doctest-snippets` should be empty or not created.

```bash
go run ./cmd/doctest coverage --content-dir content/docs
```

Expected: lists untested bash/sh blocks from `getting-started/_index.md`.

- [ ] **Step 6: Commit**

```bash
git add cmd/doctest/main.go cmd/doctest/extract.go go.mod go.sum
git commit -m "feat: add doctest extraction tool

Go tool using goldmark to parse Markdown AST and extract fenced code
blocks annotated with {test=\"...\"} attributes. Supports extract and
coverage subcommands.

Part of testable documentation infrastructure (spec 015)."
```

---

### Task 2: Go Extraction Tool — Unit Tests

**Files:**
- Create: `cmd/doctest/extract_test.go`

**Context:** Follow the existing test patterns from `cmd/sync-content/path_test.go` — table-driven tests, `t.TempDir()` for filesystem tests, `testing` package only (no external test framework). The file under test is `cmd/doctest/extract.go` which exports `parseFrontmatter`, `isOptedOut`, `parseInfoString`, `extractBlocks`, `pageSlug`, `lineNumber`, `langExtension`, `runExtract`, `runCoverage`.

- [ ] **Step 1: Create `cmd/doctest/extract_test.go`**

```go
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseInfoStringBasic(t *testing.T) {
	tests := []struct {
		name     string
		info     string
		wantLang string
		wantTest string
		wantErr  bool
	}{
		{"empty", "", "", "", false},
		{"lang only", "bash", "bash", "", false},
		{"lang with test", `bash {test="install"}`, "bash", "install", false},
		{"no space before brace", `bash{test="install"}`, "bash", "install", false},
		{"attrs without test key", `bash {.highlight}`, "bash", "", false},
		{"test with hyphens and numbers", `sh {test="my-test-01"}`, "sh", "my-test-01", false},
		{"invalid test id uppercase", `bash {test="Install"}`, "", "", true},
		{"invalid test id spaces", `bash {test="my test"}`, "", "", true},
		{"invalid test id underscore", `bash {test="my_test"}`, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lang, testName, err := parseInfoString(tt.info)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseInfoString(%q) error = %v, wantErr = %v", tt.info, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if lang != tt.wantLang {
				t.Errorf("lang = %q, want %q", lang, tt.wantLang)
			}
			if testName != tt.wantTest {
				t.Errorf("testName = %q, want %q", testName, tt.wantTest)
			}
		})
	}
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantOptOut bool
		wantNil    bool
	}{
		{"no frontmatter", "# Hello\nworld", false, true},
		{"empty frontmatter", "---\n---\n# Hello", false, false},
		{"opted out", "---\ntestable_docs: false\n---\n# Hello", true, false},
		{"opted in explicitly", "---\ntestable_docs: true\n---\n# Hello", false, false},
		{"no testable_docs key", "---\ntitle: Test\n---\n# Hello", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, err := parseFrontmatter([]byte(tt.source))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil && fm != nil && fm.TestableDocs != nil {
				t.Fatal("expected nil TestableDocs")
			}
			if got := isOptedOut(fm); got != tt.wantOptOut {
				t.Errorf("isOptedOut = %v, want %v", got, tt.wantOptOut)
			}
		})
	}
}

func TestExtractBlocksBasic(t *testing.T) {
	source := []byte("---\ntitle: Test\n---\n\n# Hello\n\n```bash {test=\"install\"}\necho hello\n```\n")
	blocks, err := extractBlocks(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	b := blocks[0]
	if b.lang != "bash" {
		t.Errorf("lang = %q, want %q", b.lang, "bash")
	}
	if b.testName != "install" {
		t.Errorf("testName = %q, want %q", b.testName, "install")
	}
	if strings.TrimSpace(string(b.content)) != "echo hello" {
		t.Errorf("content = %q, want %q", string(b.content), "echo hello\n")
	}
}

func TestExtractBlocksOrdering(t *testing.T) {
	source := []byte("```bash {test=\"first\"}\necho 1\n```\n\nsome text\n\n```bash {test=\"second\"}\necho 2\n```\n")
	blocks, err := extractBlocks(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var annotated []codeBlock
	for _, b := range blocks {
		if b.testName != "" {
			annotated = append(annotated, b)
		}
	}
	if len(annotated) != 2 {
		t.Fatalf("got %d annotated blocks, want 2", len(annotated))
	}
	if annotated[0].testName != "first" {
		t.Errorf("first block testName = %q, want %q", annotated[0].testName, "first")
	}
	if annotated[1].testName != "second" {
		t.Errorf("second block testName = %q, want %q", annotated[1].testName, "second")
	}
}

func TestExtractBlocksNoInfoString(t *testing.T) {
	source := []byte("```\necho hello\n```\n")
	blocks, err := extractBlocks(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	if blocks[0].lang != "" {
		t.Errorf("lang = %q, want empty", blocks[0].lang)
	}
	if blocks[0].testName != "" {
		t.Errorf("testName = %q, want empty", blocks[0].testName)
	}
}

func TestExtractBlocksNonTestableLanguage(t *testing.T) {
	source := []byte("```yaml {test=\"my-config\"}\nkey: value\n```\n")
	blocks, err := extractBlocks(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	if blocks[0].lang != "yaml" {
		t.Errorf("lang = %q, want %q", blocks[0].lang, "yaml")
	}
	if blocks[0].testName != "my-config" {
		t.Errorf("testName = %q, want %q", blocks[0].testName, "my-config")
	}
}

func TestPageSlug(t *testing.T) {
	tests := []struct {
		relPath string
		want    string
	}{
		{"getting-started/_index.md", "getting-started"},
		{"guides/advanced/_index.md", "guides-advanced"},
		{"_index.md", "root"},
		{"overview.md", "root"},
	}
	for _, tt := range tests {
		t.Run(tt.relPath, func(t *testing.T) {
			if got := pageSlug(tt.relPath); got != tt.want {
				t.Errorf("pageSlug(%q) = %q, want %q", tt.relPath, got, tt.want)
			}
		})
	}
}

func TestLangExtension(t *testing.T) {
	tests := []struct {
		lang string
		want string
	}{
		{"bash", "bash"},
		{"sh", "sh"},
		{"yaml", "yaml"},
		{"python", "py"},
		{"go", "go"},
		{"", "txt"},
		{"rust", "rust"},
	}
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			if got := langExtension(tt.lang); got != tt.want {
				t.Errorf("langExtension(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

func TestRunExtractBasic(t *testing.T) {
	contentDir := t.TempDir()
	outputDir := t.TempDir()

	md := "---\ntitle: Test\n---\n\n```bash {test=\"hello\"}\necho hello\n```\n"
	subDir := filepath.Join(contentDir, "getting-started")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "_index.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runExtract(contentDir, outputDir); err != nil {
		t.Fatalf("runExtract error: %v", err)
	}

	// Check snippet file
	snippetPath := filepath.Join(outputDir, "getting-started", "01-hello.bash")
	data, err := os.ReadFile(snippetPath)
	if err != nil {
		t.Fatalf("reading snippet: %v", err)
	}
	if strings.TrimSpace(string(data)) != "echo hello" {
		t.Errorf("snippet content = %q, want %q", string(data), "echo hello\n")
	}

	// Check manifest
	manifestPath := filepath.Join(outputDir, "getting-started", "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}
	var m manifest
	if err := json.Unmarshal(manifestData, &m); err != nil {
		t.Fatalf("parsing manifest: %v", err)
	}
	if len(m.Snippets) != 1 {
		t.Fatalf("manifest has %d snippets, want 1", len(m.Snippets))
	}
	if m.Snippets[0].Test != "hello" {
		t.Errorf("snippet test = %q, want %q", m.Snippets[0].Test, "hello")
	}
	if m.Snippets[0].Language != "bash" {
		t.Errorf("snippet language = %q, want %q", m.Snippets[0].Language, "bash")
	}
}

func TestRunExtractDuplicateError(t *testing.T) {
	contentDir := t.TempDir()
	outputDir := t.TempDir()

	md := "```bash {test=\"dupe\"}\necho 1\n```\n\n```bash {test=\"dupe\"}\necho 2\n```\n"
	if err := os.WriteFile(filepath.Join(contentDir, "test.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runExtract(contentDir, outputDir)
	if err == nil {
		t.Fatal("expected error for duplicate test names, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate test name") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "duplicate test name")
	}
}

func TestRunExtractFrontmatterOptOut(t *testing.T) {
	contentDir := t.TempDir()
	outputDir := t.TempDir()

	md := "---\ntestable_docs: false\n---\n\n```bash {test=\"hello\"}\necho hello\n```\n"
	if err := os.WriteFile(filepath.Join(contentDir, "test.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runExtract(contentDir, outputDir); err != nil {
		t.Fatalf("runExtract error: %v", err)
	}

	// Output directory should have no subdirectories
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("output dir has %d entries, want 0 (page should be skipped)", len(entries))
	}
}

func TestRunExtractOrdering(t *testing.T) {
	contentDir := t.TempDir()
	outputDir := t.TempDir()

	md := "```bash {test=\"alpha\"}\necho a\n```\n\n```bash {test=\"beta\"}\necho b\n```\n\n```bash {test=\"gamma\"}\necho c\n```\n"
	if err := os.WriteFile(filepath.Join(contentDir, "test.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runExtract(contentDir, outputDir); err != nil {
		t.Fatalf("runExtract error: %v", err)
	}

	expected := []string{"01-alpha.bash", "02-beta.bash", "03-gamma.bash"}
	pageDir := filepath.Join(outputDir, "root")
	for _, name := range expected {
		if _, err := os.Stat(filepath.Join(pageDir, name)); err != nil {
			t.Errorf("expected file %s not found: %v", name, err)
		}
	}
}

func TestRunCoverageReport(t *testing.T) {
	contentDir := t.TempDir()

	// One tested, one untested bash block, one yaml block (not testable)
	md := "```bash {test=\"tested\"}\necho tested\n```\n\n```bash\necho untested\n```\n\n```yaml\nkey: value\n```\n"
	if err := os.WriteFile(filepath.Join(contentDir, "test.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	// Capture coverage output by running the function directly
	// runCoverage prints to stdout but we just verify it doesn't error
	err := runCoverage(contentDir)
	if err != nil {
		t.Fatalf("runCoverage error: %v", err)
	}
}

func TestRunCoverageOptOut(t *testing.T) {
	contentDir := t.TempDir()

	md := "---\ntestable_docs: false\n---\n\n```bash\necho untested\n```\n"
	if err := os.WriteFile(filepath.Join(contentDir, "test.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should not error and should not report the untested block
	err := runCoverage(contentDir)
	if err != nil {
		t.Fatalf("runCoverage error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

```bash
go test -v ./cmd/doctest/...
```

Expected: all tests pass.

- [ ] **Step 3: Run tests with race detector**

```bash
go test -race ./cmd/doctest/...
```

Expected: passes with zero race warnings.

- [ ] **Step 4: Commit**

```bash
git add cmd/doctest/extract_test.go
git commit -m "test: add unit tests for doctest extraction tool

Table-driven tests covering info string parsing, frontmatter opt-out,
block extraction, ordering, duplicate detection, page slug derivation,
language extensions, manifest generation, and coverage reporting."
```

---

### Task 3: Bats Test Harness Setup

**Files:**
- Create: `tests/docs/setup_suite.bash`
- Create: `tests/docs/helpers/bash.bash`
- Create: `tests/docs/getting-started.bats`
- Create: `tests/libs/` (git submodules)
- Modify: `.gitmodules` (created by `git submodule add`)

**Context:** Bats-core, bats-support, and bats-assert are installed as git submodules. This is the standard distribution pattern — no brew/npm dependency. The `tests/` directory does not exist yet. The project has no `.gitmodules` file yet.

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p tests/docs/helpers tests/libs
```

- [ ] **Step 2: Add Bats git submodules**

```bash
git submodule add https://github.com/bats-core/bats-core.git tests/libs/bats-core
git submodule add https://github.com/bats-core/bats-support.git tests/libs/bats-support
git submodule add https://github.com/bats-core/bats-assert.git tests/libs/bats-assert
```

This creates `.gitmodules` and clones the repos into `tests/libs/`.

- [ ] **Step 3: Create `tests/docs/setup_suite.bash`**

```bash
# SPDX-License-Identifier: Apache-2.0

# Suite-level setup for documentation tests.
# Sets the default snippets directory. Individual .bats files load
# their own libraries in setup().

export SNIPPETS_DIR="${SNIPPETS_DIR:-/tmp/doctest-snippets}"
```

- [ ] **Step 4: Create `tests/docs/helpers/bash.bash`**

```bash
# SPDX-License-Identifier: Apache-2.0

# Helper for running extracted bash snippets.
# Usage in a @test block:
#   run_snippet "getting-started/01-install-complyctl.bash"

run_snippet() {
    local snippet="$SNIPPETS_DIR/$1"
    [[ -f "$snippet" ]] || { echo "Snippet not found: $snippet" >&2; return 1; }
    run bash "$snippet"
}
```

- [ ] **Step 5: Create `tests/docs/getting-started.bats`**

```bash
# SPDX-License-Identifier: Apache-2.0

# Documentation tests for content/docs/getting-started/_index.md
#
# Tests will be added here as code blocks in the getting started guide
# are annotated with {test="..."} attributes. Each @test name must
# match a test attribute value in the Markdown source.

setup() {
    load 'helpers/bash'
    load '../libs/bats-support/load'
    load '../libs/bats-assert/load'
}

# Placeholder: add @test blocks as snippets are annotated.
# Example:
#
# @test "install-complyctl" {
#     run_snippet "getting-started/01-install-complyctl.bash"
#     assert_success
# }
```

- [ ] **Step 6: Verify Bats runs (expect no tests)**

```bash
tests/libs/bats-core/bin/bats tests/docs/
```

Expected: `0 tests, 0 failures` or similar output indicating no test functions found. The skeleton file has no `@test` blocks, so Bats should report zero tests.

- [ ] **Step 7: Commit**

```bash
git add .gitmodules tests/libs/bats-core tests/libs/bats-support tests/libs/bats-assert
git add tests/docs/setup_suite.bash tests/docs/helpers/bash.bash tests/docs/getting-started.bats
git commit -m "feat: add Bats test harness for documentation tests

Install bats-core, bats-support, and bats-assert as git submodules.
Set up test directory structure with helpers and a skeleton test file
for the getting-started guide.

Part of testable documentation infrastructure (spec 015)."
```

---

### Task 4: Makefile Targets

**Files:**
- Modify: `Makefile`

**Context:** The Makefile is 153 lines. Sections are separated by comment headers. The `check` target is on line 77. New doc test targets go after the "Content sync" section (after line 108). The convention uses `##` for help comments, `.PHONY` before each target, and `@` prefix for quiet execution.

- [ ] **Step 1: Add documentation test section to Makefile**

Insert after line 108 (after the `sync-single` target), before the "Hugo / Node" section:

```makefile
# ---------------------------------------------------------------------------
# Documentation tests — extract, validate, and test code blocks
# ---------------------------------------------------------------------------

.PHONY: test-docs-extract
test-docs-extract: ## Extract testable code blocks from documentation
	@go run ./cmd/doctest extract --content-dir content/docs --output-dir /tmp/doctest-snippets

.PHONY: test-docs
test-docs: test-docs-extract ## Run documentation tests (Bats)
	@tests/libs/bats-core/bin/bats tests/docs/

.PHONY: test-docs-coverage
test-docs-coverage: ## Report untested code blocks in documentation
	@go run ./cmd/doctest coverage --content-dir content/docs
```

- [ ] **Step 2: Update `check` meta-target**

Change line 77 from:

```makefile
check: vet fmt-check test-race ## Run vet + fmt-check + race tests (CI equivalent)
```

to:

```makefile
check: vet fmt-check test-race test-docs-coverage ## Run vet + fmt-check + race tests + doc coverage (CI equivalent)
```

Also update the quick reference comment at the top of the Makefile (line 11) to mention doc tests:

```makefile
#   make check           — vet + fmt-check + race tests + doc coverage
```

- [ ] **Step 3: Update Go targets to include doctest**

The existing `test` and `test-race` targets only test `./cmd/sync-content/...`. Update them to also test `./cmd/doctest/...`:

Change `test` (line 53):
```makefile
test: ## Run all Go unit tests
	go test $(SYNC_PKG) ./cmd/doctest/...
```

Change `test-race` (line 57):
```makefile
test-race: ## Run Go tests with the race detector
	go test -race $(SYNC_PKG) ./cmd/doctest/...
```

Also update `vet` (line 61) and `fmt`/`fmt-check` to cover the new package:

Change `vet`:
```makefile
vet: ## Run go vet
	go vet $(SYNC_PKG) ./cmd/doctest/...
```

Change `fmt`:
```makefile
fmt: ## Format Go source files with gofmt
	gofmt -w cmd/sync-content/ cmd/doctest/
```

Change `fmt-check`:
```makefile
fmt-check: ## Check Go formatting (non-destructive)
	@out=$$(gofmt -l cmd/sync-content/ cmd/doctest/); \
	if [ -n "$$out" ]; then \
		echo "The following files need formatting:"; \
		echo "$$out"; \
		exit 1; \
	fi
```

- [ ] **Step 4: Verify targets work**

```bash
make help
```

Expected: new targets `test-docs-extract`, `test-docs`, `test-docs-coverage` appear in help output.

```bash
make test-docs-extract
```

Expected: exit 0 (no annotated blocks to extract yet).

```bash
make test-docs-coverage
```

Expected: lists untested bash blocks from getting-started page.

```bash
make test
```

Expected: runs both sync-content and doctest Go tests.

```bash
make check
```

Expected: runs vet, fmt-check, race tests, and doc coverage.

- [ ] **Step 5: Commit**

```bash
git add Makefile
git commit -m "feat: add Makefile targets for documentation testing

Add test-docs-extract, test-docs, and test-docs-coverage targets.
Include doctest package in existing Go test/vet/fmt targets.
Add test-docs-coverage to check meta-target."
```

---

### Task 5: CI Integration

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/deploy-gh-pages.yml`

**Context:** Both workflows use `actions/checkout` with SHA-pinned versions and `persist-credentials: false`. Neither currently checks out submodules. The Bats submodules in `tests/libs/` must be available for `make test-docs` to work. The `ci.yml` checkout is at line 16-18, `deploy-gh-pages.yml` at line 22-24.

- [ ] **Step 1: Update `ci.yml` — add submodule checkout and doc tests**

In `.github/workflows/ci.yml`, update the checkout step to include submodules, and add a doc tests step after the Hugo build:

Update the Checkout step (lines 16-18) to:
```yaml
      - name: Checkout
        uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0
        with:
          persist-credentials: false
          submodules: true
```

Add after the "Build site" step (after line 48):
```yaml
      - name: Run documentation tests
        run: make test-docs
```

- [ ] **Step 2: Update `deploy-gh-pages.yml` — add submodule checkout and doc tests**

In `.github/workflows/deploy-gh-pages.yml`, update the checkout step to include submodules, and add a doc tests step after the Hugo build:

Update the Checkout step (lines 22-24) to:
```yaml
      - name: Checkout
        uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0
        with:
          persist-credentials: false
          submodules: true
```

Add after the "Build" step (after line 66):
```yaml
      - name: Run documentation tests
        run: make test-docs
```

- [ ] **Step 3: Also update Go test step in `ci.yml` to include doctest**

Update line 40 from:
```yaml
      - name: Run tests
        run: go test -race ./cmd/sync-content/...
```

to:
```yaml
      - name: Run tests
        run: go test -race ./cmd/sync-content/... ./cmd/doctest/...
```

And in `deploy-gh-pages.yml`, update line 57-58 from:
```yaml
      - name: Run tests
        run: go test -race ./cmd/sync-content/...
```

to:
```yaml
      - name: Run tests
        run: go test -race ./cmd/sync-content/... ./cmd/doctest/...
```

- [ ] **Step 4: Verify YAML validity**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml')); print('ci.yml OK')"
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/deploy-gh-pages.yml')); print('deploy-gh-pages.yml OK')"
```

Expected: both print OK.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/ci.yml .github/workflows/deploy-gh-pages.yml
git commit -m "ci: add documentation test steps to CI workflows

Check out git submodules (bats-core, bats-support, bats-assert) and
run make test-docs in both ci.yml and deploy-gh-pages.yml. Include
cmd/doctest in Go race-test steps."
```

---

### Task 6: Documentation Updates

**Files:**
- Modify: `CONTRIBUTING.md`
- Modify: `README.md`
- Create: `AGENTS.md` (if the project wants one; otherwise skip)

**Context:** `CONTRIBUTING.md` is 537 lines with a clear section structure. The spec calls for a new "Testing Documentation" section after existing testing sections. `README.md` is 49 lines. The project has no `AGENTS.md` file (only `.agents/skills/.gitkeep`).

- [ ] **Step 1: Add "Testing Documentation" section to CONTRIBUTING.md**

Insert after the "Testing the Sync Tool" section (after line 476, before the "### Testing Tips" section). Add:

```markdown
### Testing Documentation

Documentation pages with shell commands use **testable code blocks** — fenced
code blocks annotated with a `{test="..."}` attribute that links them to
automated tests.

**Annotating a code block:**

````markdown
```bash {test="install-complyctl"}
go install github.com/complytime/complyctl@latest
```
````

The `test` value must be lowercase alphanumeric with hyphens (`[a-z0-9-]+`).
It becomes both the extracted snippet filename and the Bats test reference.
Each value must be unique within a page.

**Writing the corresponding test:**

Create or update a `.bats` file in `tests/docs/` matching the page name:

```bash
# tests/docs/getting-started.bats

@test "install-complyctl" {
    run_snippet "getting-started/01-install-complyctl.bash"
    assert_success
}
```

**Opting out a page:** Add `testable_docs: false` to the page's YAML frontmatter
to skip it entirely from extraction and coverage reporting.

**Make targets:**

| Target | What it does |
|--------|-------------|
| `make test-docs-extract` | Extract annotated code blocks to `/tmp/doctest-snippets` |
| `make test-docs` | Extract + run Bats tests |
| `make test-docs-coverage` | Report untested executable code blocks (warnings only) |

Coverage warnings are non-blocking — they show which blocks could benefit from
test annotations but do not fail the build.
```

- [ ] **Step 2: Update CONTRIBUTING.md table of contents**

Add "Testing Documentation" to the table of contents under "Common Tasks":

After the line `  - [Add Images](#add-images)` (line 23), the existing TOC does not have a "Testing Documentation" entry. Find the "Development Workflow" section entry and add the new entry in the appropriate location. Insert under the existing testing items at the right nesting level.

Actually, looking at the TOC structure, "Testing the Sync Tool" is under "Development Workflow". Add "Testing Documentation" after it:

Find the line:
```
- [Troubleshooting](#troubleshooting)
```

And add before it (under Development Workflow):
```
  - [Testing Documentation](#testing-documentation)
```

Wait — looking more carefully, the TOC items for Development Workflow sub-sections are not listed in the TOC. The "Testing the Sync Tool" section is not in the TOC either. So just add the section content and it will be discoverable by scrolling or heading search.

- [ ] **Step 3: Update CONTRIBUTING.md PR checklist**

Add a checklist item for documentation testing. Find the PR checklist section (around line 399) and add:

```markdown
- [ ] If documentation code blocks were changed: `make test-docs` passes
```

- [ ] **Step 4: Update README.md project structure**

In `README.md`, update the project structure tree (lines 23-33) to include the new directories:

```
website/
├── cmd/sync-content/      # Go content sync tool (10 source files, package main)
├── cmd/doctest/           # Go documentation test extraction tool
├── config/_default/       # Hugo configuration (TOML)
├── content/docs/          # Markdown content (projects/ is generated by sync tool)
├── data/projects.json     # Generated landing page cards (gitignored)
├── layouts/               # Custom Hugo layout overrides
├── tests/docs/            # Bats documentation tests
├── tests/libs/            # Bats test libraries (git submodules)
├── sync-config.yaml       # Declarative sync configuration
├── .content-lock.json     # Approved upstream SHAs per repo (committed)
└── .github/workflows/     # CI, deploy, weekly content check
```

- [ ] **Step 5: Update README.md quick start / development info**

After the "Production build" line (line 19), add:

```markdown
**Documentation tests**: `make test-docs` extracts annotated code blocks and runs Bats tests against them.
```

- [ ] **Step 6: Create AGENTS.md**

Create `AGENTS.md` in the project root with guidance for AI agents:

```markdown
# Agent Instructions

## Documentation with Shell Commands

When editing documentation that contains shell commands:

1. **Always add `{test="..."}` attributes** to fenced code blocks that contain
   runnable shell commands. Use lowercase alphanumeric identifiers with hyphens.

2. **Write a corresponding Bats test** in `tests/docs/` before fixing a snippet
   (TDD for docs). The test name must match the `test` attribute value.

3. **Run `make test-docs-coverage`** to check for untested code blocks.

4. **Run `make test-docs`** to verify all annotated snippets pass their tests.

## Go Code

- Run `make check` before committing (includes `go vet`, `gofmt`, race tests,
  and doc coverage).
- Follow existing patterns in `cmd/sync-content/` for test structure.
```

- [ ] **Step 7: Commit**

```bash
git add CONTRIBUTING.md README.md AGENTS.md
git commit -m "docs: add testable documentation workflow to contributor guides

Add Testing Documentation section to CONTRIBUTING.md with annotation
convention, Bats test examples, and make targets. Update README.md
project structure. Create AGENTS.md with documentation testing guidance."
```

---

### Task 7: .gitignore and Housekeeping

**Files:**
- Modify: `.gitignore`

**Context:** The `.gitignore` is 60 lines. Extracted snippets go to `/tmp/doctest-snippets` which is outside the repo and doesn't need gitignoring. But the `doctest` binary (if built locally) should be ignored, similar to the existing `/sync-content` ignore on line 13.

- [ ] **Step 1: Add doctest binary to .gitignore**

After the existing line `/sync-content` (line 13), add:

```gitignore
/doctest
```

Also add a test output directory entry:

```gitignore
# ─── Test output ─────────────────────────────────────────────────────
.test-output/
```

- [ ] **Step 2: Commit**

```bash
git add .gitignore
git commit -m "chore: gitignore doctest binary and test output directory"
```

---

### Task 8: End-to-End Verification

**Files:** None created or modified. This task verifies the full pipeline.

**Context:** At this point all code is written and committed. This task runs through the complete workflow to verify everything works together.

- [ ] **Step 1: Run Go tests for doctest**

```bash
go test -v ./cmd/doctest/...
```

Expected: all tests pass.

- [ ] **Step 2: Run Go tests with race detector**

```bash
go test -race ./cmd/doctest/...
```

Expected: passes with zero data race warnings.

- [ ] **Step 3: Run full check**

```bash
make check
```

Expected: vet, fmt-check, race tests (sync-content + doctest), and doc coverage all pass. Coverage reports untested bash blocks in getting-started page.

- [ ] **Step 4: Run extraction**

```bash
make test-docs-extract
```

Expected: exit 0. `/tmp/doctest-snippets` should be empty (no annotated blocks yet).

- [ ] **Step 5: Run doc tests**

```bash
make test-docs
```

Expected: exit 0. Bats reports 0 tests (skeleton file has no `@test` blocks).

- [ ] **Step 6: Run coverage**

```bash
make test-docs-coverage
```

Expected: lists untested bash/sh blocks from getting-started page with file, line, language.

- [ ] **Step 7: Verify Hugo still renders correctly**

```bash
hugo --minify --gc 2>&1 | head -5
```

Expected: Hugo builds successfully. The `{test="..."}` attributes don't exist on any blocks yet, but verify Hugo configuration is still valid.

- [ ] **Step 8: Verify Go formatting**

```bash
make fmt-check
```

Expected: no unformatted files.

- [ ] **Step 9: Review all commits**

```bash
git log --oneline feat/testable-documentaiton ^main
```

Expected: 7 commits (Tasks 1-7) in logical order with conventional commit messages.
