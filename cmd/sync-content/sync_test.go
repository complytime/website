// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestProcessRepo(t *testing.T) {
	readmeContent := "# test-repo\n\nThis is a test README."
	branchSHA := "abc123def456"

	mux := http.NewServeMux()

	mux.HandleFunc("/repos/testorg/test-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64(readmeContent),
			Encoding: "base64",
			SHA:      "sha-readme",
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/branches/main", func(w http.ResponseWriter, r *http.Request) {
		resp := BranchResponse{}
		resp.Commit.SHA = branchSHA
		_ = json.NewEncoder(w).Encode(resp)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
	}

	result := &syncResult{}
	oldState := map[string]repoState{}

	work := processRepo(ctx, gh, "testorg", output, repo, true, false, result, oldState, nil, "")

	if work == nil {
		t.Fatal("processRepo returned nil")
	}
	if work.card.Name != "test-repo" {
		t.Errorf("card.Name = %q, want %q", work.card.Name, "test-repo")
	}
	if work.card.Language != "Go" {
		t.Errorf("card.Language = %q, want %q", work.card.Language, "Go")
	}
	if work.card.Description != "A test repository" {
		t.Errorf("card.Description = %q, want %q", work.card.Description, "A test repository")
	}

	indexPath := filepath.Join(output, "content", "docs", "projects", "test-repo", "_index.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("section index not written: %v", err)
	}
	index := string(data)
	if !strings.Contains(index, `title: "Test Repo"`) {
		t.Error("section index title should use formatRepoTitle")
	}
	if !strings.Contains(index, `linkTitle: "test-repo"`) {
		t.Error("section index should have linkTitle with raw repo name")
	}
	if !strings.Contains(index, "readme_sha:") {
		t.Error("section index should contain readme_sha in frontmatter")
	}
	if !strings.Contains(index, "sha-readme") {
		t.Error("section index should contain the README blob SHA value")
	}
	if strings.Contains(index, "This is a test README.") {
		t.Error("section index should be frontmatter-only, no README body")
	}

	overviewPath := filepath.Join(output, "content", "docs", "projects", "test-repo", "overview.md")
	overviewData, err := os.ReadFile(overviewPath)
	if err != nil {
		t.Fatalf("overview page not written: %v", err)
	}
	overview := string(overviewData)
	if !strings.Contains(overview, "This is a test README.") {
		t.Error("overview page should contain README body")
	}
	if strings.Contains(overview, "# test-repo") || strings.Contains(overview, "## Test-repo") {
		t.Error("leading H1 should be stripped — title is already in frontmatter")
	}
	if !strings.Contains(overview, `title: "Overview"`) {
		t.Error("overview page should have title 'Overview'")
	}
	if work.unchanged {
		t.Error("unchanged should be false for new repos")
	}
}

func TestProcessRepo_BranchUnchanged(t *testing.T) {
	branchSHA := "abc123def456"
	readmeCalls := 0

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/testorg/test-repo/branches/main", func(w http.ResponseWriter, r *http.Request) {
		resp := BranchResponse{}
		resp.Commit.SHA = branchSHA
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/repos/testorg/test-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		readmeCalls++
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# test-repo\n\nContent"),
			Encoding: "base64",
			SHA:      "sha-readme",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
	}

	oldState := map[string]repoState{
		"test-repo": {branchSHA: branchSHA, readmeSHA: "sha-readme"},
	}
	oldManifest := map[string]bool{
		"content/docs/projects/test-repo/_index.md": true,
	}

	result := &syncResult{}
	work := processRepo(ctx, gh, "testorg", output, repo, true, false, result, oldState, oldManifest, "")

	if work == nil {
		t.Fatal("processRepo returned nil for unchanged repo in write mode")
	}
	if work.card.Name != "test-repo" {
		t.Errorf("card.Name = %q, want %q", work.card.Name, "test-repo")
	}
	if readmeCalls != 0 {
		t.Errorf("README was fetched %d times, want 0 (fast path should skip)", readmeCalls)
	}
	if !work.unchanged {
		t.Error("unchanged should be true when branch SHA matches")
	}
	if len(result.unchanged) != 1 || result.unchanged[0] != "test-repo" {
		t.Errorf("unchanged = %v, want [test-repo]", result.unchanged)
	}
	if len(result.writtenFiles) != 1 {
		t.Errorf("writtenFiles = %d, want 1 (carried forward from manifest)", len(result.writtenFiles))
	}
}

func TestProcessRepo_BranchChangedReadmeUnchanged(t *testing.T) {
	readmeContent := "# test-repo\n\nThis is a test README."
	readmeSHA := "sha-readme-stable"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/testorg/test-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64(readmeContent),
			Encoding: "base64",
			SHA:      readmeSHA,
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/branches/main", func(w http.ResponseWriter, r *http.Request) {
		resp := BranchResponse{}
		resp.Commit.SHA = "new-branch-sha"
		_ = json.NewEncoder(w).Encode(resp)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
	}

	oldState := map[string]repoState{
		"test-repo": {branchSHA: "old-branch-sha", readmeSHA: readmeSHA},
	}

	result := &syncResult{}
	work := processRepo(ctx, gh, "testorg", output, repo, true, false, result, oldState, nil, "")

	if work == nil {
		t.Fatal("processRepo returned nil")
	}
	if len(result.unchanged) != 1 || result.unchanged[0] != "test-repo" {
		t.Errorf("repo should be classified as unchanged when README SHA matches, got unchanged=%v updated=%v", result.unchanged, result.updated)
	}
}

func TestSyncConfigSource(t *testing.T) {
	fileContent := "[![badge](https://img.svg)](https://ci)\n\n# complyctl\n\nSome [link](docs/guide.md) here."

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/complyctl/contents/README.md", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64(fileContent),
			Encoding: "base64",
			SHA:      "sha-file",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	src := Source{
		Repo:   "org/complyctl",
		Branch: "main",
		Files: []FileSpec{
			{
				Src:  "README.md",
				Dest: "content/docs/projects/complyctl/_index.md",
				Transform: Transform{
					InjectFrontmatter: map[string]any{
						"title":       "complyctl",
						"description": "CLI tool",
						"weight":      10,
					},
					RewriteLinks: true,
					StripBadges:  true,
				},
			},
		},
	}

	t.Run("write mode applies transforms", func(t *testing.T) {
		result := &syncResult{}
		syncConfigSource(ctx, gh, src, Defaults{Branch: "main"}, output, true, result, "")

		if result.errors > 0 {
			t.Fatalf("syncConfigSource had %d errors", result.errors)
		}
		if result.filesProcessed != 1 {
			t.Errorf("filesProcessed = %d, want 1", result.filesProcessed)
		}

		destPath := filepath.Join(output, "content", "docs", "projects", "complyctl", "_index.md")
		data, err := os.ReadFile(destPath)
		if err != nil {
			t.Fatalf("config file not written: %v", err)
		}
		content := string(data)

		if !strings.Contains(content, "title: complyctl") {
			t.Error("injected frontmatter should contain title")
		}
		if strings.Contains(content, "[![badge") {
			t.Error("badges should be stripped")
		}
		if strings.Contains(content, "](docs/guide.md)") {
			t.Error("relative links should be rewritten")
		}
		if !strings.Contains(content, "https://github.com/org/complyctl/blob/main/docs/guide.md") {
			t.Error("relative link should become absolute GitHub URL")
		}
		if strings.Contains(content, "# complyctl") || strings.Contains(content, "## Complyctl") {
			t.Error("leading H1 should be stripped — title is already in frontmatter")
		}
	})

	t.Run("dry-run writes nothing", func(t *testing.T) {
		dryOutput := t.TempDir()
		result := &syncResult{}
		syncConfigSource(ctx, gh, src, Defaults{Branch: "main"}, dryOutput, false, result, "")

		if result.filesProcessed != 1 {
			t.Errorf("dry-run filesProcessed = %d, want 1", result.filesProcessed)
		}

		destPath := filepath.Join(dryOutput, "content", "docs", "projects", "complyctl", "_index.md")
		if _, err := os.Stat(destPath); !os.IsNotExist(err) {
			t.Error("dry-run should not create files")
		}
	})
}

func TestConcurrentSyncResult(t *testing.T) {
	result := &syncResult{}
	var wg sync.WaitGroup

	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result.addSynced()
		}()
	}

	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result.addError()
		}()
	}

	for range 25 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result.addWarning()
		}()
	}

	wg.Wait()

	if result.synced != 100 {
		t.Errorf("synced = %d, want 100", result.synced)
	}
	if result.errors != 50 {
		t.Errorf("errors = %d, want 50", result.errors)
	}
	if result.warnings != 25 {
		t.Errorf("warnings = %d, want 25", result.warnings)
	}
}

func TestRecordFile(t *testing.T) {
	result := &syncResult{}
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			result.recordFile(fmt.Sprintf("file-%d.md", n))
		}(i)
	}
	wg.Wait()
	if len(result.writtenFiles) != 50 {
		t.Errorf("writtenFiles = %d, want 50", len(result.writtenFiles))
	}
}

func TestSyncConfigSourceProvenance(t *testing.T) {
	fileContent := "# complyctl\n\nSome content."
	fileSHA := "abc123def456789"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/complyctl/contents/README.md", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64(fileContent),
			Encoding: "base64",
			SHA:      fileSHA,
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	src := Source{
		Repo:   "org/complyctl",
		Branch: "main",
		Files: []FileSpec{
			{
				Src:  "README.md",
				Dest: "content/docs/projects/complyctl/_index.md",
				Transform: Transform{
					InjectFrontmatter: map[string]any{"title": "complyctl"},
				},
			},
		},
	}

	result := &syncResult{}
	syncConfigSource(ctx, gh, src, Defaults{Branch: "main"}, output, true, result, "")

	destPath := filepath.Join(output, "content", "docs", "projects", "complyctl", "_index.md")
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "<!-- synced from org/complyctl/README.md@main (abc123def456) -->") {
		t.Errorf("provenance comment missing or incorrect, got:\n%s", content)
	}
}

func TestSyncRepoDocPages(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/testorg/test-repo/contents/docs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]DirEntry{
			{Name: "installation.md", Path: "docs/installation.md", Type: "file"},
			{Name: "usage.md", Path: "docs/usage.md", Type: "file"},
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/installation.md", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# Installation\n\nRun `go install`."),
			Encoding: "base64",
			SHA:      "sha-install",
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/usage.md", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# Usage\n\nRun the CLI tool."),
			Encoding: "base64",
			SHA:      "sha-usage",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
		PushedAt:      "2025-01-15T00:00:00Z",
	}

	discovery := Discovery{ScanPaths: []string{"docs"}}
	result := &syncResult{}
	syncRepoDocPages(ctx, gh, "testorg", repo, output, true, discovery, nil, nil, result, "")

	if result.errors != 0 {
		t.Fatalf("errors = %d, want 0", result.errors)
	}
	if result.filesProcessed != 2 {
		t.Errorf("filesProcessed = %d, want 2", result.filesProcessed)
	}

	cases := []struct {
		relPath string
		title   string
		provSrc string
	}{
		{
			relPath: "content/docs/projects/test-repo/installation.md",
			title:   "Installation",
			provSrc: "testorg/test-repo/docs/installation.md@main",
		},
		{
			relPath: "content/docs/projects/test-repo/usage.md",
			title:   "Usage",
			provSrc: "testorg/test-repo/docs/usage.md@main",
		},
	}

	for _, tc := range cases {
		fullPath := filepath.Join(output, tc.relPath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("file not written: %s: %v", tc.relPath, err)
		}
		content := string(data)

		if !strings.Contains(content, fmt.Sprintf("title: %q", tc.title)) {
			t.Errorf("%s: missing title %q in frontmatter:\n%s", tc.relPath, tc.title, content)
		}
		if !strings.Contains(content, "draft: false") {
			t.Errorf("%s: missing draft: false", tc.relPath)
		}
		if !strings.Contains(content, "weight: 10") {
			t.Errorf("%s: missing weight: 10", tc.relPath)
		}
		if !strings.Contains(content, "date: 2025-01-15T00:00:00Z") {
			t.Errorf("%s: missing or wrong date", tc.relPath)
		}
		if !strings.Contains(content, "<!-- synced from "+tc.provSrc) {
			t.Errorf("%s: missing provenance comment for %s:\n%s", tc.relPath, tc.provSrc, content)
		}
		if strings.Contains(content, "# "+tc.title) {
			t.Errorf("%s: leading H1 should be stripped — title is already in frontmatter", tc.relPath)
		}
	}

	if len(result.writtenFiles) != 2 {
		t.Errorf("writtenFiles = %d, want 2", len(result.writtenFiles))
	}
}

func TestSyncRepoDocPages_SkipsConfigTracked(t *testing.T) {
	fetchedFiles := make(map[string]bool)

	mux := http.NewServeMux()

	mux.HandleFunc("/repos/testorg/test-repo/contents/docs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]DirEntry{
			{Name: "installation.md", Path: "docs/installation.md", Type: "file"},
			{Name: "usage.md", Path: "docs/usage.md", Type: "file"},
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/installation.md", func(w http.ResponseWriter, r *http.Request) {
		fetchedFiles["docs/installation.md"] = true
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# Installation\n\nSteps."),
			Encoding: "base64",
			SHA:      "sha-install",
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/usage.md", func(w http.ResponseWriter, r *http.Request) {
		fetchedFiles["docs/usage.md"] = true
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# Usage\n\nRun it."),
			Encoding: "base64",
			SHA:      "sha-usage",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
		PushedAt:      "2025-01-15T00:00:00Z",
	}

	discovery := Discovery{ScanPaths: []string{"docs"}}
	configTracked := map[string]bool{"docs/installation.md": true}

	result := &syncResult{}
	syncRepoDocPages(ctx, gh, "testorg", repo, output, true, discovery, nil, configTracked, result, "")

	if fetchedFiles["docs/installation.md"] {
		t.Error("config-tracked file docs/installation.md should not have been fetched")
	}
	if !fetchedFiles["docs/usage.md"] {
		t.Error("non-tracked file docs/usage.md should have been fetched")
	}

	trackedPath := filepath.Join(output, "content", "docs", "projects", "test-repo", "installation.md")
	if _, err := os.Stat(trackedPath); !os.IsNotExist(err) {
		t.Error("config-tracked file should not have been written")
	}

	untrackedPath := filepath.Join(output, "content", "docs", "projects", "test-repo", "usage.md")
	if _, err := os.Stat(untrackedPath); err != nil {
		t.Fatalf("non-tracked file should have been written: %v", err)
	}

	if result.filesProcessed != 1 {
		t.Errorf("filesProcessed = %d, want 1 (only the non-tracked file)", result.filesProcessed)
	}
}

func TestPathTraversalRejection(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/complyctl/contents/README.md", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# README\nContent here."),
			Encoding: "base64",
			SHA:      "sha-traversal",
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()

	src := Source{
		Repo:   "org/complyctl",
		Branch: "main",
		Files: []FileSpec{
			{
				Src:  "README.md",
				Dest: "../../etc/cron.d/backdoor.md",
			},
		},
	}

	result := &syncResult{}
	syncConfigSource(context.Background(), gh, src, Defaults{Branch: "main"}, output, true, result, "")

	if result.errors != 1 {
		t.Errorf("expected 1 error for path traversal, got %d", result.errors)
	}

	escapedPath := filepath.Join(output, "../../etc/cron.d/backdoor.md")
	if _, err := os.Stat(escapedPath); !os.IsNotExist(err) {
		t.Error("traversal file should not have been written")
	}
}

func TestDryRunReturnsCard(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/testorg/test-repo/branches/main", func(w http.ResponseWriter, r *http.Request) {
		resp := BranchResponse{}
		resp.Commit.SHA = "new-sha"
		_ = json.NewEncoder(w).Encode(resp)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "Test",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
	}

	result := &syncResult{}
	work := processRepo(context.Background(), gh, "testorg", output, repo, false, false, result, map[string]repoState{}, nil, "")

	if work == nil {
		t.Fatal("dry-run processRepo should return non-nil repoWork")
	}
	if work.card.Name != "test-repo" {
		t.Errorf("card.Name = %q, want %q", work.card.Name, "test-repo")
	}
}

func TestProcessRepo_NilOmitsCard(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/testorg/test-repo/branches/main", func(w http.ResponseWriter, r *http.Request) {
		resp := BranchResponse{}
		resp.Commit.SHA = "new-sha"
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/repos/testorg/test-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# test-repo\n\nContent"),
			Encoding: "base64",
			SHA:      "sha-readme",
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()

	repoDir := filepath.Join(output, "content", "docs", "projects", "test-repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Chmod(repoDir, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	defer func() { _ = os.Chmod(repoDir, 0o755) }()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
	}

	var result syncResult
	var cards []ProjectCard

	work := processRepo(context.Background(), gh, "testorg", output, repo, true, false, &result, map[string]repoState{}, nil, "")
	if work != nil {
		cards = append(cards, work.card)
	}

	if len(cards) != 0 {
		t.Fatalf("cards = %d, want 0 (failed repo must not produce a landing page card)", len(cards))
	}
}

func TestProcessRepo_LockedSHA(t *testing.T) {
	lockedSHA := "locked-sha-999"
	upstreamSHA := "upstream-sha-new"

	var readmeRefReceived string

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/testorg/test-repo/branches/main", func(w http.ResponseWriter, r *http.Request) {
		resp := BranchResponse{}
		resp.Commit.SHA = upstreamSHA
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/repos/testorg/test-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		readmeRefReceived = r.URL.Query().Get("ref")
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# test-repo\n\nLocked content."),
			Encoding: "base64",
			SHA:      "sha-readme-locked",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
	}

	result := &syncResult{}
	work := processRepo(ctx, gh, "testorg", output, repo, true, false, result, map[string]repoState{}, nil, lockedSHA)

	if work == nil {
		t.Fatal("processRepo returned nil")
	}
	if work.sha != upstreamSHA {
		t.Errorf("work.sha = %q, want upstream SHA %q", work.sha, upstreamSHA)
	}
	if readmeRefReceived != lockedSHA {
		t.Errorf("README fetched with ref=%q, want locked SHA %q", readmeRefReceived, lockedSHA)
	}
}

func TestProcessRepo_LockedSHA_MatchesUpstream(t *testing.T) {
	sameSHA := "same-sha-abc"

	var readmeRefReceived string

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/testorg/test-repo/branches/main", func(w http.ResponseWriter, r *http.Request) {
		resp := BranchResponse{}
		resp.Commit.SHA = sameSHA
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/repos/testorg/test-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		readmeRefReceived = r.URL.Query().Get("ref")
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# test-repo\n\nContent."),
			Encoding: "base64",
			SHA:      "sha-readme",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "Test",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
	}

	result := &syncResult{}
	processRepo(context.Background(), gh, "testorg", output, repo, true, false, result, map[string]repoState{}, nil, sameSHA)

	if readmeRefReceived != "" {
		t.Errorf("when locked SHA matches upstream, ref should be empty (fetch HEAD), got %q", readmeRefReceived)
	}
}

func TestSyncRepoDocPages_SkipsIndexMD(t *testing.T) {
	fetchedFiles := make(map[string]bool)

	mux := http.NewServeMux()

	mux.HandleFunc("/repos/testorg/test-repo/contents/docs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]DirEntry{
			{Name: "index.md", Path: "docs/index.md", Type: "file"},
			{Name: "usage.md", Path: "docs/usage.md", Type: "file"},
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/index.md", func(w http.ResponseWriter, r *http.Request) {
		fetchedFiles["docs/index.md"] = true
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# Index\n\nThis is a mkdocs index."),
			Encoding: "base64",
			SHA:      "sha-index",
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/usage.md", func(w http.ResponseWriter, r *http.Request) {
		fetchedFiles["docs/usage.md"] = true
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# Usage\n\nRun it."),
			Encoding: "base64",
			SHA:      "sha-usage",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
		PushedAt:      "2025-01-15T00:00:00Z",
	}

	discovery := Discovery{ScanPaths: []string{"docs"}}
	result := &syncResult{}
	syncRepoDocPages(ctx, gh, "testorg", repo, output, true, discovery, nil, nil, result, "")

	if fetchedFiles["docs/index.md"] {
		t.Error("index.md should not have been fetched (conflicts with Hugo _index.md)")
	}
	if !fetchedFiles["docs/usage.md"] {
		t.Error("usage.md should have been fetched")
	}

	indexPath := filepath.Join(output, "content", "docs", "projects", "test-repo", "index.md")
	if _, err := os.Stat(indexPath); !os.IsNotExist(err) {
		t.Error("index.md should not have been written (would create Hugo leaf bundle conflict)")
	}

	usagePath := filepath.Join(output, "content", "docs", "projects", "test-repo", "usage.md")
	if _, err := os.Stat(usagePath); err != nil {
		t.Fatalf("usage.md should have been written: %v", err)
	}

	if result.filesProcessed != 1 {
		t.Errorf("filesProcessed = %d, want 1 (only usage.md)", result.filesProcessed)
	}
}

func TestToMarkdown_NewRepos(t *testing.T) {
	result := &syncResult{
		synced:  4,
		skipped: 2,
		added:   []string{"complyctl", "complyscribe"},
		repoDetails: map[string]repoSummary{
			"complyctl": {
				description: "CLI for compliance",
				newSHA:      "9515e3dafb25a8f4b17e61ebcc79d4a1668cd6af",
				htmlURL:     "https://github.com/complytime/complyctl",
			},
			"complyscribe": {
				description: "Compliance authoring tool",
				newSHA:      "e04417ca36b2c8a27ddd415276dbc280b22551da",
				htmlURL:     "https://github.com/complytime/complyscribe",
			},
		},
	}

	md := result.toMarkdown()

	checks := []struct {
		name     string
		contains string
	}{
		{"section header", "### New Repositories"},
		{"linked repo name", "[`complyctl`](https://github.com/complytime/complyctl)"},
		{"description", "CLI for compliance"},
		{"short SHA", "9515e3da"},
		{"commit link", "https://github.com/complytime/complyctl/commit/9515e3dafb25a8f4b17e61ebcc79d4a1668cd6af"},
		{"stats", "**Repositories**: 4 synced, 2 skipped"},
	}
	for _, tc := range checks {
		if !strings.Contains(md, tc.contains) {
			t.Errorf("%s: expected markdown to contain %q\n\nGot:\n%s", tc.name, tc.contains, md)
		}
	}
}

func TestToMarkdown_UpdatedWithCompareLink(t *testing.T) {
	result := &syncResult{
		synced:  2,
		skipped: 0,
		updated: []string{"complyctl"},
		repoDetails: map[string]repoSummary{
			"complyctl": {
				description: "CLI tool",
				newSHA:      "ef01ab2345678900000000000000000000000000",
				oldSHA:      "9515e3dafb25a8f4b17e61ebcc79d4a1668cd6af",
				htmlURL:     "https://github.com/complytime/complyctl",
			},
		},
	}

	md := result.toMarkdown()

	checks := []struct {
		name     string
		contains string
	}{
		{"section header", "### Updated"},
		{"linked repo name", "[`complyctl`](https://github.com/complytime/complyctl)"},
		{"old short SHA", "9515e3da"},
		{"new short SHA", "ef01ab23"},
		{"compare link", "https://github.com/complytime/complyctl/compare/9515e3dafb25a8f4b17e61ebcc79d4a1668cd6af...ef01ab2345678900000000000000000000000000"},
	}
	for _, tc := range checks {
		if !strings.Contains(md, tc.contains) {
			t.Errorf("%s: expected markdown to contain %q\n\nGot:\n%s", tc.name, tc.contains, md)
		}
	}
}

func TestToMarkdown_UnchangedCollapsible(t *testing.T) {
	result := &syncResult{
		synced:    5,
		skipped:   0,
		added:     []string{"new-repo"},
		unchanged: []string{"stable-b", "stable-a"},
		repoDetails: map[string]repoSummary{
			"new-repo": {
				description: "New tool",
				newSHA:      "aaaa",
				htmlURL:     "https://github.com/org/new-repo",
			},
		},
	}

	md := result.toMarkdown()

	if !strings.Contains(md, "<details>") {
		t.Errorf("expected collapsible section for unchanged repos\n\nGot:\n%s", md)
	}
	if !strings.Contains(md, "Unchanged (2 repositories)") {
		t.Errorf("expected unchanged count in summary\n\nGot:\n%s", md)
	}
	// Unchanged repos should be sorted
	idxA := strings.Index(md, "stable-a")
	idxB := strings.Index(md, "stable-b")
	if idxA > idxB {
		t.Errorf("unchanged repos should be sorted alphabetically, got stable-b before stable-a")
	}
}

func TestToMarkdown_NoChanges(t *testing.T) {
	result := &syncResult{
		synced:  3,
		skipped: 1,
	}

	md := result.toMarkdown()

	if !strings.Contains(md, "No changes detected.") {
		t.Errorf("expected 'No changes detected' when no changes\n\nGot:\n%s", md)
	}
}

func TestToMarkdown_FallbackWithoutDetails(t *testing.T) {
	result := &syncResult{
		synced: 1,
		added:  []string{"bare-repo"},
	}

	md := result.toMarkdown()

	if !strings.Contains(md, "- `bare-repo`") {
		t.Errorf("expected fallback plain name when no repoDetails\n\nGot:\n%s", md)
	}
}

func TestToMarkdown_FilesProcessedLine(t *testing.T) {
	result := &syncResult{
		synced:         4,
		skipped:        2,
		filesProcessed: 33,
		added:          []string{"complyctl"},
		repoDetails: map[string]repoSummary{
			"complyctl": {htmlURL: "https://github.com/org/complyctl", newSHA: "abc"},
		},
	}

	md := result.toMarkdown()

	if !strings.Contains(md, "**Repositories**: 4 synced, 2 skipped") {
		t.Errorf("expected repo stats line\n\nGot:\n%s", md)
	}
	if !strings.Contains(md, "**Files processed**: 33") {
		t.Errorf("expected files processed line\n\nGot:\n%s", md)
	}
}

func TestToMarkdown_NoFilesProcessedWhenZero(t *testing.T) {
	result := &syncResult{
		synced:  2,
		skipped: 1,
		added:   []string{"repo-a"},
	}

	md := result.toMarkdown()

	if strings.Contains(md, "Files processed") {
		t.Errorf("should not show files processed line when count is 0\n\nGot:\n%s", md)
	}
}

func TestToMarkdown_FileManifest(t *testing.T) {
	result := &syncResult{
		synced:         2,
		filesProcessed: 5,
		added:          []string{"complyctl", "complyscribe"},
		repoDetails: map[string]repoSummary{
			"complyctl":    {htmlURL: "https://github.com/org/complyctl", newSHA: "aaa"},
			"complyscribe": {htmlURL: "https://github.com/org/complyscribe", newSHA: "bbb"},
		},
		repoFiles: map[string][]string{
			"complyctl":    {"docs/usage.md", "docs/api.md", "docs/installation.md"},
			"complyscribe": {"docs/guide.md", "README.md"},
		},
		changedRepoFiles: map[string][]string{
			"complyctl":    {"docs/usage.md", "docs/api.md", "docs/installation.md"},
			"complyscribe": {"docs/guide.md", "README.md"},
		},
	}

	md := result.toMarkdown()

	checks := []struct {
		name     string
		contains string
	}{
		{"collapsible", "<details>"},
		{"summary count", "Changed files (5 across 2 repositories)"},
		{"complyctl header", "**complyctl** (3 files)"},
		{"complyscribe header", "**complyscribe** (2 files)"},
		{"file entry", "`docs/installation.md`"},
		{"file entry", "`docs/guide.md`"},
	}
	for _, tc := range checks {
		if !strings.Contains(md, tc.contains) {
			t.Errorf("%s: expected markdown to contain %q\n\nGot:\n%s", tc.name, tc.contains, md)
		}
	}

	// Files within a repo should be sorted
	idxAPI := strings.Index(md, "docs/api.md")
	idxInstall := strings.Index(md, "docs/installation.md")
	idxUsage := strings.Index(md, "docs/usage.md")
	if idxAPI >= idxInstall || idxInstall >= idxUsage {
		t.Errorf("files should be sorted alphabetically within a repo\n\nGot:\n%s", md)
	}

	// Repos should be sorted
	idxCtl := strings.Index(md, "**complyctl**")
	idxScribe := strings.Index(md, "**complyscribe**")
	if idxCtl > idxScribe {
		t.Errorf("repos should be sorted alphabetically in file manifest")
	}
}

func TestToMarkdown_FileManifestSplitsChangedAndUnchanged(t *testing.T) {
	result := &syncResult{
		synced:         2,
		filesProcessed: 5,
		updated:        []string{"complyctl"},
		added:          []string{"complyscribe"},
		repoDetails: map[string]repoSummary{
			"complyctl":    {htmlURL: "https://github.com/org/complyctl", newSHA: "aaa"},
			"complyscribe": {htmlURL: "https://github.com/org/complyscribe", newSHA: "bbb"},
		},
		repoFiles: map[string][]string{
			"complyctl":    {"docs/usage.md", "docs/api.md", "docs/installation.md"},
			"complyscribe": {"docs/guide.md", "README.md"},
		},
		changedRepoFiles: map[string][]string{
			"complyctl": {"docs/api.md"},
		},
	}

	md := result.toMarkdown()

	checks := []struct {
		name     string
		contains string
	}{
		{"changed section", "Changed files (1 across 1 repository)"},
		{"unchanged section", "Unchanged files (4 across 2 repositories)"},
		{"changed file", "`docs/api.md`"},
		{"unchanged file", "`docs/usage.md`"},
		{"unchanged file", "`docs/installation.md`"},
		{"unchanged file", "`docs/guide.md`"},
		{"unchanged file", "`README.md`"},
	}
	for _, tc := range checks {
		if !strings.Contains(md, tc.contains) {
			t.Errorf("%s: expected markdown to contain %q\n\nGot:\n%s", tc.name, tc.contains, md)
		}
	}

	if strings.Contains(md, "Synced files") {
		t.Errorf("should not use old 'Synced files' label\n\nGot:\n%s", md)
	}
}

func TestToMarkdown_FileManifestAllUnchanged(t *testing.T) {
	result := &syncResult{
		synced:  1,
		updated: []string{"complyctl"},
		repoFiles: map[string][]string{
			"complyctl": {"docs/api.md", "docs/usage.md"},
		},
	}

	md := result.toMarkdown()

	if strings.Contains(md, "Changed files") {
		t.Errorf("should not show changed section when no files changed\n\nGot:\n%s", md)
	}
	if !strings.Contains(md, "Unchanged files (2 across 1 repository)") {
		t.Errorf("expected unchanged files section\n\nGot:\n%s", md)
	}
}

func TestToMarkdown_NoFileManifestWhenEmpty(t *testing.T) {
	result := &syncResult{
		synced: 1,
		added:  []string{"repo-a"},
	}

	md := result.toMarkdown()

	if strings.Contains(md, "Synced files") {
		t.Errorf("should not show file manifest when no files tracked\n\nGot:\n%s", md)
	}
}

func TestSyncRepoDocPages_RecordsRepoFiles(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/testorg/test-repo/contents/docs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]DirEntry{
			{Name: "installation.md", Path: "docs/installation.md", Type: "file"},
			{Name: "usage.md", Path: "docs/usage.md", Type: "file"},
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/installation.md", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# Installation\n\nSteps."),
			Encoding: "base64",
			SHA:      "sha-install",
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/usage.md", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# Usage\n\nRun it."),
			Encoding: "base64",
			SHA:      "sha-usage",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	ctx := context.Background()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
		PushedAt:      "2025-01-15T00:00:00Z",
	}

	discovery := Discovery{ScanPaths: []string{"docs"}}

	t.Run("write mode records files", func(t *testing.T) {
		output := t.TempDir()
		result := &syncResult{}
		syncRepoDocPages(ctx, gh, "testorg", repo, output, true, discovery, nil, nil, result, "")

		files := result.repoFiles["test-repo"]
		if len(files) != 2 {
			t.Fatalf("repoFiles[test-repo] = %d files, want 2", len(files))
		}
		changed := result.changedRepoFiles["test-repo"]
		if len(changed) != 2 {
			t.Fatalf("changedRepoFiles[test-repo] = %d files, want 2 (first write)", len(changed))
		}
	})

	t.Run("dry-run records files", func(t *testing.T) {
		output := t.TempDir()
		result := &syncResult{}
		syncRepoDocPages(ctx, gh, "testorg", repo, output, false, discovery, nil, nil, result, "")

		files := result.repoFiles["test-repo"]
		if len(files) != 2 {
			t.Fatalf("dry-run repoFiles[test-repo] = %d files, want 2", len(files))
		}
		if result.changedRepoFiles != nil {
			t.Error("dry-run should not populate changedRepoFiles")
		}
	})

	t.Run("second write records unchanged files", func(t *testing.T) {
		output := t.TempDir()
		// First write creates the files
		result1 := &syncResult{}
		syncRepoDocPages(ctx, gh, "testorg", repo, output, true, discovery, nil, nil, result1, "")

		// Second write with identical content — nothing should change
		result2 := &syncResult{}
		syncRepoDocPages(ctx, gh, "testorg", repo, output, true, discovery, nil, nil, result2, "")

		files := result2.repoFiles["test-repo"]
		if len(files) != 2 {
			t.Fatalf("repoFiles[test-repo] = %d files, want 2", len(files))
		}
		changed := result2.changedRepoFiles["test-repo"]
		if len(changed) != 0 {
			t.Errorf("changedRepoFiles[test-repo] = %d files, want 0 (content unchanged)", len(changed))
		}
	})
}

func TestToMarkdown_SortedOutput(t *testing.T) {
	result := &syncResult{
		synced: 3,
		added:  []string{"zebra", "alpha", "middle"},
		repoDetails: map[string]repoSummary{
			"zebra":  {htmlURL: "https://github.com/org/zebra", newSHA: "aaa"},
			"alpha":  {htmlURL: "https://github.com/org/alpha", newSHA: "bbb"},
			"middle": {htmlURL: "https://github.com/org/middle", newSHA: "ccc"},
		},
	}

	md := result.toMarkdown()

	idxAlpha := strings.Index(md, "alpha")
	idxMiddle := strings.Index(md, "middle")
	idxZebra := strings.Index(md, "zebra")
	if idxAlpha >= idxMiddle || idxMiddle >= idxZebra {
		t.Errorf("expected sorted order alpha < middle < zebra in markdown\n\nGot:\n%s", md)
	}
}

func TestProcessRepo_RecordsRepoDetail(t *testing.T) {
	branchSHA := "abc123def456"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/testorg/test-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# test-repo\n\nContent."),
			Encoding: "base64",
			SHA:      "sha-readme",
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/branches/main", func(w http.ResponseWriter, r *http.Request) {
		resp := BranchResponse{}
		resp.Commit.SHA = branchSHA
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
	}

	result := &syncResult{}
	processRepo(context.Background(), gh, "testorg", output, repo, true, false, result, map[string]repoState{}, nil, "")

	detail, ok := result.repoDetails["test-repo"]
	if !ok {
		t.Fatal("processRepo should record repoDetails for test-repo")
	}
	if detail.description != "A test repository" {
		t.Errorf("description = %q, want %q", detail.description, "A test repository")
	}
	if detail.newSHA != branchSHA {
		t.Errorf("newSHA = %q, want %q", detail.newSHA, branchSHA)
	}
	if detail.htmlURL != "https://github.com/testorg/test-repo" {
		t.Errorf("htmlURL = %q, want %q", detail.htmlURL, "https://github.com/testorg/test-repo")
	}
}

func TestParseNameList_RepoFilterOverridesExclude(t *testing.T) {
	_ = parseNameList("")
	excludeSet := parseNameList("complyctl,complyscribe")

	repoFilter := "complytime/complyctl"
	parts := strings.SplitN(repoFilter, "/", 2)
	shortName := parts[1]
	includeSet := map[string]bool{shortName: true}
	delete(excludeSet, shortName)

	if excludeSet["complyctl"] {
		t.Error("complyctl should have been removed from excludeSet by --repo filter")
	}
	if !excludeSet["complyscribe"] {
		t.Error("complyscribe should remain in excludeSet")
	}
	if !includeSet["complyctl"] {
		t.Error("complyctl should be in includeSet")
	}
}
