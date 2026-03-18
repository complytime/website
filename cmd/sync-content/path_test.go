// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidRepoName(t *testing.T) {
	valid := []string{"my-repo", "repo123", "a", "repo.name", "dotdot..name"}
	for _, name := range valid {
		if !isValidRepoName(name) {
			t.Errorf("isValidRepoName(%q) = false, want true", name)
		}
	}

	invalid := []string{"", ".", "..", "path/sep", "back\\slash"}
	for _, name := range invalid {
		if isValidRepoName(name) {
			t.Errorf("isValidRepoName(%q) = true, want false", name)
		}
	}
}

func TestIsUnderDir(t *testing.T) {
	base := t.TempDir()

	tests := []struct {
		name   string
		target string
		want   bool
	}{
		{"child file", filepath.Join(base, "content", "file.md"), true},
		{"same dir", base, true},
		{"traversal", filepath.Join(base, "..", "etc", "passwd"), false},
		{"double traversal", filepath.Join(base, "content", "..", "..", "etc"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUnderDir(base, tt.target)
			if got != tt.want {
				t.Errorf("isUnderDir(%q, %q) = %v, want %v", base, tt.target, got, tt.want)
			}
		})
	}
}

func TestIsUnderDir_ResolvesBaseSymlinks(t *testing.T) {
	real := t.TempDir()
	parent := t.TempDir()
	link := filepath.Join(parent, "symlink-to-real")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	child := filepath.Join(link, "content", "file.md")
	if !isUnderDir(link, child) {
		t.Error("isUnderDir should allow child under symlinked base")
	}

	escape := filepath.Join(link, "..", "etc", "passwd")
	if isUnderDir(link, escape) {
		t.Error("isUnderDir should reject traversal out of symlinked base")
	}
}
