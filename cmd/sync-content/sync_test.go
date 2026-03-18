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
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64(readmeContent),
			Encoding: "base64",
			SHA:      "sha-readme",
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/branches/main", func(w http.ResponseWriter, r *http.Request) {
		resp := BranchResponse{}
		resp.Commit.SHA = branchSHA
		json.NewEncoder(w).Encode(resp)
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
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/repos/testorg/test-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		readmeCalls++
		json.NewEncoder(w).Encode(FileResponse{
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
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64(readmeContent),
			Encoding: "base64",
			SHA:      readmeSHA,
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/branches/main", func(w http.ResponseWriter, r *http.Request) {
		resp := BranchResponse{}
		resp.Commit.SHA = "new-branch-sha"
		json.NewEncoder(w).Encode(resp)
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
		json.NewEncoder(w).Encode(FileResponse{
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
		if result.synced != 1 {
			t.Errorf("synced = %d, want 1", result.synced)
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

		if result.synced != 1 {
			t.Errorf("dry-run synced = %d, want 1", result.synced)
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
		json.NewEncoder(w).Encode(FileResponse{
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
		json.NewEncoder(w).Encode([]DirEntry{
			{Name: "installation.md", Path: "docs/installation.md", Type: "file"},
			{Name: "usage.md", Path: "docs/usage.md", Type: "file"},
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/installation.md", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# Installation\n\nRun `go install`."),
			Encoding: "base64",
			SHA:      "sha-install",
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/usage.md", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(FileResponse{
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
	if result.synced != 2 {
		t.Errorf("synced = %d, want 2", result.synced)
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
		json.NewEncoder(w).Encode([]DirEntry{
			{Name: "installation.md", Path: "docs/installation.md", Type: "file"},
			{Name: "usage.md", Path: "docs/usage.md", Type: "file"},
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/installation.md", func(w http.ResponseWriter, r *http.Request) {
		fetchedFiles["docs/installation.md"] = true
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# Installation\n\nSteps."),
			Encoding: "base64",
			SHA:      "sha-install",
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/usage.md", func(w http.ResponseWriter, r *http.Request) {
		fetchedFiles["docs/usage.md"] = true
		json.NewEncoder(w).Encode(FileResponse{
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

	if result.synced != 1 {
		t.Errorf("synced = %d, want 1 (only the non-tracked file)", result.synced)
	}
}

func TestPathTraversalRejection(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/complyctl/contents/README.md", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(FileResponse{
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
		json.NewEncoder(w).Encode(resp)
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
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/repos/testorg/test-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# test-repo\n\nContent"),
			Encoding: "base64",
			SHA:      "sha-readme",
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()

	os.MkdirAll(filepath.Join(output, "content", "docs", "projects", "test-repo"), 0o000)
	defer os.Chmod(filepath.Join(output, "content", "docs", "projects", "test-repo"), 0o755)

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
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/repos/testorg/test-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		readmeRefReceived = r.URL.Query().Get("ref")
		json.NewEncoder(w).Encode(FileResponse{
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
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/repos/testorg/test-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		readmeRefReceived = r.URL.Query().Get("ref")
		json.NewEncoder(w).Encode(FileResponse{
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

func TestParseNameList_RepoFilterOverridesExclude(t *testing.T) {
	includeSet := parseNameList("")
	excludeSet := parseNameList("complyctl,complyscribe")

	repoFilter := "complytime/complyctl"
	parts := strings.SplitN(repoFilter, "/", 2)
	shortName := parts[1]
	includeSet = map[string]bool{shortName: true}
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
