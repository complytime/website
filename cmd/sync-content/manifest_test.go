// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"content/docs/projects/complyctl/_index.md",
		"content/docs/projects/complyctl/quick-start.md",
	}

	if err := writeManifest(dir, files); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}

	got := readManifest(dir)
	if got == nil {
		t.Fatal("readManifest returned nil")
	}
	for _, f := range files {
		if !got[f] {
			t.Errorf("manifest missing %q", f)
		}
	}
	if len(got) != len(files) {
		t.Errorf("manifest has %d entries, want %d", len(got), len(files))
	}
}

func TestReadManifest_Missing(t *testing.T) {
	dir := t.TempDir()
	got := readManifest(dir)
	if got != nil {
		t.Errorf("readManifest for missing file should return nil, got %v", got)
	}
}

func TestReadFrontmatterParams(t *testing.T) {
	t.Run("reads params from generated frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "_index.md")
		os.WriteFile(path, []byte("---\ntitle: \"test\"\nparams:\n  source_sha: \"abc123\"\n  readme_sha: \"def456\"\n---\n"), 0o644)

		params := readFrontmatterParams(path)
		if params == nil {
			t.Fatal("params should not be nil")
		}
		if v, _ := params["source_sha"].(string); v != "abc123" {
			t.Errorf("source_sha = %q, want %q", v, "abc123")
		}
		if v, _ := params["readme_sha"].(string); v != "def456" {
			t.Errorf("readme_sha = %q, want %q", v, "def456")
		}
	})

	t.Run("does not match similarly-prefixed keys", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "_index.md")
		os.WriteFile(path, []byte("---\ntitle: \"test\"\nparams:\n  source_sha_v2: \"wrong\"\n  source_sha: \"correct\"\n---\n"), 0o644)

		params := readFrontmatterParams(path)
		if v, _ := params["source_sha"].(string); v != "correct" {
			t.Errorf("source_sha = %q, want %q", v, "correct")
		}
		if v, _ := params["source_sha_v2"].(string); v != "wrong" {
			t.Errorf("source_sha_v2 = %q, want %q (should be separate key)", v, "wrong")
		}
	})

	t.Run("missing file returns nil", func(t *testing.T) {
		params := readFrontmatterParams("/nonexistent/path.md")
		if params != nil {
			t.Errorf("expected nil for missing file, got %v", params)
		}
	})

	t.Run("no frontmatter returns nil", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "plain.md")
		os.WriteFile(path, []byte("# No frontmatter\nBody."), 0o644)

		params := readFrontmatterParams(path)
		if params != nil {
			t.Errorf("expected nil for file without frontmatter, got %v", params)
		}
	})

	t.Run("no params section returns nil", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "no-params.md")
		os.WriteFile(path, []byte("---\ntitle: test\n---\nBody."), 0o644)

		params := readFrontmatterParams(path)
		if params != nil {
			t.Errorf("expected nil for frontmatter without params, got %v", params)
		}
	})
}

func TestReadExistingState_UsesYAMLParsing(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "content", "docs", "projects", "test-repo")
	os.MkdirAll(repoDir, 0o755)
	os.WriteFile(filepath.Join(repoDir, "_index.md"), []byte(
		"---\ntitle: \"test-repo\"\nparams:\n  source_sha: \"branch-sha-123\"\n  readme_sha: \"readme-sha-456\"\n---\n",
	), 0o644)

	state := readExistingState(dir)
	if len(state) != 1 {
		t.Fatalf("state has %d entries, want 1", len(state))
	}
	s := state["test-repo"]
	if s.branchSHA != "branch-sha-123" {
		t.Errorf("branchSHA = %q, want %q", s.branchSHA, "branch-sha-123")
	}
	if s.readmeSHA != "readme-sha-456" {
		t.Errorf("readmeSHA = %q, want %q", s.readmeSHA, "readme-sha-456")
	}
}

func TestBuildDocPagesIndex(t *testing.T) {
	manifest := map[string]bool{
		"content/docs/projects/complyctl/_index.md":       true,
		"content/docs/projects/complyctl/overview.md":     true,
		"content/docs/projects/complyctl/installation.md": true,
		"content/docs/projects/complyscribe/_index.md":    true,
		"content/docs/projects/collector/_index.md":       true,
		"content/docs/projects/collector/docs/guide.md":   true,
		"data/projects.json":                              true,
	}

	index := buildDocPagesIndex(manifest)

	if !index["complyctl"] {
		t.Error("complyctl should be in index (has overview.md and installation.md)")
	}
	if index["complyscribe"] {
		t.Error("complyscribe should NOT be in index (only has _index.md)")
	}
	if !index["collector"] {
		t.Error("collector should be in index (has docs/guide.md)")
	}
}

func TestBuildDocPagesIndex_NilManifest(t *testing.T) {
	index := buildDocPagesIndex(nil)
	if len(index) != 0 {
		t.Errorf("nil manifest should produce empty index, got %d entries", len(index))
	}
}
