// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const defaultWorkers = 5

// repoState holds the SHAs read from an existing project page for two-tier
// change detection: branchSHA is a fast pre-filter, readmeSHA enables
// content-level comparison when the branch has moved.
type repoState struct {
	branchSHA string
	readmeSHA string
}

// syncResult tracks outcomes for the final summary and exit code.
type syncResult struct {
	mu           sync.Mutex
	synced       int
	skipped      int
	warnings     int
	errors       int
	added        []string
	updated      []string
	removed      []string
	unchanged    []string
	writtenFiles []string
}

// recordFile appends a relative file path to the manifest of files written
// during this sync run. Thread-safe.
func (r *syncResult) recordFile(relPath string) {
	r.mu.Lock()
	r.writtenFiles = append(r.writtenFiles, relPath)
	r.mu.Unlock()
}

func (r *syncResult) addError()   { r.mu.Lock(); r.errors++; r.mu.Unlock() }
func (r *syncResult) addWarning() { r.mu.Lock(); r.warnings++; r.mu.Unlock() }
func (r *syncResult) addSynced()  { r.mu.Lock(); r.synced++; r.mu.Unlock() }

func (r *syncResult) hasChanges() bool {
	return len(r.added) > 0 || len(r.updated) > 0 || len(r.removed) > 0
}

func (r *syncResult) toMarkdown() string {
	var b strings.Builder
	b.WriteString("## Content Sync Summary\n\n")

	if len(r.added) > 0 {
		b.WriteString("### New Repositories\n\n")
		for _, name := range r.added {
			fmt.Fprintf(&b, "- `%s`\n", name)
		}
		b.WriteString("\n")
	}
	if len(r.updated) > 0 {
		b.WriteString("### Updated\n\n")
		for _, name := range r.updated {
			fmt.Fprintf(&b, "- `%s`\n", name)
		}
		b.WriteString("\n")
	}
	if len(r.removed) > 0 {
		b.WriteString("### Removed\n\n")
		for _, name := range r.removed {
			fmt.Fprintf(&b, "- `%s`\n", name)
		}
		b.WriteString("\n")
	}
	if !r.hasChanges() {
		b.WriteString("No changes detected.\n\n")
	}

	fmt.Fprintf(&b, "**Stats**: %d synced, %d skipped",
		r.synced, r.skipped)
	if r.warnings > 0 {
		fmt.Fprintf(&b, ", %d warnings", r.warnings)
	}
	if r.errors > 0 {
		fmt.Fprintf(&b, ", %d errors", r.errors)
	}
	b.WriteString("\n")

	return b.String()
}

func (r *syncResult) printSummary() {
	slog.Info("sync summary",
		"synced", r.synced,
		"skipped", r.skipped,
		"warnings", r.warnings,
		"errors", r.errors,
	)
	if len(r.added) > 0 {
		slog.Info("new repos", "repos", strings.Join(r.added, ", "))
	}
	if len(r.updated) > 0 {
		slog.Info("updated repos", "repos", strings.Join(r.updated, ", "))
	}
	if len(r.removed) > 0 {
		slog.Info("removed repos", "repos", strings.Join(r.removed, ", "))
	}
	if !r.hasChanges() {
		slog.Info("no content changes detected")
	}
}

// writeFileSafe writes data to path, skipping the write if the file already
// exists with identical content. Returns true if the file was actually written.
func writeFileSafe(path string, data []byte) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, data) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, data, 0o644) //nolint:gosec // G306: content files need 0o644 for Hugo
}

// syncConfigSource processes all FileSpec entries for a single config source,
// fetching each declared file from GitHub, applying the requested transforms,
// and writing the result to the output directory. When ref is non-empty,
// content is fetched at that specific commit SHA.
func syncConfigSource(ctx context.Context, gh *apiClient, src Source, defaults Defaults, output string, write bool, result *syncResult, ref string) {
	parts := strings.SplitN(src.Repo, "/", 2)
	if len(parts) != 2 {
		slog.Error("invalid repo format in config, expected owner/name", "repo", src.Repo)
		result.addError()
		return
	}
	owner, repoName := parts[0], parts[1]
	logger := slog.With("config_repo", src.Repo)

	for _, file := range src.Files {
		content, fileSHA, err := gh.getFileContent(ctx, owner, repoName, file.Src, ref)
		if err != nil {
			logger.Error("could not fetch config file", "src", file.Src, "error", err)
			result.addError()
			continue
		}

		if file.Transform.StripBadges {
			content = stripBadges(content)
		}
		content = stripLeadingH1(content)
		content = shiftHeadings(content)
		content = titleCaseHeadings(content)
		if file.Transform.RewriteLinks {
			content = rewriteRelativeLinks(content, owner, repoName, src.Branch)
		}

		out := []byte(content)
		if len(file.Transform.InjectFrontmatter) > 0 {
			var fmErr error
			out, fmErr = injectFrontmatter(out, file.Transform.InjectFrontmatter)
			if fmErr != nil {
				logger.Error("frontmatter injection failed", "src", file.Src, "error", fmErr)
				result.addError()
				continue
			}
		}

		shortSHA := fileSHA
		if len(shortSHA) > 12 {
			shortSHA = shortSHA[:12]
		}
		provenance := fmt.Sprintf(
			"<!-- synced from %s/%s@%s (%s) -->\n",
			src.Repo, file.Src, src.Branch, shortSHA,
		)
		out = insertAfterFrontmatter(out, []byte(provenance))

		destPath := filepath.Join(output, file.Dest)

		if !isUnderDir(output, destPath) {
			logger.Error("path traversal blocked", "dest", file.Dest, "resolved", destPath)
			result.addError()
			continue
		}

		if !write {
			logger.Info("would write config file (dry-run)", "src", file.Src, "dest", destPath)
			result.addSynced()
			continue
		}

		written, err := writeFileSafe(destPath, out)
		if err != nil {
			logger.Error("error writing config file", "src", file.Src, "dest", destPath, "error", err)
			result.addError()
			continue
		}

		result.recordFile(file.Dest)

		if written {
			logger.Info("wrote config file", "src", file.Src, "dest", destPath)
		} else {
			logger.Info("config file unchanged", "src", file.Src, "dest", destPath)
		}

		result.addSynced()
	}
}

func parseNameList(raw string) map[string]bool {
	set := make(map[string]bool)
	for _, name := range strings.Split(raw, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			set[name] = true
		}
	}
	return set
}

// repoWork holds the inputs and outputs for processing a single repo.
type repoWork struct {
	repo      Repo
	sha       string
	card      ProjectCard
	unchanged bool
}

// processRepo handles a single repository: fetches content, writes pages.
// When skipReadme is true, README fetching and project page generation are
// skipped but the ProjectCard is still produced.
//
// lockedSHA, when non-empty, pins content fetches to the approved commit.
// If the upstream branch has moved past the lock, content is still fetched
// at the locked version so only reviewed content reaches production.
//
// Two-tier change detection:
//  1. Branch SHA unchanged → skip all fetches (fast path).
//  2. Branch SHA changed  → fetch README, compare blob SHA for accurate
//     content-level change reporting.
//
// All shared state mutations go through result.mu.
func processRepo(ctx context.Context, gh *apiClient, org, output string, repo Repo, write bool, skipReadme bool, result *syncResult, oldState map[string]repoState, oldManifest map[string]bool, lockedSHA string) *repoWork {
	logger := slog.With("repo", repo.Name)

	sha, err := gh.getBranchSHA(ctx, org, repo.Name, repo.DefaultBranch)
	if err != nil {
		logger.Warn("could not get branch SHA", "error", err)
		sha = "unknown"
		result.addWarning()
	}

	old, existed := oldState[repo.Name]

	// Fast path: branch hasn't changed since last sync — skip all fetches.
	if existed && old.branchSHA == sha {
		result.mu.Lock()
		result.unchanged = append(result.unchanged, repo.Name)
		result.mu.Unlock()
		result.addSynced()

		if !write {
			logger.Info("unchanged (branch SHA match), skipping", "sha", sha)
			return &repoWork{repo: repo, sha: sha, card: buildProjectCard(repo), unchanged: true}
		}

		logger.Info("unchanged (branch SHA match), skipping fetches", "sha", sha)
		if oldManifest != nil {
			carryForwardManifest(result, repo.Name, oldManifest)
		}

		return &repoWork{repo: repo, sha: sha, card: buildProjectCard(repo), unchanged: true}
	}

	// Dry-run: report what would happen without fetching content.
	if !write {
		result.mu.Lock()
		if !existed {
			result.added = append(result.added, repo.Name)
		} else {
			result.updated = append(result.updated, repo.Name)
		}
		result.mu.Unlock()
		result.addSynced()
		logger.Info("would sync (dry-run)", "sha", sha)
		return &repoWork{repo: repo, sha: sha, card: buildProjectCard(repo)}
	}

	// Slow path: branch SHA changed — fetch content and compare file-level SHAs.
	// When a lock is active, fetch at the locked commit rather than HEAD.
	fetchRef := ""
	if lockedSHA != "" && lockedSHA != sha {
		fetchRef = lockedSHA
	}

	contentChanged := !existed
	var readmeSHA string

	if !skipReadme {
		readme, rSHA, err := gh.getREADME(ctx, org, repo.Name, fetchRef)
		readmeSHA = rSHA
		if err != nil {
			logger.Warn("no README found", "error", err)
			result.addWarning()
		}

		if existed && old.readmeSHA != "" && old.readmeSHA == readmeSHA {
			logger.Info("README unchanged despite branch update", "branch_sha", sha, "readme_sha", readmeSHA)
		} else {
			contentChanged = true
		}

		if readme != "" {
			readme = stripLeadingH1(readme)
			readme = shiftHeadings(readme)
			readme = titleCaseHeadings(readme)
			readme = stripBadges(readme)
			readme = rewriteRelativeLinks(readme, org, repo.Name, repo.DefaultBranch)
		} else {
			readme = fmt.Sprintf(
				"*No README available.* Visit the [repository on GitHub](%s) for more information.\n",
				repo.HTMLURL,
			)
		}

		indexPage := buildSectionIndex(repo, sha, readmeSHA)
		indexRel := filepath.Join("content", "docs", "projects", repo.Name, "_index.md")
		indexPath := filepath.Join(output, indexRel)
		if !isUnderDir(output, indexPath) {
			logger.Error("path traversal blocked", "path", indexRel)
			result.addError()
			return nil
		}
		written, err := writeFileSafe(indexPath, []byte(indexPage))
		if err != nil {
			logger.Error("error writing section index", "path", indexPath, "error", err)
			result.addError()
			return nil
		}
		result.recordFile(indexRel)
		if written {
			logger.Info("wrote section index", "path", indexPath)
		} else {
			logger.Info("section index unchanged", "path", indexPath)
		}

		overviewPage := buildOverviewPage(repo, readme)
		overviewRel := filepath.Join("content", "docs", "projects", repo.Name, "overview.md")
		overviewPath := filepath.Join(output, overviewRel)
		if !isUnderDir(output, overviewPath) {
			logger.Error("path traversal blocked", "path", overviewRel)
			result.addError()
			return nil
		}
		written, err = writeFileSafe(overviewPath, []byte(overviewPage))
		if err != nil {
			logger.Error("error writing overview page", "path", overviewPath, "error", err)
			result.addError()
			return nil
		}
		result.recordFile(overviewRel)
		if written {
			logger.Info("wrote overview page", "path", overviewPath)
		} else {
			logger.Info("overview page unchanged", "path", overviewPath)
		}
	}

	result.mu.Lock()
	switch {
	case !existed:
		result.added = append(result.added, repo.Name)
	case contentChanged:
		result.updated = append(result.updated, repo.Name)
	default:
		result.unchanged = append(result.unchanged, repo.Name)
	}
	result.mu.Unlock()
	result.addSynced()

	return &repoWork{repo: repo, sha: sha, card: buildProjectCard(repo)}
}

// syncRepoDocPages auto-syncs Markdown files found under each scan_path in the
// discovery config. Files already tracked by explicit config sources or listed
// in ignoreFiles are skipped. Intermediate directories get auto-generated
// _index.md section pages. When ref is non-empty, content is fetched at that
// specific commit SHA.
func syncRepoDocPages(ctx context.Context, gh *apiClient, org string, repo Repo, output string, write bool, discovery Discovery, ignoreFiles map[string]bool, configTracked map[string]bool, result *syncResult, ref string) {
	logger := slog.With("repo", repo.Name, "phase", "doc-pages")

	for _, scanPath := range discovery.ScanPaths {
		files, err := gh.listDirMD(ctx, org, repo.Name, scanPath, ref)
		if err != nil {
			logger.Debug("scan path not found", "path", scanPath, "error", err)
			continue
		}

		neededDirs := make(map[string]bool)

		for _, filePath := range files {
			baseName := filepath.Base(filePath)
			if ignoreFiles[baseName] {
				continue
			}
			if configTracked[filePath] {
				continue
			}

			relPath := strings.TrimPrefix(filePath, scanPath+"/")
			destRel := filepath.Join("content", "docs", "projects", repo.Name, relPath)
			destPath := filepath.Join(output, destRel)

			if !isUnderDir(output, destPath) {
				logger.Error("path traversal blocked", "src", filePath, "dest", destRel)
				result.addError()
				continue
			}

			dir := filepath.Dir(relPath)
			for dir != "." && dir != "" {
				neededDirs[dir] = true
				dir = filepath.Dir(dir)
			}

			if !write {
				logger.Info("would write doc page (dry-run)", "src", filePath, "dest", destRel)
				result.addSynced()
				continue
			}

			content, sha, err := gh.getFileContent(ctx, org, repo.Name, filePath, ref)
			if err != nil {
				logger.Warn("could not fetch doc file", "path", filePath, "error", err)
				result.addWarning()
				continue
			}

			content = stripBadges(content)
			content = stripLeadingH1(content)
			content = shiftHeadings(content)
			content = titleCaseHeadings(content)
			fileDir := filepath.Dir(filePath)
			content = rewriteRelativeLinks(content, org, repo.Name, repo.DefaultBranch, fileDir)

			page := buildDocPage(filePath, repo.FullName, repo.Description, repo.PushedAt, repo.DefaultBranch, sha, content)

			written, err := writeFileSafe(destPath, []byte(page))
			if err != nil {
				logger.Error("error writing doc page", "path", destPath, "error", err)
				result.addError()
				continue
			}

			result.recordFile(destRel)
			if written {
				logger.Info("wrote doc page", "src", filePath, "dest", destPath)
			} else {
				logger.Info("doc page unchanged", "src", filePath, "dest", destPath)
			}

			result.addSynced()
		}

		for dir := range neededDirs {
			indexRel := filepath.Join("content", "docs", "projects", repo.Name, dir, "_index.md")
			indexPath := filepath.Join(output, indexRel)

			if !isUnderDir(output, indexPath) {
				logger.Error("path traversal blocked for section index", "path", indexRel)
				result.addError()
				continue
			}

			if _, err := os.Stat(indexPath); err == nil {
				result.recordFile(indexRel)
				continue
			}

			if !write {
				continue
			}

			title := titleFromFilename(filepath.Base(dir))
			var b strings.Builder
			b.WriteString("---\n")
			fmt.Fprintf(&b, "title: %q\n", title)
			fmt.Fprintf(&b, "description: %q\n", repo.Description+" — "+title)
			fmt.Fprintf(&b, "date: %s\n", repo.PushedAt)
			fmt.Fprintf(&b, "lastmod: %s\n", repo.PushedAt)
			b.WriteString("draft: false\n")
			b.WriteString("---\n")

			written, err := writeFileSafe(indexPath, []byte(b.String()))
			if err != nil {
				logger.Error("error writing section index", "path", indexPath, "error", err)
				continue
			}

			result.recordFile(indexRel)
			if written {
				logger.Info("wrote section index", "path", indexPath)
			}
		}
	}
}

// writeGitHubOutputs writes structured outputs for GitHub Actions integration.
func writeGitHubOutputs(result *syncResult) {
	if ghOutput := os.Getenv("GITHUB_OUTPUT"); ghOutput != "" {
		ghOutput = filepath.Clean(ghOutput)
		f, err := os.OpenFile(ghOutput, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600) //nolint:gosec // G304: path from trusted Actions env
		if err == nil {
			defer func() { _ = f.Close() }()
			hasChanges := "false"
			if result.hasChanges() {
				hasChanges = "true"
			}
			_, _ = fmt.Fprintf(f, "has_changes=%s\n", hasChanges)
			_, _ = fmt.Fprintf(f, "changed_count=%d\n", len(result.added)+len(result.updated))
			_, _ = fmt.Fprintf(f, "error_count=%d\n", result.errors)
		}
	}

	if summaryPath := os.Getenv("GITHUB_STEP_SUMMARY"); summaryPath != "" {
		summaryPath = filepath.Clean(summaryPath)
		f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600) //nolint:gosec // G304: path from trusted Actions env
		if err == nil {
			defer func() { _ = f.Close() }()
			_, _ = fmt.Fprint(f, result.toMarkdown())
		}
	}
}
