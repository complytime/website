// SPDX-License-Identifier: Apache-2.0

// Command sync-content pulls documentation from upstream GitHub repositories
// into the website's Hugo content tree. It operates in hybrid mode:
//
//  1. Governance-driven discovery — reads the org's peribolos.yaml governance
//     registry to determine which repos exist, then enriches each with GitHub
//     API metadata. Generates project pages and landing-page cards.
//  2. Config sync — reads sync-config.yaml and pulls specific files with
//     transforms (frontmatter injection, link rewriting, badge stripping).
//
// The tool is fully idempotent: unchanged files are not rewritten. A sync
// manifest (.sync-manifest.json) tracks written files so orphaned content
// from previous runs is cleaned up automatically.
//
// Usage:
//
//	go run ./cmd/sync-content --org complytime --config sync-config.yaml          # dry-run
//	go run ./cmd/sync-content --org complytime --config sync-config.yaml --write  # apply
//	go run ./cmd/sync-content --config sync-config.yaml --repo complytime/complyctl --write  # single repo
//
// Environment:
//
//	GITHUB_TOKEN   GitHub API token (recommended; unauthenticated rate limit is 60 req/hr)
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

func main() { os.Exit(run()) }

func run() int {
	org := flag.String("org", "complytime", "GitHub organization name")
	token := flag.String("token", "", "GitHub API token (or set GITHUB_TOKEN env var)")
	output := flag.String("output", ".", "Hugo site root directory")
	include := flag.String("include", "", "Comma-separated repo allowlist (empty = auto-discover all)")
	exclude := flag.String("exclude", "", "Comma-separated repo names to skip (merged with config discovery.ignore_repos)")
	write := flag.Bool("write", false, "Apply changes to disk (default: dry-run)")
	summaryFile := flag.String("summary", "", "Write markdown change summary to this file (for PR body)")
	timeout := flag.Duration("timeout", 3*time.Minute, "Overall timeout for all API operations")
	workers := flag.Int("workers", defaultWorkers, "Maximum concurrent repo processing goroutines")
	configPath := flag.String("config", "", "Path to sync-config.yaml for declarative file syncs")
	repoFilter := flag.String("repo", "", "Sync only this repo (e.g. complytime/complyctl)")
	lockPath := flag.String("lock", "", "Path to .content-lock.json for content approval gating")
	updateLock := flag.Bool("update-lock", false, "Write updated upstream SHAs to the lockfile (requires --lock)")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if *workers < 1 {
		slog.Error("--workers must be at least 1", "value", *workers)
		return 1
	}

	apiToken := *token
	if apiToken != "" {
		slog.Warn("--token flag is visible in process listings and shell history; prefer GITHUB_TOKEN env var")
	} else {
		apiToken = os.Getenv("GITHUB_TOKEN")
	}
	if apiToken == "" {
		slog.Warn("no GitHub token provided; API rate limits will be restrictive")
	}

	includeSet := parseNameList(*include)
	excludeSet := parseNameList(*exclude)

	if *repoFilter != "" {
		parts := strings.SplitN(*repoFilter, "/", 2)
		shortName := *repoFilter
		if len(parts) == 2 {
			shortName = parts[1]
		}
		includeSet = map[string]bool{shortName: true}
		delete(excludeSet, shortName)
		slog.Info("filtering to single repo", "repo", *repoFilter)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	gh := &apiClient{
		token: apiToken,
		http:  &http.Client{Timeout: 30 * time.Second},
	}

	var cfg *SyncConfig
	if *configPath != "" {
		var err error
		cfg, err = loadConfig(*configPath)
		if err != nil {
			slog.Error("error loading config", "error", err)
			return 1
		}
		slog.Info("loaded sync config", "path", *configPath, "sources", len(cfg.Sources))
		for _, r := range cfg.Discovery.IgnoreRepos {
			if !includeSet[r] {
				excludeSet[r] = true
			}
		}
	}

	if *updateLock && *lockPath == "" {
		slog.Error("--update-lock requires --lock")
		return 1
	}

	var lock *ContentLock
	if *lockPath != "" {
		var err error
		lock, err = readLock(*lockPath)
		if err != nil {
			slog.Error("error loading lockfile", "error", err)
			return 1
		}
		slog.Info("loaded content lock", "path", *lockPath, "repos", len(lock.Repos))
	}

	// lockGate is true when the lock should constrain which repos are synced
	// and pin content to approved SHAs. Disabled during --update-lock, which
	// scans HEAD to propose lockfile updates.
	lockGate := lock != nil && len(lock.Repos) > 0 && !*updateLock

	configSources := make(map[string]Source)
	if cfg != nil {
		for _, src := range cfg.Sources {
			if *repoFilter != "" && src.Repo != *repoFilter {
				continue
			}
			configSources[src.Repo] = src
		}
	}

	oldState := readExistingState(*output)
	oldManifest := readManifest(*output)

	slog.Info("fetching governance registry", "org", *org)
	repoNames, err := gh.fetchPeribolosRepos(ctx, *org)
	if err != nil {
		slog.Error("error fetching peribolos.yaml", "error", err)
		return 1
	}
	slog.Info("found repos in governance registry", "count", len(repoNames))

	peribolosSet := make(map[string]bool, len(repoNames))
	for _, name := range repoNames {
		peribolosSet[name] = true
	}

	if *repoFilter != "" {
		parts := strings.SplitN(*repoFilter, "/", 2)
		shortName := *repoFilter
		if len(parts) == 2 {
			shortName = parts[1]
		}
		if !peribolosSet[shortName] {
			slog.Error("--repo target is not in the governance registry (peribolos.yaml)", "repo", *repoFilter)
			return 1
		}
	}

	var repos []Repo
	for _, name := range repoNames {
		if *repoFilter != "" && !includeSet[name] {
			continue
		}
		r, err := gh.getRepoMetadata(ctx, *org, name)
		if err != nil {
			slog.Warn("could not fetch repo metadata, skipping", "repo", name, "error", err)
			continue
		}
		repos = append(repos, *r)
	}
	slog.Info("found repositories", "count", len(repos))

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Name < repos[j].Name
	})

	var result syncResult
	newState := make(map[string]bool)

	var eligible []Repo
	for _, repo := range repos {
		if !isValidRepoName(repo.Name) {
			slog.Warn("skipping repo with invalid name", "name", repo.Name)
			result.skipped++
			continue
		}
		included := len(includeSet) == 0 || includeSet[repo.Name]
		if !included || excludeSet[repo.Name] {
			result.skipped++
			continue
		}
		newState[repo.Name] = true
		eligible = append(eligible, repo)
	}

	if len(eligible) == 0 && len(oldState) > 0 && *write && len(includeSet) == 0 {
		slog.Error("refusing to clean: zero eligible repos from API but previous state has entries — possible API outage or misconfiguration",
			"old_repos", len(oldState))
		return 1
	}

	ignoreFiles := make(map[string]bool)
	if cfg != nil {
		for _, f := range cfg.Discovery.IgnoreFiles {
			ignoreFiles[f] = true
		}
	}

	docPagesIndex := buildDocPagesIndex(oldManifest)

	var upstreamSHAs sync.Map

	sem := make(chan struct{}, *workers)
	var wg sync.WaitGroup
	var cardsMu sync.Mutex
	var cards []ProjectCard
	var processedConfigMu sync.Mutex
	processedConfig := make(map[string]bool)

	for _, repo := range eligible {
		if ctx.Err() != nil {
			slog.Warn("context cancelled, stopping repo dispatch")
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(r Repo) {
			defer wg.Done()
			defer func() { <-sem }()

			slog.Info("processing repo", "repo", r.Name)

			lockedSHA := ""
			if lockGate {
				lockedSHA = lock.sha(r.Name)
				if lockedSHA == "" {
					slog.Info("repo not in lockfile, skipping (unapproved)", "repo", r.Name)
					result.mu.Lock()
					result.skipped++
					result.mu.Unlock()
					return
				}
			}

			cfgSrc, inConfig := configSources[r.FullName]
			skipReadme := inConfig && cfgSrc.SkipOrgSync

			work := processRepo(ctx, gh, *org, *output, r, *write, skipReadme, &result, oldState, oldManifest, lockedSHA)
			if work != nil {
				upstreamSHAs.Store(r.Name, work.sha)
				cardsMu.Lock()
				cards = append(cards, work.card)
				cardsMu.Unlock()
			}

			configTracked := make(map[string]bool)
			if inConfig {
				for _, f := range cfgSrc.Files {
					configTracked[f.Src] = true
				}
			}

			// Determine ref for doc pages and config sources.
			fetchRef := ""
			if work != nil && lockedSHA != "" && lockedSHA != work.sha {
				fetchRef = lockedSHA
			}

			if work != nil && !skipReadme && cfg != nil && len(cfg.Discovery.ScanPaths) > 0 {
				if !work.unchanged || !docPagesIndex[r.Name] {
					syncRepoDocPages(ctx, gh, *org, r, *output, *write, cfg.Discovery, ignoreFiles, configTracked, &result, fetchRef)
				}
			}

			if inConfig {
				syncConfigSource(ctx, gh, cfgSrc, cfg.Defaults, *output, *write, &result, fetchRef)
				processedConfigMu.Lock()
				processedConfig[r.FullName] = true
				processedConfigMu.Unlock()
			}
		}(repo)
	}

	wg.Wait()

	if cfg != nil {
		for _, src := range cfg.Sources {
			if processedConfig[src.Repo] {
				continue
			}

			parts := strings.SplitN(src.Repo, "/", 2)
			if len(parts) != 2 {
				slog.Error("config source repo must be in owner/name format", "repo", src.Repo)
				result.addError()
				continue
			}
			shortName := parts[1]

			if !peribolosSet[shortName] {
				slog.Error("config source repo is not in the governance registry (peribolos.yaml), skipping", "repo", src.Repo)
				result.addError()
				continue
			}

			if lockGate && lock.sha(src.Repo) == "" {
				slog.Info("config-only source not in lockfile, skipping (unapproved)", "repo", src.Repo)
				result.mu.Lock()
				result.skipped++
				result.mu.Unlock()
				continue
			}

			slog.Info("processing config-only source", "repo", src.Repo)

			cfgRef := ""
			if lockGate {
				sha, err := gh.getBranchSHA(ctx, parts[0], parts[1], src.Branch)
				if err == nil {
					upstreamSHAs.Store(src.Repo, sha)
					locked := lock.sha(src.Repo)
					if locked != "" && locked != sha {
						cfgRef = locked
					}
				}
			} else if *updateLock {
				sha, err := gh.getBranchSHA(ctx, parts[0], parts[1], src.Branch)
				if err == nil {
					upstreamSHAs.Store(src.Repo, sha)
				}
			}

			syncConfigSource(ctx, gh, src, cfg.Defaults, *output, *write, &result, cfgRef)

			prefix := filepath.Join("content", "docs", "projects", shortName) + string(filepath.Separator)
			for _, f := range src.Files {
				if strings.HasPrefix(f.Dest, prefix) {
					newState[shortName] = true
					break
				}
			}
		}
	}

	sort.Slice(cards, func(i, j int) bool {
		return cards[i].Name < cards[j].Name
	})

	for name := range oldState {
		if !newState[name] {
			result.removed = append(result.removed, name)
		}
	}
	sort.Strings(result.removed)

	if *write {
		if oldManifest != nil {
			orphans := cleanOrphanedFiles(*output, oldManifest, result.writtenFiles)
			if orphans > 0 {
				slog.Info("cleaned orphaned files from previous sync", "count", orphans)
			}
		}
		if err := writeManifest(*output, result.writtenFiles); err != nil {
			slog.Warn("could not write sync manifest", "error", err)
			result.addWarning()
		}
	}

	if *updateLock {
		newLock := &ContentLock{Repos: make(map[string]string)}
		upstreamSHAs.Range(func(key, value any) bool {
			newLock.Repos[key.(string)] = value.(string)
			return true
		})
		if err := writeLock(*lockPath, newLock); err != nil {
			slog.Error("error writing lockfile", "path", *lockPath, "error", err)
			return 1
		}
		slog.Info("updated content lock", "path", *lockPath, "repos", len(newLock.Repos))
	}

	if *write {
		jsonData, err := json.MarshalIndent(cards, "", "  ")
		if err != nil {
			slog.Error("error marshaling projects.json", "error", err)
			return 1
		}
		jsonPath := filepath.Join(*output, "data", "projects.json")
		written, err := writeFileSafe(jsonPath, append(jsonData, '\n'))
		if err != nil {
			slog.Error("error writing projects.json", "error", err)
			return 1
		}
		if written {
			slog.Info("wrote projects.json", "count", len(cards))
		} else {
			slog.Info("projects.json unchanged, skipped write")
		}
	}

	result.printSummary()
	writeGitHubOutputs(&result)

	if *summaryFile != "" {
		if !isUnderDir(*output, *summaryFile) {
			slog.Error("summary file path is outside output directory", "path", *summaryFile)
		} else {
			md := result.toMarkdown()
			if _, err := writeFileSafe(*summaryFile, []byte(md)); err != nil {
				slog.Error("error writing summary file", "path", *summaryFile, "error", err)
			} else {
				slog.Info("wrote change summary", "path", *summaryFile)
			}
		}
	}

	if !*write {
		slog.Info("dry run complete, no files were written")
	} else {
		slog.Info("sync complete")
	}

	if result.errors > 0 {
		return 1
	}
	return 0
}
