// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	goyaml "github.com/goccy/go-yaml"
)

// gitTrackedFiles returns the set of absolute paths for files under dir that
// are tracked by git or untracked-but-not-ignored (i.e. new files not yet
// committed that aren't covered by .gitignore). This requires dir to be
// inside a git repository.
func gitTrackedFiles(dir string) (map[string]bool, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", dir, err)
	}

	// --cached: tracked files; --others: untracked; --exclude-standard:
	// honour .gitignore, .git/info/exclude, and core.excludesFile.
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "--", "*.md")
	cmd.Dir = absDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files in %s: %w", absDir, err)
	}

	files := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		files[filepath.Join(absDir, line)] = true
	}
	return files, nil
}

// testableLangs are languages whose untested blocks produce coverage warnings.
var testableLangs = map[string]bool{
	"bash":  true,
	"sh":    true,
	"shell": true,
	"zsh":   true,
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
			return ast.WalkStop, fmt.Errorf("line %d: %w", lineNumber(source, node), err)
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
	base := filepath.Base(relPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))

	if name == "_index" || name == "index" {
		// Section pages: use directory path
		if dir == "." || dir == "" {
			return "root"
		}
		return strings.ReplaceAll(filepath.ToSlash(dir), "/", "-")
	}

	// Named pages: include filename stem to avoid collisions
	if dir == "." || dir == "" {
		return name
	}
	return strings.ReplaceAll(filepath.ToSlash(dir), "/", "-") + "-" + name
}

// runExtract walks the content directory, extracts annotated code blocks,
// and writes them to the output directory.
func runExtract(contentDir, outputDir string) error {
	tracked, err := gitTrackedFiles(contentDir)
	if err != nil {
		return err
	}
	return filepath.WalkDir(contentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolving %s: %w", path, err)
		}
		if !tracked[absPath] {
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
			Page:     relPath,
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
	tracked, err := gitTrackedFiles(contentDir)
	if err != nil {
		return err
	}

	var entries []coverageEntry

	err = filepath.WalkDir(contentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolving %s: %w", path, err)
		}
		if !tracked[absPath] {
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
				continue // non-testable or unknown language
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
		return fmt.Errorf("%d untested executable code block(s) found", len(entries))
	}

	fmt.Println("All executable code blocks are tested.")
	return nil
}
