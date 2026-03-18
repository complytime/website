// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadWriteLock_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".content-lock.json")

	original := &ContentLock{Repos: map[string]string{
		"complyctl":     "abc123",
		"comply-scribe": "def456",
	}}
	if err := writeLock(path, original); err != nil {
		t.Fatalf("writeLock: %v", err)
	}

	loaded, err := readLock(path)
	if err != nil {
		t.Fatalf("readLock: %v", err)
	}

	if len(loaded.Repos) != 2 {
		t.Fatalf("repos count = %d, want 2", len(loaded.Repos))
	}
	if loaded.Repos["complyctl"] != "abc123" {
		t.Errorf("complyctl SHA = %q, want %q", loaded.Repos["complyctl"], "abc123")
	}
	if loaded.Repos["comply-scribe"] != "def456" {
		t.Errorf("comply-scribe SHA = %q, want %q", loaded.Repos["comply-scribe"], "def456")
	}
}

func TestReadLock_MissingFile(t *testing.T) {
	lock, err := readLock(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("readLock should not error for missing file: %v", err)
	}
	if lock == nil || lock.Repos == nil {
		t.Fatal("readLock should return empty lock with initialized map")
	}
	if len(lock.Repos) != 0 {
		t.Errorf("repos count = %d, want 0", len(lock.Repos))
	}
}

func TestReadLock_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0o644)

	_, err := readLock(path)
	if err == nil {
		t.Fatal("readLock should error on invalid JSON")
	}
}

func TestContentLock_SHA(t *testing.T) {
	lock := &ContentLock{Repos: map[string]string{
		"complyctl": "abc123",
	}}

	if got := lock.sha("complyctl"); got != "abc123" {
		t.Errorf("sha(complyctl) = %q, want %q", got, "abc123")
	}
	if got := lock.sha("unknown"); got != "" {
		t.Errorf("sha(unknown) = %q, want empty", got)
	}

	var nilLock *ContentLock
	if got := nilLock.sha("anything"); got != "" {
		t.Errorf("nil lock sha() = %q, want empty", got)
	}
}

func TestWriteLock_DeterministicOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lock.json")

	lock := &ContentLock{Repos: map[string]string{
		"zebra": "z1",
		"alpha": "a1",
		"mike":  "m1",
	}}

	if err := writeLock(path, lock); err != nil {
		t.Fatalf("writeLock: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	alphaIdx := indexOf(content, "alpha")
	mikeIdx := indexOf(content, "mike")
	zebraIdx := indexOf(content, "zebra")

	if alphaIdx > mikeIdx || mikeIdx > zebraIdx {
		t.Errorf("keys should be sorted alphabetically, got:\n%s", content)
	}
}

func TestReadLock_NilReposInitialized(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lock.json")
	os.WriteFile(path, []byte(`{}`), 0o644)

	lock, err := readLock(path)
	if err != nil {
		t.Fatalf("readLock: %v", err)
	}
	if lock.Repos == nil {
		t.Error("Repos should be initialized even when missing from JSON")
	}
}

func indexOf(s, substr string) int {
	for i := range len(s) - len(substr) + 1 {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
