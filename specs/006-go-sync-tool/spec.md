# Feature Specification: Go Content Sync Tool

**Feature Branch**: `006-go-sync-tool`
**Phase**: 2 (Content Infrastructure)

## Overview

The ComplyTime website (`complytime.dev`) documents a growing ecosystem of open-source compliance tools hosted across multiple repositories in the `complytime` GitHub organization. Before this feature, project documentation was manually copied into the site — error-prone, inconsistent, and unable to scale as new repos were added.

This feature replaces that workflow with a Go CLI tool (`cmd/sync-content/`, ~2,100 lines across 10 source files in `package main`) that derives the set of eligible repositories from the org's governance registry (`peribolos.yaml` in the `.github` repo), fetches their README content and per-repo metadata via the GitHub REST API, applies Markdown transforms (heading level shifting, Title Case normalisation with acronym awareness and ALL CAPS normalisation, badge stripping, relative link rewriting), and generates Hugo-compatible pages and landing page card data. A declarative config overlay (`sync-config.yaml`) provides precision control for repos needing custom documentation layouts.

**Dependencies**: Go 1.25+, `gopkg.in/yaml.v3` (sole third-party Go dep), Hugo 0.155.1 extended, Node.js 22.

## Scope

### In Scope

> IDs are grouped by domain (001–018: core, 030–031: detection, 040–041: site integration, 070: content approval). Gaps between groups are intentional.

| ID | Capability |
|----|-----------|
| IS-001 | Governance-driven repo discovery: fetch `peribolos.yaml` from `{org}/.github` repo, parse `orgs.{org}.repos` map as authoritative repo list, enrich with GitHub API metadata (stars, language, topics) per repo |
| IS-002 | README fetch with base64 decoding and SHA tracking |
| IS-003 | Per-repo page generation: section index (`_index.md`, frontmatter only, with `formatRepoTitle` for `title` and raw repo name as `linkTitle` for sidebar; ALL CAPS repo/file names normalised to Title Case) + overview page (`overview.md`, README content) |
| IS-004 | Landing page card generation (`data/projects.json`) with type derivation from topics |
| IS-005 | Config-driven file sync with transforms (`inject_frontmatter`, `rewrite_links`, `strip_badges`); heading shift and Title Case applied unconditionally to all synced content |
| IS-006 | Concurrent processing with bounded worker pool (`--workers`) |
| IS-007 | Dry-run by default; `--write` flag required for disk I/O |
| IS-008 | Markdown transforms: `stripLeadingH1` (removes leading H1 — title already in frontmatter), `shiftHeadings` (H1→H2, H2→H3, …), `titleCaseHeadings` (acronym-aware Title Case for in-page headings and TOC; normalises ALL CAPS words to Title Case while preserving known acronyms from the `knownAcronyms` map in `hugo.go` — ~30 domain terms; maintainers add entries as new projects introduce terminology), `stripBadges`, `rewriteRelativeLinks` |
| IS-009 | Repo filtering: exclude archived repos, forks, `--include`/`--exclude` lists |
| IS-012 | Sync manifest (`.sync-manifest.json`) for orphan file tracking |
| IS-014 | Doc page auto-sync from `discovery.scan_paths` directories |
| IS-016 | Single-repo mode (`--repo`): sync only one repository (validated against peribolos) |
| IS-017 | Summary file generation (`--summary report.md`) |
| IS-018 | GitHub CI outputs: `GITHUB_OUTPUT` variables and `GITHUB_STEP_SUMMARY` |
| IS-030 | Two-tier SHA-based change detection (branch SHA + README SHA) |
| IS-031 | Stale content cleanup via manifest diff (`cleanOrphanedFiles`); legacy directory-scan fallback removed |
| IS-040 | Dynamic landing page project cards from `data/projects.json` |
| IS-041 | Docs sidebar with collapsed repo-level sections via Hugo cascade |
| IS-042 | Hugo render heading hook (`render-heading.html`): adds anchor `id`, clickable `#` link, and `heading` CSS class to all headings site-wide |
| IS-070 | Content lockfile (`.content-lock.json`) for SHA-pinned content approval |

### Out of Scope

- `.specify/` artifact sync (fetching upstream `constitution.md`, `spec.md`, `plan.md` into site) — deferred to a future feature
- Private repository access
- GitHub Enterprise / custom API URL
- Log level control (`--verbose` / `--quiet`)
- Config schema versioning

### Edge Cases (Peribolos Integration)

| Case | Expected Behavior |
|------|-------------------|
| Repo in peribolos but archived on GitHub | Excluded by existing archived-repo filter (fetched from API metadata) |
| Repo in peribolos but deleted on GitHub | API metadata fetch returns 404; log warning, skip repo, continue |
| Repo on GitHub but NOT in peribolos | Excluded — governance registry is authoritative |
| `.github` repo missing or peribolos.yaml absent | Fatal error — log and exit non-zero |
| `--org` flag value doesn't match peribolos `orgs` key | Fatal error — log mismatch and exit non-zero |
| `--repo` flag used (single-repo mode) | Validated against peribolos — repo must exist in governance registry; metadata fetched from API |

## User Stories

### US1: Safe Local Preview (Priority: P1) — MVP

**As a** contributor, **I want to** clone the repo, run the sync tool, and preview the full site locally, **so that** I can verify documentation changes without risk.

**Acceptance Scenarios**:
- **US1-SC1**: Running without `--write` creates zero files. Tool logs intended actions.
- **US1-SC2**: Running with `--write` generates: (a) section indexes at `content/docs/projects/{repo}/_index.md` (frontmatter only), (b) overview pages at `content/docs/projects/{repo}/overview.md` (README content), (c) doc sub-pages from `discovery.scan_paths`, (d) `data/projects.json`, (e) `.sync-manifest.json`.
- **US1-SC3**: `hugo server` after sync produces zero build errors. Pages accessible at `/docs/projects/`.

### US2: Governance-Driven Discovery (Priority: P1)

**As a** site maintainer, **I want** repos declared in the org's governance registry to automatically appear on the website, **so that** the site reflects the org's official repo list without ad-hoc API discovery.

**Acceptance Scenarios**:
- **US2-SC1**: Repos listed in `peribolos.yaml` (and NOT in `sync-config.yaml`) produce: (a) `_index.md` with frontmatter (`title` via `formatRepoTitle`, `linkTitle` with raw repo name, `description`, `params.language`, `params.stars`, `params.source_sha`, `params.readme_sha`, `params.seo.*`) and no body, (b) `overview.md` with transformed README content (headings shifted and Title Cased).
- **US2-SC2**: `data/projects.json` contains a `ProjectCard` for every eligible repo from peribolos (non-archived, non-forked), sorted alphabetically, with fields `name`, `language`, `type`, `description`, `url`, `repo`, `stars`.
- **US2-SC3**: Repos present on GitHub but NOT in `peribolos.yaml` are excluded from sync (governance registry is authoritative).
- **US2-SC4**: If `peribolos.yaml` cannot be fetched (e.g., `.github` repo missing or network error), the tool logs an error and exits non-zero rather than silently falling back to API listing.

> **Note**: When `--lock` is active (production deploys), new repos not yet in `.content-lock.json` are skipped until the next content sync check PR (US7) adds them to the lockfile and is merged. The approval gate controls when they reach production.

### US3: Config-Driven Precision Sync (Priority: P1)

**As a** documentation lead, **I want** precise control over specific files' destinations, frontmatter, and transforms, **so that** key projects have customized documentation layouts.

**Acceptance Scenarios**:
- **US3-SC1**: For repos with `skip_org_sync: true`, no auto-generated section index or overview page exists, BUT the repo's `ProjectCard` is in `data/projects.json`.
- **US3-SC2**: Config-declared files appear at their `dest` paths with correct transforms applied.

### US4: Change Detection and Stale Cleanup (Priority: P2)

**As a** CI pipeline, **I want** the sync tool to skip unchanged repos and clean up stale content, **so that** builds are fast and the site stays clean.

**Acceptance Scenarios**:
- **US4-SC1**: On a second consecutive run, unchanged repos show "unchanged" in log output with zero disk writes.
- **US4-SC2**: When a repo is removed from the org, all generated files (section index, overview, doc sub-pages, entire directory) are cleaned up.

### US5: CI/CD Pipeline Integration (Priority: P2)

**As a** DevOps engineer, **I want** the sync tool to run automatically in GitHub Actions, **so that** production deploys always use reviewed, approved content.

**Acceptance Scenarios**:
- **US5-SC1**: Deploy workflow includes Go setup, sync step with `GITHUB_TOKEN` and `--lock`, runs before Hugo build. Content is fetched at approved SHAs from `.content-lock.json`.
- **US5-SC2**: CI workflow validates PRs with `go test -race`, content sync (with `--lock`), and Hugo build. Deploy workflow additionally runs `go vet` and `gofmt` checks.
- **US5-SC3**: `GITHUB_OUTPUT` contains `has_changes`, `changed_count`, `error_count`. `GITHUB_STEP_SUMMARY` contains a markdown summary. CI deploys proceed even with non-fatal warnings.

### US6: Concurrent Processing with Race Safety (Priority: P3)

**As a** developer, **I want** the tool to process repos concurrently and pass race detection, **so that** processing is fast and correct.

**Acceptance Scenarios**:
- **US6-SC1**: `go test -race ./cmd/sync-content/...` passes with zero data race warnings.
- **US6-SC2**: Unit tests cover all pure functions; integration tests verify end-to-end processing with mock API.

### US7: Content Approval Gate (Priority: P2)

**As a** site maintainer, **I want** upstream documentation changes to require human review before reaching production, **so that** broken or undesirable content never deploys automatically.

**Acceptance Scenarios**:
- **US7-SC1**: A committed `.content-lock.json` pins each repo to an approved branch SHA. The deploy workflow fetches content at those locked SHAs — not HEAD.
- **US7-SC2**: A weekly check workflow detects upstream changes, updates `.content-lock.json`, and opens a PR. No content change reaches production without a merged PR.
- **US7-SC3**: Running with `--lock` and a repo not in the lockfile skips that repo (unapproved content is not fetched).
- **US7-SC4**: Running with `--lock --update-lock` writes current upstream SHAs to the lockfile for all discovered repos.

## CLI Interface

| Flag | Default | Description |
|------|---------|-------------|
| `--org` | `complytime` | GitHub organization — used to locate `peribolos.yaml` in `{org}/.github` and as the `orgs` key for repo extraction |
| `--token` | `$GITHUB_TOKEN` | GitHub API token (or set env var) |
| `--config` | (none) | Path to `sync-config.yaml` for config-driven file syncs |
| `--write` | `false` | Required to write files to disk (default: dry-run) |
| `--output` | `.` | Hugo site root directory |
| `--workers` | `5` | Max concurrent repo processing goroutines |
| `--timeout` | `3m` | Overall timeout for all API operations |
| `--include` | (all) | Comma-separated repo allowlist |
| `--exclude` | (see config) | Comma-separated repo names to skip |
| `--repo` | (none) | Sync only this repo (e.g., `complytime/complyctl`); validated against peribolos |
| `--summary` | (none) | Write markdown change summary to this file |
| `--lock` | (none) | Path to `.content-lock.json` for content approval gating |
| `--update-lock` | `false` | Write current upstream SHAs to the lockfile (requires `--lock`) |

## Output Structure

```text
content/docs/projects/
├── _index.md                 # Hand-maintained section index (committed)
└── {repo}/                   # Generated per-repo content (gitignored)
    ├── _index.md             # Section index — frontmatter only, no body
    ├── overview.md           # README content as child page (weight: 1)
    └── {doc}.md              # Doc pages from discovery.scan_paths

data/
└── projects.json             # Landing page project cards (gitignored)

layouts/_default/_markup/
└── render-heading.html       # Hugo render hook — anchor links and heading class (committed)

.sync-manifest.json           # Written file manifest for orphan cleanup (gitignored)
.content-lock.json            # Approved upstream SHAs per repo (committed)
```

## Non-Functional Requirements

| ID | Requirement | Target |
|----|------------|--------|
| NFR-001 | Full org sync completes within timeout | < 60s with token |
| NFR-002 | Hugo build time with generated content | < 2s |
| NFR-003 | All logging via `log/slog` with structured fields | — |
| NFR-004 | SPDX license headers on all Go source files | — |
| NFR-005 | All code in `package main` within `cmd/sync-content/`; no unnecessary packages or abstractions | — |
| NFR-006 | Only permitted third-party dep: `gopkg.in/yaml.v3` | — |
| NFR-007 | Generated content gitignored, not committed | — |
| NFR-008 | Idempotent runs: same input produces same output | — |

## Security Requirements

| ID | Requirement | Task |
|----|------------|------|
| SEC-001 | Path traversal prevention: all write paths validated under `--output` directory | T028 |
| SEC-002 | Bounded error response body reads (4KB max) to prevent memory exhaustion | T031 |
| SEC-003 | URL path escaping for all API URL construction to prevent injection | T032 |

## Inherited Capabilities

The following capabilities were ported from the test-website reference implementation and are functional:

- **Two-tier SHA-based change detection**: Branch SHA (`params.source_sha`) for fast pre-filtering; README SHA (`params.readme_sha`) for content-level accuracy
- **Single-repo filtering** (`--repo`): Process one repo (validated against peribolos governance registry)
- **Doc page auto-sync**: Syncs Markdown files from `discovery.scan_paths` directories
- **Context cancellation**: `--timeout` flag with context propagation; retry sleep respects cancellation (T029)
- **CI integration outputs**: Writes `GITHUB_OUTPUT` variables and `GITHUB_STEP_SUMMARY` for GitHub Actions; deploys proceed even with non-fatal warnings
- **Content approval gate** (`--lock`): SHA-pinned lockfile gates deployments to reviewed content; weekly check workflow proposes updates via PR

## Success Criteria

All criteria must pass before feature 006 merges to `main`.

| ID | Criterion | Verification |
|----|----------|--------------|
| SC-001 | `go.mod` exists and `go mod verify` passes | `go mod verify` |
| SC-002 | `cmd/sync-content/` compiles without errors | `go build ./cmd/sync-content` |
| SC-003 | Dry-run produces zero files; write mode produces correct output structure | T003, T004 |
| SC-004 | Hugo builds with zero errors after sync | T005 |
| SC-005 | Auto-discovered repos have section index + overview + card | T006, T007 |
| SC-006 | Config overlay applies transforms at declared dest paths | T008, T009 (deferred until sources declared; code paths covered by unit tests) |
| SC-007 | Change detection skips unchanged repos; stale cleanup removes all files | T010 |
| SC-008 | Unit and integration tests pass | T015, T016 |
| SC-009 | `go vet` and `gofmt` pass with zero issues | T019 |
| SC-010 | CI workflow validates PRs with test, sync, build; deploy workflow adds vet/gofmt | T014 |
| SC-011 | Path traversal prevention rejects paths escaping `--output` directory | T028, T037 |
| SC-012 | Context-aware retry sleep respects cancellation promptly | T029, T037 |
| SC-013 | Stale cleanup removes all generated files (overview.md, doc sub-pages), not just `_index.md` | T030, T037 |
| SC-014 | `--lock` gates content to approved SHAs; unapproved repos are skipped | `lock_test.go`, `sync_test.go` (`TestProcessRepo_LockedSHA`) |
| SC-015 | `--update-lock` writes current upstream SHAs to lockfile | `lock_test.go` (`TestWriteLock`, `TestWriteLock_DeterministicOrder`) |
| SC-016 | Weekly check workflow creates/updates a PR with lockfile changes | `sync-content-check.yml` manual dispatch |

## Merge Readiness Gate

All 16 success criteria (SC-001 through SC-016) MUST pass before merging feature 006 to `main`. SC-006 is deferred (blocked on config sources being declared) but its code paths are covered by unit tests (`TestSyncConfigSource`, `TestProcessRepo`). SC-016 requires a manual `workflow_dispatch` run of `sync-content-check.yml` after merge.

## Appendix: Legacy ID Cross-Reference

The In Scope table above uses consolidated IDs. Earlier development phases used a more granular implementation status table with additional IDs. Tasks in `tasks.md` reference some of these legacy IDs. This table maps them to their current equivalents for traceability.

| Legacy ID | Current Mapping | Context |
|-----------|----------------|---------|
| IS-010 | NFR-007 | Gitignore patterns for generated repo pages |
| IS-011 | NFR-007 | Gitignore patterns for landing page cards |
| IS-032 | SC-008 | Unit test requirement |
| IS-050 | Constitution III | Remove hand-maintained committed project docs |
| IS-051 | IS-018 | CI integration outputs (GITHUB_OUTPUT, step summary) |
| IS-052 | Constitution III, IV | Constitution memory file sync to v1.5.0 |
| IS-060–IS-065 | — | Implementation status tracking items (historical; used during T021 final sweep) |
| IS-061 | Inherited Capabilities | Context cancellation in retry sleep (referenced in spec Inherited Capabilities section) |
| IS-071 | IS-070 | Content lockfile — `readLock`/`writeLock` implementation |
| IS-072 | IS-070 | Content lockfile — `ref` parameter threading through API methods |
| IS-073 | IS-001 | Governance-driven discovery via `peribolos.yaml` (Constitution v1.5.0 update) |
