// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const manifestFile = ".sync-manifest.json"

// readExistingState reads source_sha and readme_sha from existing project
// pages to enable two-tier change detection: branch SHA as a fast pre-filter,
// readme SHA for content-level comparison.
func readExistingState(outputDir string) map[string]repoState {
	state := make(map[string]repoState)
	dir := filepath.Join(outputDir, "content", "docs", "projects")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return state
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		repoName := e.Name()
		indexPath := filepath.Join(dir, repoName, "_index.md")
		params := readFrontmatterParams(indexPath)
		branchSHA, _ := params["source_sha"].(string)
		if branchSHA != "" {
			readmeSHA, _ := params["readme_sha"].(string)
			state[repoName] = repoState{
				branchSHA: branchSHA,
				readmeSHA: readmeSHA,
			}
		}
	}
	return state
}

// carryForwardManifest records files from the previous manifest that belong to
// a repo being skipped on the fast path, preventing orphan cleanup from
// deleting them.
func carryForwardManifest(result *syncResult, repoName string, oldManifest map[string]bool) {
	prefix := "content/docs/projects/" + repoName + "/"
	for relPath := range oldManifest {
		if strings.HasPrefix(relPath, prefix) {
			result.recordFile(relPath)
		}
	}
}

// buildDocPagesIndex pre-computes which repos have doc pages (files other than
// _index.md) in the manifest. This avoids an O(manifest) scan per repo during
// the concurrent worker loop.
func buildDocPagesIndex(manifest map[string]bool) map[string]bool {
	index := make(map[string]bool)
	const prefix = "content/docs/projects/"
	for relPath := range manifest {
		if !strings.HasPrefix(relPath, prefix) {
			continue
		}
		tail := relPath[len(prefix):]
		if slash := strings.IndexByte(tail, '/'); slash > 0 {
			repoName := tail[:slash]
			if filepath.Base(relPath) != "_index.md" {
				index[repoName] = true
			}
		}
	}
	return index
}

// readFrontmatterParams reads the YAML frontmatter from a Hugo content file
// and returns the "params" map. Returns nil if the file cannot be read or has
// no frontmatter/params section.
func readFrontmatterParams(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return nil
	}
	endIdx := strings.Index(content[4:], "\n---")
	if endIdx < 0 {
		return nil
	}
	fmBytes := content[4 : 4+endIdx]

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(fmBytes), &fm); err != nil {
		return nil
	}
	params, _ := fm["params"].(map[string]any)
	return params
}

// readManifest loads the set of files written by the previous sync run.
// Returns nil if no manifest exists (e.g. first run).
func readManifest(outputDir string) map[string]bool {
	data, err := os.ReadFile(filepath.Join(outputDir, manifestFile))
	if err != nil {
		return nil
	}
	var files []string
	if err := json.Unmarshal(data, &files); err != nil {
		slog.Warn("could not parse sync manifest", "error", err)
		return nil
	}
	m := make(map[string]bool, len(files))
	for _, f := range files {
		m[f] = true
	}
	return m
}

// writeManifest persists the list of files written during this sync run.
func writeManifest(outputDir string, files []string) error {
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Strings(sorted)
	data, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, manifestFile), append(data, '\n'), 0o644)
}
