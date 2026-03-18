// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "sync-config.yaml")
		os.WriteFile(path, []byte(`
defaults:
  branch: main
sources:
  - repo: org/repo1
    files:
      - src: README.md
        dest: content/docs/projects/repo1/_index.md
`), 0o644)

		cfg, err := loadConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Defaults.Branch != "main" {
			t.Errorf("branch = %q, want %q", cfg.Defaults.Branch, "main")
		}
		if len(cfg.Sources) != 1 {
			t.Fatalf("sources count = %d, want 1", len(cfg.Sources))
		}
		if cfg.Sources[0].Repo != "org/repo1" {
			t.Errorf("repo = %q, want %q", cfg.Sources[0].Repo, "org/repo1")
		}
		if cfg.Sources[0].Branch != "main" {
			t.Errorf("source branch = %q, want %q (inherited from defaults)", cfg.Sources[0].Branch, "main")
		}
	})

	t.Run("default branch applied", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "cfg.yaml")
		os.WriteFile(path, []byte(`
sources:
  - repo: org/repo1
    files:
      - src: README.md
        dest: out/README.md
`), 0o644)

		cfg, err := loadConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Defaults.Branch != "main" {
			t.Errorf("default branch = %q, want %q", cfg.Defaults.Branch, "main")
		}
	})

	t.Run("malformed YAML", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.yaml")
		os.WriteFile(path, []byte(`{{{not yaml`), 0o644)

		_, err := loadConfig(path)
		if err == nil {
			t.Fatal("expected error for malformed YAML")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := loadConfig("/nonexistent/path.yaml")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("missing repo field", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "cfg.yaml")
		os.WriteFile(path, []byte(`
sources:
  - files:
      - src: README.md
        dest: out/README.md
`), 0o644)

		_, err := loadConfig(path)
		if err == nil {
			t.Fatal("expected error for missing repo")
		}
		if !strings.Contains(err.Error(), "missing required field 'repo'") {
			t.Errorf("error = %q, want it to mention missing repo", err)
		}
	})

	t.Run("missing src field", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "cfg.yaml")
		os.WriteFile(path, []byte(`
sources:
  - repo: org/repo1
    files:
      - dest: out/README.md
`), 0o644)

		_, err := loadConfig(path)
		if err == nil {
			t.Fatal("expected error for missing src")
		}
		if !strings.Contains(err.Error(), "missing 'src'") {
			t.Errorf("error = %q, want it to mention missing src", err)
		}
	})
}
