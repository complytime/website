// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initRepoWithBlock creates an isolated git repo containing a content dir with
// a single tracked Markdown file and returns the content directory path.
func initRepoWithBlock(t *testing.T, md string) string {
	t.Helper()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)
	contentDir := filepath.Join(repoDir, "content")
	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "test.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repoDir, "content/test.md")
	return contentDir
}

func TestRunExitCodes(t *testing.T) {
	// Content with one untested executable block so coverage fails.
	untestedContent := initRepoWithBlock(t, "```bash\necho hi\n```\n")
	// Content fully annotated so coverage passes.
	testedContent := initRepoWithBlock(t, "```bash {test=\"ok\"}\necho hi\n```\n")

	outputDir := t.TempDir()

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"no args", nil, 1},
		{"unknown subcommand", []string{"bogus"}, 1},
		{"extract missing flags", []string{"extract"}, 1},
		{"extract unknown flag", []string{"extract", "--nope"}, 1},
		{"coverage missing flag", []string{"coverage"}, 1},
		{"extract success", []string{"extract", "--content-dir", testedContent, "--output-dir", outputDir}, 0},
		{"coverage failure", []string{"coverage", "--content-dir", untestedContent}, 1},
		{"coverage success", []string{"coverage", "--content-dir", testedContent}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(tt.args); got != tt.want {
				t.Errorf("run(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestRunExtractError(t *testing.T) {
	// content-dir outside any git repo makes gitTrackedFiles fail.
	nonRepo := t.TempDir()
	env := append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	// Ensure the temp dir is not inside a repo by checking git status fails.
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = nonRepo
	cmd.Env = env
	if err := cmd.Run(); err == nil {
		t.Skip("temp dir unexpectedly inside a git repo; skipping")
	}

	if got := run([]string{"extract", "--content-dir", nonRepo, "--output-dir", t.TempDir()}); got != 1 {
		t.Errorf("run extract on non-repo = %d, want 1", got)
	}
}
