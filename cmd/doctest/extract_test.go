// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"os/exec"
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
		{"overview.md", "overview"},
		{"guides/quickstart.md", "guides-quickstart"},
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
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)
	contentDir := filepath.Join(repoDir, "content")
	outputDir := t.TempDir()

	md := "---\ntitle: Test\n---\n\n```bash {test=\"hello\"}\necho hello\n```\n"
	subDir := filepath.Join(contentDir, "getting-started")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "_index.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repoDir, "content/getting-started/_index.md")

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
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)
	contentDir := filepath.Join(repoDir, "content")
	outputDir := t.TempDir()

	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "```bash {test=\"dupe\"}\necho 1\n```\n\n```bash {test=\"dupe\"}\necho 2\n```\n"
	if err := os.WriteFile(filepath.Join(contentDir, "test.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repoDir, "content/test.md")

	err := runExtract(contentDir, outputDir)
	if err == nil {
		t.Fatal("expected error for duplicate test names, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate test name") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "duplicate test name")
	}
}

func TestRunExtractFrontmatterOptOut(t *testing.T) {
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)
	contentDir := filepath.Join(repoDir, "content")
	outputDir := t.TempDir()

	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\ntestable_docs: false\n---\n\n```bash {test=\"hello\"}\necho hello\n```\n"
	if err := os.WriteFile(filepath.Join(contentDir, "test.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repoDir, "content/test.md")

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
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)
	contentDir := filepath.Join(repoDir, "content")
	outputDir := t.TempDir()

	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "```bash {test=\"alpha\"}\necho a\n```\n\n```bash {test=\"beta\"}\necho b\n```\n\n```bash {test=\"gamma\"}\necho c\n```\n"
	if err := os.WriteFile(filepath.Join(contentDir, "test.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repoDir, "content/test.md")

	if err := runExtract(contentDir, outputDir); err != nil {
		t.Fatalf("runExtract error: %v", err)
	}

	expected := []string{"01-alpha.bash", "02-beta.bash", "03-gamma.bash"}
	pageDir := filepath.Join(outputDir, "test")
	for _, name := range expected {
		if _, err := os.Stat(filepath.Join(pageDir, name)); err != nil {
			t.Errorf("expected file %s not found: %v", name, err)
		}
	}
}

func TestRunCoverageReport(t *testing.T) {
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)
	contentDir := filepath.Join(repoDir, "content")

	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// One tested, one untested bash block, one yaml block (not testable)
	md := "```bash {test=\"tested\"}\necho tested\n```\n\n```bash\necho untested\n```\n\n```yaml\nkey: value\n```\n"
	if err := os.WriteFile(filepath.Join(contentDir, "test.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repoDir, "content/test.md")

	err := runCoverage(contentDir)
	if err == nil {
		t.Fatal("expected error for untested blocks, got nil")
	}
	if !strings.Contains(err.Error(), "1 untested") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRunCoverageOptOut(t *testing.T) {
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)
	contentDir := filepath.Join(repoDir, "content")

	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\ntestable_docs: false\n---\n\n```bash\necho untested\n```\n"
	if err := os.WriteFile(filepath.Join(contentDir, "test.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repoDir, "content/test.md")

	err := runCoverage(contentDir)
	if err != nil {
		t.Fatalf("runCoverage error: %v", err)
	}
}

// initGitRepo initialises a git repository in dir with isolated config
// so that user/system gitconfig cannot interfere with tests.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	env := append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// gitAdd stages and commits files in the repo at dir.
func gitAdd(t *testing.T, dir string, files ...string) {
	t.Helper()
	env := append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	args := append([]string{"add"}, files...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "test")
	cmd.Dir = dir
	cmd.Env = env
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

func TestGitTrackedFiles(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create tracked file
	tracked := filepath.Join(dir, "docs", "guide.md")
	if err := os.MkdirAll(filepath.Dir(tracked), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tracked, []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "docs/guide.md")

	// Create gitignored file
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ignoredDir := filepath.Join(dir, "ignored")
	if err := os.MkdirAll(ignoredDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ignoredDir, "secret.md"), []byte("# Secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, ".gitignore")

	// Create untracked-but-not-ignored file (should be included)
	untracked := filepath.Join(dir, "docs", "new.md")
	if err := os.WriteFile(untracked, []byte("# New"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := gitTrackedFiles(dir)
	if err != nil {
		t.Fatalf("gitTrackedFiles: %v", err)
	}

	if !files[filepath.Join(dir, "docs", "guide.md")] {
		t.Error("tracked file docs/guide.md should be in set")
	}
	if !files[filepath.Join(dir, "docs", "new.md")] {
		t.Error("untracked-but-not-ignored file docs/new.md should be in set")
	}
	if files[filepath.Join(dir, "ignored", "secret.md")] {
		t.Error("gitignored file ignored/secret.md should NOT be in set")
	}
}

func TestRunExtractSkipsGitignored(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	contentDir := filepath.Join(dir, "content")
	outputDir := t.TempDir()

	// Create tracked file with annotated block
	trackedDir := filepath.Join(contentDir, "guide")
	if err := os.MkdirAll(trackedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "```bash {test=\"tracked\"}\necho tracked\n```\n"
	if err := os.WriteFile(filepath.Join(trackedDir, "_index.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create gitignored file with annotated block
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("content/generated/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ignoredDir := filepath.Join(contentDir, "generated")
	if err := os.MkdirAll(ignoredDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ignoredMd := "```bash {test=\"ignored\"}\necho ignored\n```\n"
	if err := os.WriteFile(filepath.Join(ignoredDir, "_index.md"), []byte(ignoredMd), 0o644); err != nil {
		t.Fatal(err)
	}

	gitAdd(t, dir, "content/guide/_index.md", ".gitignore")

	if err := runExtract(contentDir, outputDir); err != nil {
		t.Fatalf("runExtract error: %v", err)
	}

	// Tracked file should produce output
	if _, err := os.Stat(filepath.Join(outputDir, "guide", "01-tracked.bash")); err != nil {
		t.Errorf("expected output for tracked file: %v", err)
	}

	// Gitignored file should NOT produce output
	if _, err := os.Stat(filepath.Join(outputDir, "generated", "01-ignored.bash")); err == nil {
		t.Error("gitignored file should not produce output, but it did")
	}
}

func TestRunCoverageSkipsGitignored(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	contentDir := filepath.Join(dir, "content")

	// Create gitignored file with untested block
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("content/generated/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ignoredDir := filepath.Join(contentDir, "generated")
	if err := os.MkdirAll(ignoredDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "```bash\necho untested\n```\n"
	if err := os.WriteFile(filepath.Join(ignoredDir, "_index.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	gitAdd(t, dir, ".gitignore")

	// Coverage should pass since the only untested block is gitignored
	err := runCoverage(contentDir)
	if err != nil {
		t.Fatalf("runCoverage should pass (gitignored file), got: %v", err)
	}
}
