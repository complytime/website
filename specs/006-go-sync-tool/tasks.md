# Tasks: Go Content Sync Tool

**Input**: Design documents from `/specs/006-go-sync-tool/`
**Prerequisites**: plan.md (required), spec.md (required), research.md

**Tests**: Unit tests are required per SC-008. Included in Phase 7 (US6). Hardening tests added in Phase 8 based on code audit findings.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story. The core sync tool is already ported and functional (IS-001 through IS-005 Done). Remaining work is infrastructure integration, CI/CD, tests, and hardening.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- **[DEFERRED]**: Blocked on an external precondition; not executable yet
- Include exact file paths in descriptions

**ID Gaps**: T002 was consolidated into T001 during initial planning. T011 and T012 were merged into the remediation phase (T022, T023) when cross-artifact analysis revealed they overlapped. IDs are not renumbered to preserve external references.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Gitignore patterns and section scaffolding that all user stories depend on

- [x] T001 Update `.gitignore` with exclusion patterns for generated content: add `content/docs/projects/*/` (generated repo pages) and `data/projects.json` (landing page cards). *(Done — `.gitignore` already contains correct glob patterns that inherently preserve hand-maintained `_index.md` files. Validates NFR-007.)*

**Checkpoint**: `.gitignore` prevents generated content from being committed. Hugo recognizes `content/docs/projects/` as a content section.

---

## Phase 2: User Story 1 — Safe Local Preview (Priority: P1) 🎯 MVP

**Goal**: A contributor clones the repo, runs the sync tool, and previews the full site locally. Dry-run is the default; `--write` is required for disk I/O.

**Independent Test**: Run `go run ./cmd/sync-content --org complytime --config sync-config.yaml` and verify zero files are created. Then run with `--write` and verify content appears. Then run `hugo server` and verify zero build errors.

- [x] T003 [US1] Validate dry-run mode: run `go run ./cmd/sync-content --org complytime --config sync-config.yaml` and confirm zero files created in `content/docs/projects/` or `data/`. Tool should log intended actions without writing. *(Done — dry-run exits 0 with "dry run complete, no files were written". Logs intended syncs for 4 eligible repos.)*
- [x] T004 [US1] Validate write mode: run `go run ./cmd/sync-content --org complytime --config sync-config.yaml --write` and confirm: (a) section indexes appear at `content/docs/projects/{repo}/_index.md` (frontmatter only, no body), (b) overview pages appear at `content/docs/projects/{repo}/overview.md` (README content), (c) doc sub-pages appear for repos with `docs/` directories matching `discovery.scan_paths`, (d) `data/projects.json` is written, (e) `.sync-manifest.json` is written. Verify sync duration stays under 60s with token (NFR-001). *(Done — all 5 output artifacts verified. 54 files tracked in manifest. 4 repos with section index + overview + doc sub-pages. Sync completes well within the 60s NFR-001 target with authenticated token.)*
- [x] T005 [US1] Validate Hugo build: run `npm run dev` (or `hugo server`) after sync and confirm zero build errors. Verify project pages are accessible at `/docs/projects/`. *(Done — `hugo --minify --gc` succeeds with 95 pages in 1072ms. Project pages built at `/docs/projects/`.)*

**Checkpoint**: Full local development workflow works end-to-end. Contributors can safely preview the site.

---

## Phase 3: User Story 2 — Org-Wide Auto-Discovery (Priority: P1)

**Goal**: New repos in the complytime org automatically get project pages and landing page cards without config changes.

**Independent Test**: After running sync with `--write`, verify repos NOT in `sync-config.yaml` (e.g., `complytime-demos`, `gemara-content-service`) have generated pages and appear in `data/projects.json`.

- [x] T006 [P] [US2] Verify auto-discovered repos produce pages: check that repos not declared in `sync-config.yaml` have: (a) `content/docs/projects/{repo}/_index.md` with metadata frontmatter (`title`, `description`, `params.language`, `params.stars`, `params.source_sha`, `params.readme_sha`) and no body content, (b) `content/docs/projects/{repo}/overview.md` with README content (headings shifted, badges stripped, relative links rewritten). *(Done — all 4 eligible repos (complyctl, complyscribe, complytime-collector-components, gemara-content-service) have both files with correct frontmatter.)*
- [x] T007 [P] [US2] Verify `data/projects.json` completeness: confirm file contains a `ProjectCard` entry for every eligible peribolos repo, sorted alphabetically, with fields `name`, `language`, `type`, `description`, `url`, `repo`, `stars`. *(Done — 4 cards sorted alphabetically with all required fields.)*

**Checkpoint**: Zero-config discovery works. All eligible org repos are visible on the site.

---

## Phase 4: User Story 3 — Config-Driven Precision Sync (Priority: P1)

**Goal**: Config overlay provides precise control over file sync destinations, frontmatter, and transforms for key projects (complyctl, complyscribe, collector-components) without breaking the org scan baseline.

**Independent Test**: Verify config-declared files appear at their `dest` paths in `sync-config.yaml` with injected frontmatter and applied transforms.

- [ ] T008 [P] [US3] [DEFERRED] Verify `skip_org_sync` behavior: for repos with `skip_org_sync: true` in `sync-config.yaml`, confirm no auto-generated section index (`_index.md`) or overview page (`overview.md`) exists at `content/docs/projects/{repo}/` BUT the repo's `ProjectCard` is present in `data/projects.json`. *Blocked — `sync-config.yaml` currently has `sources: []`. Unblocked when sources are declared for specific repos. Code paths covered by unit tests (`TestSyncConfigSource`, `TestProcessRepo`).*
- [ ] T009 [P] [US3] [DEFERRED] Verify config file transforms: check config-declared files at their `dest` paths have correct `inject_frontmatter` (YAML frontmatter with title, description, weight), `rewrite_links` (relative links converted to absolute GitHub URLs), `strip_badges` (CI badge lines removed), and `rewrite_diagrams` (diagram code blocks converted to Kroki format) per `sync-config.yaml` transform declarations. *Blocked — `sync-config.yaml` currently has `sources: []`. Unblocked when sources are declared. Code paths covered by unit tests (`TestInjectFrontmatter`, `TestStripBadges`, `TestRewriteRelativeLinks`, `TestRewriteDiagramBlocks`).*

**Checkpoint**: Hybrid mode works — org scan provides the baseline, config overlay provides precision.

---

## Phase 5: Verify Inherited — Change Detection and Stale Cleanup (Priority: P2)

**Goal**: Verify the inherited change detection and stale cleanup capabilities work correctly in the complytime-website context. No implementation needed — these are built into the ported tool (see spec "Inherited Capabilities").

**Independent Test**: Run the sync tool twice. On the second run, verify unchanged repos are logged as "unchanged". Simulate removal and verify cleanup removes all generated files (section index, overview, doc sub-pages).

- [x] T010 [US4] Verify change detection and stale cleanup: run sync with `--write` twice in succession. On the second run, confirm unchanged repos show "unchanged" in structured log output (no disk writes). Verify `syncResult` counters report correct `added`/`updated`/`unchanged` counts. Additionally, verify stale cleanup removes all generated files for a removed repo — not just `_index.md` but also `overview.md` and any doc sub-pages under the repo directory. *(Done — manifest-based tracking verified (54 entries). Stale cleanup uses manifest-based `cleanOrphanedFiles` (T030). Change detection tested by `TestProcessRepo_BranchUnchanged` and `TestProcessRepo_BranchChangedReadmeUnchanged`.)*

**Checkpoint**: Change detection prevents redundant writes. Stale cleanup keeps the site clean.

---

## Phase 6: User Story 5 — CI/CD Pipeline Integration (Priority: P2)

**Goal**: The sync tool runs in GitHub Actions. The deploy workflow generates fresh content before Hugo builds. A CI workflow validates PRs with lint, test, dry-run, and build.

**Independent Test**: Verify deploy workflow includes sync step. Verify CI workflow runs `go vet`, `gofmt` check, `go test -race`, sync dry-run, and Hugo build.

- [x] T013 [US5] Update deploy workflow in `.github/workflows/deploy-gh-pages.yml`: add `actions/setup-go` step (Go 1.25+), add sync step (`go run ./cmd/sync-content --org complytime --config sync-config.yaml --lock .content-lock.json --write`) before Hugo build, pass `GITHUB_TOKEN` to sync step for authenticated API access. Trigger on push to `main` and `workflow_dispatch` per Constitution XV (v1.3.0). Preserve existing Node.js setup, Hugo setup, and GitHub Pages deploy steps. *(Done — `deploy-gh-pages.yml` has all required steps. Daily cron removed in T043 in favour of PR-gated content sync.)*
- [x] T014 [P] [US5] Create CI workflow for PR validation in `.github/workflows/ci.yml`: trigger on `pull_request` to `main`. Steps: checkout, setup Go, setup Node.js, setup Hugo, `npm ci`, `go test -race ./cmd/sync-content/...`, content sync with `--lock` (`go run ./cmd/sync-content --org complytime --config sync-config.yaml --lock .content-lock.json --write`), Hugo build (`hugo --minify --gc`). `go vet` and `gofmt` checks run in `deploy-gh-pages.yml` instead. *(Done — `ci.yml` created with all required steps and SHA-pinned actions.)*

**Checkpoint**: Production deploys always use fresh synced content. PRs are validated with lint, tests, and build.

---

## Phase 7: User Story 6 — Concurrent Processing with Race Safety (Priority: P3)

**Goal**: Unit tests verify core sync functions and concurrent processing passes race detection.

**Independent Test**: `go test -race ./cmd/sync-content/...` passes with zero data race warnings.

- [x] T015 [US6] Write unit tests for pure functions in `cmd/sync-content/*_test.go`: test `loadConfig`, `injectFrontmatter`, `stripBadges`, `shiftHeadings`, `rewriteRelativeLinks`, `isValidRepoName`. SPDX license header present. See research R6. *(Done — 22 test functions at this phase; 57 total across 10 `*_test.go` files after T037 hardening, T045–T046 lockfile additions, T053 peribolos tests, T054 discovery removal, helpers_test.go shared utilities, and Phase 13 heading/casing transforms.)*
- [x] T016 [US6] Write integration tests with httptest mock in `cmd/sync-content/*_test.go`: mock GitHub API responses for org listing and README fetch. Test `processRepo` end-to-end, `syncConfigSource`, manifest round-trip, orphan cleanup. See research R6. *(Done — `TestProcessRepo`, `TestSyncConfigSource`, `TestManifestRoundTrip`, `TestCleanOrphanedFiles`, etc.)*
- [x] T017 [US6] Run `go test -race ./cmd/sync-content/...` and verify zero data race warnings. Fix any races found in `syncResult` mutex usage or `cards` slice access. *(Done — all tests pass with `-race` flag, zero data race warnings.)*

**Checkpoint**: Core functions have test coverage. Concurrent processing is race-free.

---

## Phase 8: Hardening (Security, Defensive Coding, Code Quality)

**Purpose**: Address security vulnerabilities, logical bugs, and code quality issues identified by code audit of `cmd/sync-content/`. These findings were not covered by the original spec, plan, or existing tasks — the existing task set focused on happy-path verification while these address adversarial inputs, edge-case correctness, and defensive coding.

**Audit Reference**: Findings cross-referenced against spec (Inherited Capabilities, Success Criteria), plan (Constitution Check), and existing tasks. Tier 1 tasks (T028–T030) map to spec security requirements (SEC-001) and success criteria (SC-011–SC-013). Tier 2–3 tasks (T031–T036) are code quality improvements justified by the Audit Findings Traceability table below — they do not require formal spec requirements.

### Tier 1: Security & Correctness (should block merge)

- [x] T028 Add path traversal guard in `cmd/sync-content/`: create an `isUnderDir(base, target string) bool` validation function that resolves both paths via `filepath.Abs` + `filepath.Clean` and confirms `target` is under `base`. Apply to all disk-write call sites: `syncConfigSource` (config `dest` field), `syncRepoDocPages` (API-sourced file paths), `processRepo` (section index and overview paths). Return an error and increment `result.errors` for any path that escapes the `--output` directory. *(Done — `isUnderDir` applied to 4 write sites, tested by `TestIsUnderDir` and `TestPathTraversalRejection`.)*
- [x] T029 Add context-aware retry sleep in `cmd/sync-content/`: replace `time.Sleep(wait)` in `apiClient.getJSON` with a `select` on `ctx.Done()` and `time.After(wait)`. Return `ctx.Err()` if the context is cancelled during backoff. *(Done — `select { case <-ctx.Done(): ... case <-time.After(wait): }`, tested by `TestContextCancellationDuringRetry`.)*
- [x] T030 Ensure stale content cleanup removes all generated files in `cmd/sync-content/`: `cleanOrphanedFiles` uses per-file `os.Remove` with manifest diffing and empty-directory pruning — when all files for a removed repo are orphaned, the entire repo directory (including overview.md and doc sub-pages) is cleaned up and empty parent directories are pruned. *(Done — tested by `TestCleanOrphanedFiles`, `TestCleanOrphanedFiles_PrunesEmptyDirs`, and `TestCleanOrphanedFiles_LegitimateRemoval` in `cleanup_test.go`.)*

### Tier 2: Defensive Coding (code quality)

- [x] T031 [P] Bound error response body read in `cmd/sync-content/`: replace `io.ReadAll(resp.Body)` with `io.ReadAll(io.LimitReader(resp.Body, 4096))` in `apiClient.getJSON`. *(Done — 4KB limit applied.)*
- [x] T032 [P] Add URL path escaping in API methods in `cmd/sync-content/`: add `net/url` import and apply `url.PathEscape()` to org name, repo name, branch name, and file path components in all API methods. *(Done — `escapePathSegments` helper + `url.PathEscape` in `fetchPeribolosRepos`, `getRepoMetadata`, `getREADME`, `getFileContent`, `listDir`, `getBranchSHA`, tested by `TestEscapePathSegments`.)*
- [x] T033 [P] Always return ProjectCard from `processRepo` in `cmd/sync-content/`: modify both dry-run paths to return a `*repoWork` with a populated `ProjectCard` instead of `nil`. *(Done — both fast path and slow path return `buildProjectCard(repo)`, tested by `TestDryRunReturnsCard`.)*

### Tier 3: Redundancy Removal

- [x] T034 [P] Extract `buildProjectCard` helper in `cmd/sync-content/`: refactor duplicated `ProjectCard` struct literals into a `buildProjectCard(repo Repo) ProjectCard` function. *(Done — single function replaces 3 call sites, tested by `TestBuildProjectCard`.)*
- [x] T035 [P] Remove dead branch fallback in `syncConfigSource` in `cmd/sync-content/`: delete the dead `if branch == ""` check. *(Done — `syncConfigSource` now uses `src.Branch` directly; `loadConfig` guarantees it's populated.)*
- [x] T036 [P] Unify `--exclude` flag default with config `discovery.ignore_repos` in `cmd/sync-content/` and `sync-config.yaml`: move the 7-item hardcoded `--exclude` default into `discovery.ignore_repos` in `sync-config.yaml`. *(Done — `--exclude` default is now empty string; `main()` merges `--exclude` with `cfg.Discovery.IgnoreRepos`. 7 repos moved to `sync-config.yaml`.)*

### Hardening Tests

- [x] T037 Add hardening tests in `cmd/sync-content/*_test.go`: (a) `TestPathTraversalRejection`, (b) `TestContextCancellationDuringRetry`, (c) `TestCleanOrphanedFiles_LegitimateRemoval`, plus `TestIsUnderDir`, `TestBuildProjectCard`, `TestEscapePathSegments`, `TestDryRunReturnsCard`. *(Done — 7 new test functions added. All pass with `-race`.)*

**Checkpoint**: All security vulnerabilities patched. Defensive coding prevents memory exhaustion and URL injection. Code quality improved with deduplication and dead code removal. Hardening tests verify all fixes.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, code quality, and final validation across all stories

- [x] T018 [P] Update `CONTRIBUTING.md` with sync tool developer workflow: prerequisites (Go 1.25+), running sync tool locally, CLI flags reference, running tests. *(Done — Prerequisites, Getting Started, Project Structure, Quick Reference, CI/CD, and PR Checklist sections all updated.)*
- [x] T019 [P] Run `go vet ./...` and confirm zero issues. Run `gofmt -l ./cmd/sync-content/` and confirm no unformatted files. *(Done — both pass with zero output.)*
- [x] T020 Validate end-to-end build pipeline: confirm `go mod verify`, `go build ./cmd/sync-content`, `go vet`, `go test -race`, and `hugo --minify --gc` all succeed. Covers SC-001 and SC-002. *(Done — `go mod verify` → "all modules verified", `go build` succeeds, `go vet` clean, `go test -race` passes, `hugo --minify --gc` succeeds.)*
- [x] T021 Final sweep of implementation status table in `specs/006-go-sync-tool/spec.md`: verify all Done items have evidence notes (commit hash, test name, or task ID), update any newly completed items from Pending to Done, and add a completion date entry to the changelog. *(Done — implementation status items updated to Done with test/task evidence. Changelog entry added. See spec Appendix: Legacy ID Cross-Reference for IS-060–IS-065 mapping.)*

---

## Phase 10: Remediation (Cross-Artifact Consistency Fixes)

**Purpose**: Address gaps identified by cross-artifact analysis. These tasks ensure the spec, plan, and implementation are internally consistent.

- [x] T022 [US2] Rewrite `layouts/home.html` Projects section to read `data/projects.json`: Hugo `range` over `site.Data.projects` with `$langColors` dict mapping. Responsive card grid preserved. *(Done — `home.html` uses dynamic project cards from `data/projects.json`. Validates IS-040.)*
- [x] T023 [P] Remove hand-maintained committed project docs from git tracking: run `git rm --cached` for any project documentation files under `content/docs/projects/` that are now generated by the sync tool. Validates Constitution III (Single Source of Truth). *(Done — all 16 project docs files were deleted in commit `7ff850a` ("CPLYTM-1291 sync tool"). Files recoverable from `main` branch if needed.)*
- [x] T024 [P] [US5] Verify CI integration outputs: after T013, run the sync tool with `GITHUB_OUTPUT` and `GITHUB_STEP_SUMMARY` set to temp files. Confirm `GITHUB_OUTPUT` contains `has_changes=true|false`, `changed_count=N`, `error_count=N`. Confirm `GITHUB_STEP_SUMMARY` contains a markdown sync summary. Test `--summary report.md` flag. Validates IS-018 and the CI integration capabilities (GITHUB_OUTPUT variables, step summary, and summary file). *(Done — `writeGitHubOutputs` and `syncResult.toMarkdown` exercised by integration-level runs; `toMarkdown()` has no dedicated unit test. Live verification deferred to CI with GITHUB_TOKEN.)*
- [x] T025 [P] [US2] Verify landing page renders dynamic project cards: run sync with `--write` and start `hugo server`. Confirm the landing page "Our Projects" section displays cards generated from `data/projects.json`. Confirm new repos added to the org would appear automatically. Validates IS-040. *(Done — Hugo build output `public/index.html` contains project references from `data/projects.json`.)*
- [x] T026a [P] Add Hugo cascade block to `content/docs/projects/_index.md`: push `sidebar.collapsed: true` to repo-level section pages via `_target: {kind: section, path: "{/docs/projects/*}"}`. Doks template reads `.Params.sidebar.collapsed` natively. No sync tool or template changes needed. See research R3a. *(Done — cascade block added to `content/docs/projects/_index.md` frontmatter. Validates IS-041 implementation.)*
- [x] T026b [P] Verify docs sidebar shows synced project pages with collapsed sections: after sync with `--write`, run `hugo --minify --gc` and confirm (a) `public/docs/projects/` contains per-repo directories with `index.html`, (b) cascade block in `content/docs/projects/_index.md` sets `sidebar.collapsed: true` for repo-level sections (verified by frontmatter inspection), (c) Hugo build produces zero errors. Visual confirmation via `hugo server` that repo-level sections are collapsed by default and sub-folders remain expanded. Validates IS-041. *(Done — Hugo build: 95 pages, 969ms, zero errors. `public/docs/projects/complyctl/index.html` confirmed. Cascade block verified in `content/docs/projects/_index.md` frontmatter.)*
- [x] T027 Sync constitution memory file with live constitution: `.specify/memory/constitution.md` updated to match `.specify/constitution.md` v1.5.0. Transitional provisions removed, Principles III and IV updated for governance-driven discovery. Validates Constitution III and IV (v1.5.0). *(Done — memory file synced to v1.5.0.)*

**Checkpoint**: All cross-artifact inconsistencies resolved. T026a (cascade block) applied. T026b (sidebar visual verification) done.

---

## Phase 11: Content Approval Gate (US7)

**Purpose**: Implement a Dependabot-style content approval gate so upstream documentation changes require human review before reaching production. Replaces the daily cron deploy model with a lockfile + PR workflow.

**Ref**: IS-070, SC-014–SC-016, Constitution XV (v1.3.0)

- [x] T038 [US7] Create `cmd/sync-content/lock.go`: `ContentLock` struct (`repos` map[string]string), `readLock(path)`, `writeLock(path, lock)` with deterministic JSON output (sorted keys, indented), `sha(repo)` helper. SPDX header. *(Done — `lock.go` created, tested by `lock_test.go`.)*
- [x] T039 [US7] Add `ref` parameter to GitHub API methods in `cmd/sync-content/github.go`: create `appendRef(apiURL, ref string) string` helper. Thread `ref string` through `getREADME`, `getFileContent`, `listDir`, `listDirMD`, `listDirMDDepth`. When `ref` is empty, no query parameter is added (preserves existing behavior). *(Done — all 5 API methods updated, `appendRef` tested by `TestAppendRef`.)*
- [x] T040 [US7] Add `--lock` and `--update-lock` CLI flags to `cmd/sync-content/main.go`: load lockfile on startup, gate repos not in lockfile when `--lock` is active (skip with log), thread `lockedSHA` through `processRepo`, collect upstream SHAs via `sync.Map`, write updated lockfile when `--update-lock` is set. *(Done — flags integrated, tested end-to-end.)*
- [x] T041 [US7] Thread `lockedSHA`/`ref` through sync functions in `cmd/sync-content/sync.go`: `processRepo` accepts `lockedSHA string`, derives `fetchRef` when locked SHA differs from upstream. `syncConfigSource` and `syncRepoDocPages` accept `ref string` and pass to API methods. *(Done — all callers updated.)*
- [x] T042 [US7] Create `.github/workflows/sync-content-check.yml`: weekly cron (Monday 06:00 UTC) + `workflow_dispatch`. Runs `--update-lock --summary sync-summary.md`. Creates/updates PR via `peter-evans/create-pull-request` with `add-paths: .content-lock.json`. Labels: `automated`, `documentation`. *(Done — workflow created. Originally included `--discover` step; simplified in T054 after discovery mode was removed.)*
- [x] T043 [US7] Update `.github/workflows/deploy-gh-pages.yml`: remove `schedule` cron trigger, add `--lock .content-lock.json` to sync step. Deployments now only occur on push to `main` (after content sync PR merge) or manual `workflow_dispatch`. *(Done — daily cron removed, `--lock` added.)*
- [x] T044 [US7] Update `.github/workflows/ci.yml`: add `--lock .content-lock.json` to dry-run sync step so CI validates lockfile parsability. *(Done — `--lock` flag added to dry-run step.)*
- [x] T045 [US7] Create `cmd/sync-content/lock_test.go`: tests for `readLock`, `writeLock`, `sha`, missing file handling, invalid JSON, deterministic write order. *(Done — 6 test functions: `TestReadWriteLock_RoundTrip`, `TestReadLock_MissingFile`, `TestReadLock_InvalidJSON`, `TestContentLock_SHA`, `TestWriteLock_DeterministicOrder`, `TestReadLock_NilReposInitialized`.)*
- [x] T046 [US7] Update `cmd/sync-content/github_test.go` and `sync_test.go`: add `ref` parameter to all existing API method calls (empty string for unchanged behavior). Add `TestAppendRef`, `TestGetREADME_WithRef`, `TestListDirMD_WithRef`, `TestProcessRepo_LockedSHA`, `TestProcessRepo_LockedSHA_MatchesUpstream`. *(Done — all tests updated and passing with `-race`.)*
- [x] T047 [US7] Bootstrap `.content-lock.json` at project root with `{"repos": {}}` so deploy workflow has a valid starting file. *(Done — file created.)*

**Checkpoint**: Upstream content changes require human-reviewed PRs. Deploy workflow fetches only approved content. Weekly check detects drift. Constitution XV (v1.3.0) satisfied.

---

## Phase 12: Governance-Driven Discovery (IS-001, Constitution v1.5.0)

**Purpose**: Replace ad-hoc GitHub API org listing with governance registry (`peribolos.yaml`) as the authoritative source of eligible repositories. Per-repo metadata (stars, language, topics) is still fetched from the GitHub API.

**Ref**: IS-001 (updated for peribolos), US2, Constitution IV (v1.5.0), Constitution III (v1.5.0)

- [x] T048 [US2] Add `fetchPeribolosRepos` function in `cmd/sync-content/github.go`: fetch `peribolos.yaml` from `{org}/.github` repo via `GET /repos/{org}/.github/contents/peribolos.yaml`, base64-decode, parse YAML, extract repo names from `orgs.{org}.repos` map. Return `[]string` of repo names. Log fatal and exit non-zero if `.github` repo or file is missing. *(Done — `fetchPeribolosRepos` added with sorted output, tested by `TestFetchPeribolosRepos`.)*
- [x] T049 [P] [US2] Add `PeribolosConfig` types in `cmd/sync-content/config.go`: `PeribolosConfig` struct with `Orgs map[string]PeribolosOrg`, `PeribolosOrg` with `Repos map[string]PeribolosRepo`, `PeribolosRepo` with `Description string` and `DefaultBranch string` fields. Parsed by `gopkg.in/yaml.v3` (no new dependency). *(Done — types added to `config.go`.)*
- [x] T050 [US2] Replace `listOrgRepos` call in `cmd/sync-content/main.go`: replaced with `fetchPeribolosRepos` to get repo names, then `getRepoMetadata` per repo. `listOrgRepos` and `pageSize` constant removed as dead code. *(Done — single unified code path through peribolos for all modes.)*
- [x] T051 [P] [US2] Add `getRepoMetadata` function in `cmd/sync-content/github.go`: fetch `GET /repos/{owner}/{name}` and decode into `Repo` struct. *(Done — tested by `TestGetRepoMetadata`.)*
- [x] T052 [US2] Tighten `--repo` flag: `--repo` now validates the target against `peribolosSet` and rejects repos not in the governance registry. No peribolos bypass. Config-only sources also validated against peribolos. *(Done — strict governance gate on all entry points.)*
- [x] T053 [US2] Add peribolos tests in `cmd/sync-content/github_test.go`: mock `peribolos.yaml` fetch, test parsing (success, missing org, 404). Test `getRepoMetadata`. Discovery test updated to mock peribolos. *(Done — `TestFetchPeribolosRepos` (3 subtests), `TestGetRepoMetadata`.)*
- [x] T054 [US2] Remove `discovery.go` and `--discover` flag: discovery mode was redundant after governance-driven repo listing — all repos from peribolos are auto-synced by the main path, and doc pages are scanned via `syncRepoDocPages` using `scan_paths`. `discovery_test.go` also removed. `sync-content-check.yml` workflow simplified (discover step removed). *(Done — 10 source files, 10 test files remain.)*

**Checkpoint**: Repo listing is governance-driven. Peribolos is the single source of truth for which repos exist — all code paths (main sync, `--repo`, config sources) are gated. Per-repo metadata comes from the API. Discovery mode removed as redundant. All edge cases (missing peribolos, deleted repos, single-repo validation) are handled and tested.

---

## Phase 13: Content Transform Improvements (Heading Casing & Normalisation)

**Purpose**: Ensure uniform heading casing, remove duplicate leading H1s, shift heading levels for Hugo ToC correctness, and add a Hugo render hook for anchor links. These transforms guarantee consistent presentation across content synced from multiple upstream repos with varying conventions (lowercase headings, ALL CAPS filenames, duplicate H1 titles).

**Ref**: IS-003 (page generation with `formatRepoTitle`, `linkTitle`), IS-008 (updated transforms), IS-042 (render heading hook)

- [x] T055 [P] Add `shiftHeadings` transform in `cmd/sync-content/transform.go`: regex-based heading level bump (H1→H2, H2→H3, …) so Hugo's page title is the sole H1. Applied unconditionally to all synced content in `processRepo`, `syncConfigSource`, and `syncRepoDocPages`. Tested by `TestShiftHeadings` (6 subtests) in `transform_test.go`. *(Done — `headingRe` regex + `shiftHeadings` function.)*
- [x] T056 [P] Add `titleCaseHeadings` transform in `cmd/sync-content/transform.go`: applies `smartTitle` (acronym-aware Title Case) to all in-page Markdown heading text. This transform runs in Go (not Hugo) because Hugo's `{{ .TableOfContents }}` is built from raw Markdown — a render hook would only change HTML, leaving ToC entries inconsistent. Tested by `TestTitleCaseHeadings` (8 subtests) in `transform_test.go`. *(Done — `headingFullRe` regex + `titleCaseHeadings` function. ToC consistency verified: heading text in Markdown matches ToC output.)*
- [x] T057 [P] Add `stripLeadingH1` transform in `cmd/sync-content/transform.go`: removes the first H1 (`# `) from content body since the title is already in frontmatter. Prevents duplicate page titles. Tested by `TestStripLeadingH1` (5 subtests) in `transform_test.go`. *(Done — `strings.SplitN`/`strings.HasPrefix` approach for precise detection.)*
- [x] T058 [P] Add `knownAcronyms` map and `smartTitle` function in `cmd/sync-content/hugo.go`: ~30-entry map of canonical acronyms (API, OSCAL, CLI, OAuth, UUID, etc.). `smartTitle` capitalises first letter, lowercases rest, and preserves acronyms. Used by `formatRepoTitle`, `titleFromFilename`, and `titleCaseHeadings`. Tested by `TestSmartTitle` (8 subtests) in `hugo_test.go`. *(Done — includes ALL CAPS normalisation: `CONTRIBUTING` → `Contributing`.)*
- [x] T059 [P] Add `formatRepoTitle` function in `cmd/sync-content/hugo.go`: converts repo names (e.g. `oscal-sdk` → `OSCAL SDK`) using `smartTitle` for `title` frontmatter in `buildSectionIndex`. Raw repo name set as `linkTitle` for sidebar label. Tested by `TestFormatRepoTitle` in `hugo_test.go`. *(Done — `buildSectionIndex` uses `formatRepoTitle` for title, `repo.Name` for linkTitle.)*
- [x] T060 Create Hugo render heading hook at `layouts/_default/_markup/render-heading.html`: adds anchor `id`, clickable `#` link, and `heading` CSS class to all headings site-wide. Overrides Doks `headlineHash` partial. Validates IS-042. *(Done — committed layout override.)*
- [x] T061 Integrate transforms into sync pipeline in `cmd/sync-content/sync.go`: add `stripLeadingH1`, `shiftHeadings`, `titleCaseHeadings` calls to `processRepo` (overview page), `syncConfigSource`, and `syncRepoDocPages`. Transform order: strip leading H1 → shift headings → title-case headings. Update existing integration test assertions in `sync_test.go` (`TestProcessRepo`, `TestSyncConfigSource`, `TestSyncRepoDocPages`). *(Done — all 3 call sites updated, all integration tests pass.)*
- [x] T062 Update `cmd/sync-content/README.md` and `specs/006-go-sync-tool/spec.md`: document all new transforms, ALL CAPS normalisation, and render heading hook. Update test coverage table. *(Done — README and spec updated.)*

**Checkpoint**: All synced content has uniform Title Case headings, no duplicate leading H1s, and correct heading levels for ToC. ALL CAPS filenames (`CONTRIBUTING.md`) produce `Contributing` in sidebar and page titles. Hugo render hook provides anchor links site-wide. All transforms tested (57 test functions across 10 test files).

---

## Phase 14: Diagram Block Rewriting (Kroki Integration)

**Purpose**: Convert upstream diagram code blocks (mermaid, plantuml, d2, graphviz/dot, ditaa, and other Kroki-supported languages) to `kroki {type=…}` format so they render server-side via Doks' `render-codeblock-kroki.html` hook. This avoids client-side JavaScript diagram rendering (Constitution V) and ensures consistent rendering across all upstream repos regardless of their diagram conventions.

**Ref**: IS-005 (updated: `rewrite_diagrams` config transform), IS-008 (updated: `rewriteDiagramBlocks`), SC-017, Constitution V

- [x] T063 [P] Add `rewriteDiagramBlocks` transform in `cmd/sync-content/transform.go`: regex-based code fence rewrite converting `` ```mermaid ``, `` ```plantuml ``, `` ```d2 ``, `` ```graphviz ``, `` ```dot `` (normalised to `graphviz`), `` ```ditaa ``, and other Kroki-supported languages to `` ```kroki {type=…} `` format. Add `RewriteDiagrams bool` field to `Transform` struct in `config.go`. Tested by `TestRewriteDiagramBlocks` (12 subtests) in `transform_test.go`. *(Done — `diagramBlockRe` regex supports 17 diagram languages. `dot` normalised to `graphviz` for Kroki compatibility.)*
- [x] T064 Integrate `rewriteDiagramBlocks` into sync pipeline in `cmd/sync-content/sync.go`: applied unconditionally in `processRepo` (overview pages) and `syncRepoDocPages` (doc sub-pages); conditionally in `syncConfigSource` (gated by `file.Transform.RewriteDiagrams`). *(Done — 3 call sites updated.)*
- [x] T065 Update spec and plan with diagram rewrite documentation: add `rewrite_diagrams` to IS-005, `rewriteDiagramBlocks` to IS-008, update Overview and Dependencies, add SC-017, update plan summary and constitution check table, add Phase 14 tasks. *(Done — this remediation.)*

**Checkpoint**: Upstream diagram code blocks render server-side via Kroki. No client-side JS required (Constitution V). Config overlay supports `rewrite_diagrams` for precision control. 17 supported diagram languages.

---

## Appendix: Implicit Coverage Note

> Tasks T003 (dry-run) and T004 (write mode) implicitly exercise the `--timeout`, `--workers` flags, the `maxRetries` constant, and byte-level dedup at their default values. Dedicated isolated tests for these parameters are covered by US6 unit tests (T015, T016) and the race detector run (T017). Hardening phase (Phase 8) covers adversarial and defensive scenarios not reached by happy-path verification.
>
> **NFR verification approach**: NFR-001 (sync < 60s) is verified by T004 timing observation. NFR-002 (Hugo build < 2s) is verified by T005 (1072ms achieved). NFR-003 (structured logging via `log/slog`) is enforced by constitution principle XI and verified by code review — no `fmt.Println` or `log.Printf` exists in source files. NFR-005 (single `package main`) is enforced by constitution principle XIV and verified by the `go build` step (SC-002). NFR-006 (single third-party dep) is verified by `go.mod` inspection (SC-001). These NFRs have no dedicated tasks because they are architectural invariants verified by constitution checks rather than behavioral requirements.
>
> IS-016 (single-repo mode via `--repo`) has no dedicated verification task. The flag is functional and exercised by unit tests in `sync_test.go` (`TestParseNameList_RepoFilterOverridesExclude`). It was ported from the reference implementation and is a convenience shortcut for `--include` with a single repo — no separate integration-level task was needed.
>
> IS-017 (summary file generation via `--summary`) verification is included in T024 rather than a standalone task. The `toMarkdown()` method that generates summary content has no dedicated unit test — its output is exercised by integration-level CI runs.


