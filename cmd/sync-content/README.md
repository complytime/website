# sync-content

A Go CLI tool that pulls documentation from upstream GitHub repositories into the
ComplyTime website's Hugo content tree. It reads the org's governance registry
(`peribolos.yaml` in the `.github` repo) to determine which repositories exist,
enriches each with GitHub API metadata, generates per-project documentation pages
and landing-page card data, then layers precise config-driven file syncs on top.

**No generated content is committed to git.** The tool runs at build time (in CI)
or on-demand (locally) to populate the site. This keeps the repository lean and
ensures documentation is always sourced from upstream.

## How It Works

The tool operates in **hybrid mode** with two complementary phases:

### Phase 1: Governance-Driven Discovery (automatic)

Fetches `peribolos.yaml` from the org's `.github` repo to get the authoritative
list of repositories, then enriches each with metadata from the GitHub REST API.
For each eligible (non-archived, non-fork) repo it:

1. Fetches the README and branch HEAD SHA.
2. Generates two Hugo pages per project:
   - `content/docs/projects/{repo}/_index.md` — a section index with metadata
     frontmatter (title, description, dates, language, stars, SEO metadata,
     source/README SHAs). Contains no body content; the Doks sidebar renders
     this as a collapsible section heading.
   - `content/docs/projects/{repo}/overview.md` — the README content as a
     navigable child page with edit URL.
3. Normalises casing: ALL CAPS filenames (e.g. `CONTRIBUTING.md`) and headings become Title Case (`Contributing`); known acronyms (API, OSCAL, CLI, …) are preserved.
4. Shifts all Markdown headings down one level (H1→H2, H2→H3, …) so Hugo's page title is the sole H1.
5. Strips CI badge lines from the top of the README.
6. Rewrites relative Markdown links and images to absolute GitHub URLs.
7. Scans for doc pages under configurable `scan_paths` (e.g. `docs/`).
8. Builds a `ProjectCard` for the landing page.

After processing all repos, the tool writes `data/projects.json` — an array of
`ProjectCard` objects that Hugo templates use to render the "Our Projects" section.

### Phase 2: Config Sync (opt-in)

Reads `sync-config.yaml` and pulls specific files with per-file transforms:

- **Frontmatter injection** — prepend YAML frontmatter with arbitrary key-value
  pairs, or replace existing frontmatter.
- **Link rewriting** — convert relative Markdown links to absolute GitHub blob
  URLs and relative images to raw.githubusercontent URLs.
- **Badge stripping** — remove CI/status badge lines from the top of content.

Config sources can operate alongside or instead of the org scan per-repo:

| `skip_org_sync` | Org scan page | Config files | ProjectCard |
|-----------------|---------------|--------------|-------------|
| `false` (default) | Generated from README | Synced as additional content | Yes |
| `true` | Suppressed | Synced as primary content | Yes |

## Quick Start

### Prerequisites

- **Go 1.25+** — the sync tool is pure Go with one dependency (`gopkg.in/yaml.v3`)
- **Node.js 22+** — for the Hugo/Doks theme build (`npm ci`)
- **Hugo extended** — the static site generator
- **`GITHUB_TOKEN`** (recommended) — unauthenticated rate limit is 60 requests/hour

### 1. Dry-run (preview without writing)

```bash
go run ./cmd/sync-content --org complytime --config sync-config.yaml
```

Logs every action the tool would take but creates zero files. This is the default
mode — you must explicitly opt in to writes.

### 2. Write mode (generate content)

```bash
go run ./cmd/sync-content --org complytime --config sync-config.yaml --write
```

Produces:

| Output | Path |
|--------|------|
| Per-repo section index | `content/docs/projects/{repo}/_index.md` |
| Per-repo README page | `content/docs/projects/{repo}/overview.md` |
| Auto-discovered doc pages | `content/docs/projects/{repo}/*.md` |
| Landing page card data | `data/projects.json` |
| Sync manifest | `.sync-manifest.json` |
| Content lockfile (with `--update-lock`) | `.content-lock.json` |

### 3. Start Hugo

```bash
npm run dev
```

Navigate to `http://localhost:1313/`. Project pages appear at `/docs/projects/`.

### 4. Build for production

```bash
# Local dev (fetches HEAD):
go run ./cmd/sync-content --org complytime --config sync-config.yaml --write

# Production (fetches at approved SHAs):
go run ./cmd/sync-content --org complytime --config sync-config.yaml --lock .content-lock.json --write

hugo --minify --gc
```

Output is in `public/`. The `--lock` flag ensures content matches the approved
SHAs in `.content-lock.json`. Omit it for local development to fetch latest HEAD.

## CLI Reference

| Flag | Default | Description |
|------|---------|-------------|
| `--org` | `complytime` | GitHub organization (reads `peribolos.yaml` from `{org}/.github` repo) |
| `--token` | `$GITHUB_TOKEN` | GitHub API token (or set the env var) |
| `--config` | _(none)_ | Path to `sync-config.yaml` for config-driven file syncs |
| `--write` | `false` | Apply changes to disk (without this flag, everything is a dry-run) |
| `--output` | `.` | Hugo site root directory |
| `--repo` | _(none)_ | Sync only this repo, e.g. `complytime/complyctl` |
| `--include` | _(all)_ | Comma-separated repo allowlist (empty = all eligible repos) |
| `--exclude` | _(see config)_ | Comma-separated repo names to skip; merged with `discovery.ignore_repos` in `sync-config.yaml` |
| `--workers` | `5` | Maximum concurrent repo processing goroutines |
| `--timeout` | `3m` | Overall timeout for all API operations |
| `--summary` | _(none)_ | Write a Markdown change summary to this file (for PR bodies) |
| `--lock` | _(none)_ | Path to `.content-lock.json` for content approval gating |
| `--update-lock` | `false` | Write current upstream SHAs to the lockfile (requires `--lock`) |

## Common Tasks

### Sync a single repository

```bash
go run ./cmd/sync-content --repo complytime/complyctl --config sync-config.yaml --write
```

### Generate a change summary for PR review

```bash
go run ./cmd/sync-content --org complytime --config sync-config.yaml --write \
  --summary sync-report.md
```

The summary file contains a Markdown report with new/updated/removed repos and
stats.

### Increase concurrency for faster syncs

```bash
go run ./cmd/sync-content --org complytime --workers 10 --write
```

## Configuration

The config file `sync-config.yaml` lives at the repository root. It has three
sections:

### `defaults`

Fallback values applied to every source unless overridden.

```yaml
defaults:
  branch: main
```

### `discovery`

Controls repo filtering and automatic doc page scanning.

```yaml
discovery:
  ignore_repos:
    - .github                 # repo names to exclude from sync
    - website
  scan_paths:
    - docs                    # directories to scan for .md files
  ignore_files:
    - CHANGELOG.md            # filenames to skip during scanning
    - CODE_OF_CONDUCT.md
```

`ignore_repos` filters repos out of the peribolos-driven list. When `scan_paths`
is set, the tool recursively lists `.md` files under each path for every eligible
repo and syncs them as doc pages at
`content/docs/projects/{repo}/{relative-path}`. Files already declared in
`sources` or listed in `ignore_files` are skipped.

### `sources`

Declares specific files to sync with fine-grained control.

```yaml
sources:
  - repo: complytime/complyctl
    branch: main                      # optional, inherits from defaults
    skip_org_sync: true               # suppress auto-generated README page
    files:
      - src: README.md
        dest: content/docs/projects/complyctl/_index.md
        transform:
          inject_frontmatter:
            title: "complyctl"
            description: "A compliance CLI tool."
            weight: 10
          rewrite_links: true
          strip_badges: true

      - src: docs/QUICK_START.md
        dest: content/docs/projects/complyctl/quick-start.md
        transform:
          inject_frontmatter:
            title: "Quick Start"
            description: "Getting started with complyctl."
            weight: 20
          rewrite_links: true
```

Each `files` entry maps one upstream file (`src`) to one local destination
(`dest`) with optional transforms.

## Architecture

### Data Flow

```
GitHub REST API
    │
    ├─ GET /repos/{org}/.github/contents/peribolos.yaml  → governance registry
    ├─ GET /repos/{owner}/{repo}           → per-repo metadata enrichment
    ├─ GET /repos/{owner}/{repo}/readme      → fetch README content + SHA
    ├─ GET /repos/{owner}/{repo}/branches/{branch}  → branch HEAD SHA
    ├─ GET /repos/{owner}/{repo}/contents/{path}     → fetch config-declared files
    └─ GET /repos/{owner}/{repo}/contents/{dir}      → list docs/ for doc page scanning
           │
           ▼
    ┌─────────────────────────────────────────────┐
    │              sync-content                    │
    │                                              │
    │  Governance Discovery ──┐                    │
    │    • read peribolos.yaml│                    │
    │    • enrich via API     ├─→ Project Pages    │
    │    • fetch READMEs      │   ProjectCards     │
    │    • scan doc pages     │                    │
    │                         │                    │
    │  Config Sync ───────────┤                    │
    │    • fetch declared     ├─→ Config Files     │
    │      files              │   (with transforms)│
    │    • apply transforms   │                    │
    │                         │                    │
    │  Change Detection ──────┤                    │
    │    • branch SHA cache   ├─→ Skip unchanged   │
    │    • README blob SHA    │                    │
    │    • byte-level dedup   │                    │
    │                         │                    │
    │  Orphan Cleanup ────────┘                    │
    │    • manifest diffing   ──→ Remove stale     │
    │    • empty dir pruning       files           │
    └─────────────────────────────────────────────┘
           │
           ▼
    Hugo Content Tree
    ├─ content/docs/projects/{repo}/_index.md   (section index)
    ├─ content/docs/projects/{repo}/overview.md  (README content)
    ├─ content/docs/projects/{repo}/*.md         (discovered docs)
    ├─ data/projects.json
    ├─ .sync-manifest.json
    └─ .content-lock.json  (committed, updated by --update-lock)
```

### Key Design Decisions

**Dry-run by default.** The tool never writes to disk unless `--write` is passed.
This makes it safe to run in CI for validation and locally for exploration.

**Two-tier change detection.** On each run the tool reads `source_sha` and
`readme_sha` from existing project page frontmatter. If the branch HEAD SHA
hasn't changed, all fetches for that repo are skipped (fast path). If the branch
moved but the README blob SHA is identical, the repo is classified as unchanged.
This minimizes API calls and disk writes.

**Manifest-based orphan cleanup.** A `.sync-manifest.json` file tracks every file
written during a sync run. On the next run, files in the old manifest but absent
from the current run are deleted, and empty parent directories are pruned. This
handles repos being archived, renamed, or removed from the org.

**Idempotent writes.** Before writing a file, the tool reads the existing content
and compares bytes. If identical, the write is skipped entirely. This means
running the tool twice in succession produces zero disk I/O on the second run.

**Provenance comments.** Every synced file includes an HTML comment after the
frontmatter:

```
<!-- synced from complytime/complyctl/README.md@main (abc123def456) -->
```

This makes it trivial to trace any page back to its upstream source and commit.

**Bounded concurrency with rate-limit awareness.** A worker pool (default 5,
configurable via `--workers`) processes repos concurrently. The API client retries
on HTTP 403/429 with exponential backoff, respecting `Retry-After` and
`X-RateLimit-Reset` headers. A global context timeout (default 3 minutes) prevents
runaway execution.

**Content approval gate.** A committed `.content-lock.json` file pins each repo
to an approved branch SHA. The deploy workflow fetches content at locked SHAs —
not HEAD. A weekly check workflow (`sync-content-check.yml`) detects upstream
changes and opens a PR to update the lockfile. This prevents broken or
undesirable content from reaching production without human review.

**Single package, single dependency.** The entire tool lives in `package main` within `cmd/sync-content/` — domain-organised source files, one third-party dependency (`gopkg.in/yaml.v3`). No separate packages, no interfaces, no abstractions beyond what the problem requires.

### Output Entities

#### ProjectCard (`data/projects.json`)

```json
{
  "name": "complyctl",
  "language": "Go",
  "type": "CLI Tool",
  "description": "A compliance CLI tool for Kubernetes.",
  "url": "/docs/projects/complyctl/",
  "repo": "https://github.com/complytime/complyctl",
  "stars": 42
}
```

The `type` field is derived from repo topics and description using keyword
matching:

| Keywords | Type |
|----------|------|
| `cli` topic, "command-line" or " cli" in description | CLI Tool |
| `automation` topic, "automat" in description | Automation |
| `observability` topic, "observability" or "collector" in description | Observability |
| `framework` topic, "framework" or "bridging" in description | Framework |
| _(default)_ | Library |

#### Section Index Frontmatter (`_index.md`)

```yaml
---
title: "Complyctl"
linkTitle: "complyctl"
description: "A compliance CLI tool for Kubernetes."
date: 2026-03-10T18:30:00Z
lastmod: 2026-03-10T18:30:00Z
draft: false
toc: false
params:
  language: "Go"
  stars: 42
  repo: "https://github.com/complytime/complyctl"
  source_sha: "abc123def456"
  readme_sha: "def789abc012"
  seo:
    title: "Complyctl | ComplyTime"
    description: "A compliance CLI tool for Kubernetes."
---
```

#### Overview Page Frontmatter (`overview.md`)

```yaml
---
title: "Overview"
description: "A compliance CLI tool for Kubernetes."
date: 2026-03-10T18:30:00Z
lastmod: 2026-03-10T18:30:00Z
draft: false
toc: true
weight: 1
params:
  editURL: "https://github.com/complytime/complyctl/edit/main/README.md"
---
```

#### Auto-Discovered Doc Page Frontmatter

```yaml
---
title: "Quick Start"
description: "A compliance CLI tool for Kubernetes. — Quick Start"
date: 2026-03-10T18:30:00Z
lastmod: 2026-03-10T18:30:00Z
draft: false
weight: 10
params:
  editURL: "https://github.com/complytime/complyctl/edit/main/docs/quick-start.md"
---
<!-- synced from complytime/complyctl/docs/quick-start.md@main (abc123def456) -->
```

### Content Transforms

| Transform | What it does |
|-----------|-------------|
| `stripLeadingH1` | Removes the first H1 heading from the content body — the title is already captured in frontmatter, so the leading H1 would be a duplicate |
| `shiftHeadings` | Bumps every Markdown heading down one level (H1→H2, H2→H3, …) so Hugo's page title is the sole H1 |
| `titleCaseHeadings` | Applies acronym-aware Title Case to all in-page heading text (e.g. `## getting started` → `## Getting Started`, `## api reference` → `## API Reference`, `## CONTRIBUTING` → `## Contributing`); normalises ALL CAPS words while preserving known acronyms; ensures page headings and Hugo's TableOfContents match |
| `stripBadges` | Removes `[![alt](img)](link)` badge patterns from the start of content |
| `rewriteRelativeLinks` | Converts `[text](path)` to `[text](https://github.com/.../blob/main/path)` and `![alt](img)` to `![alt](https://raw.githubusercontent.com/.../img)` |
| `injectFrontmatter` | Prepends or replaces YAML frontmatter with declared key-value pairs |

## CI/CD Integration

### Three-Workflow Model

The tool integrates with three GitHub Actions workflows (Constitution XV v1.3.0):

**1. CI (`ci.yml`)** — PR validation (syncs content and builds the site to catch breakage):

```yaml
- name: Sync content
  run: go run ./cmd/sync-content --org complytime --config sync-config.yaml --lock .content-lock.json --write
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**2. Content Sync Check (`sync-content-check.yml`)** — weekly upstream detection:

```yaml
- name: Check for upstream changes
  run: go run ./cmd/sync-content --org complytime --config sync-config.yaml --lock .content-lock.json --update-lock --summary sync-summary.md
```

Checks upstream SHAs and creates/updates a PR with lockfile changes when content has moved. Since peribolos provides the authoritative repo list, separate discovery is unnecessary.

**3. Deploy (`deploy-gh-pages.yml`)** — production build:

```yaml
- name: Sync content
  run: go run ./cmd/sync-content --org complytime --config sync-config.yaml --lock .content-lock.json --write
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

- name: Build site
  run: hugo --minify --gc
```

Upstream content changes require a reviewed PR before reaching production — no
unreviewed content is deployed.

### Structured Outputs

When running in GitHub Actions, the tool writes structured data to
`$GITHUB_OUTPUT` and `$GITHUB_STEP_SUMMARY`:

**`GITHUB_OUTPUT`:**

```
has_changes=true
changed_count=3
error_count=0
```

**`GITHUB_STEP_SUMMARY`:** A Markdown table with new/updated/removed repos and
sync stats.

**`--summary` flag:** Writes the same Markdown report to a file, useful for
automated PR body generation.

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (all repos synced or dry-run complete) |
| 1 | One or more errors occurred (API failures, write errors) |

## Testing

Tests are split across 10 `*_test.go` files that mirror the source files. A
shared `helpers_test.go` provides common utilities.

```bash
# Run all tests
go test ./cmd/sync-content/...

# Run with race detector
go test -race ./cmd/sync-content/...

# Run with verbose output
go test -v ./cmd/sync-content/...
```

### Test Coverage

| Category | What's tested |
|----------|---------------|
| Config loading | Valid YAML, malformed YAML, missing file, default values, missing required fields |
| Frontmatter injection | Prepend to bare content, replace existing frontmatter, empty content |
| Badge stripping | Line-start badges removed, inline badges preserved, no-badge passthrough |
| Heading shifting | All headings bumped down one level (H1→H2, H2→H3, …) so Hugo page title is the sole H1 |
| Heading casing | ALL CAPS normalised to Title Case, acronyms preserved, mixed-case normalised, multi-word headings |
| Title from filename | ALL CAPS filenames (`CONTRIBUTING.md` → `Contributing`), hyphen/underscore splitting, acronym preservation |
| Link rewriting | Relative to absolute, images to raw URLs, absolute URLs unchanged, anchors unchanged, `./` prefix |
| Repo name validation | Valid names, empty, `.`, `..`, path separators |
| `processRepo` integration | Mock API server, project page written with correct frontmatter, headings shifted, README SHA recorded |
| Branch-unchanged fast path | No README fetch when branch SHA matches, manifest carry-forward |
| Branch-changed README-unchanged | Two-tier detection classifies as unchanged |
| `syncConfigSource` | All transforms applied, provenance comment inserted, dry-run writes nothing |
| Doc page scanning | Auto-syncs `docs/*.md`, skips config-tracked files, generates section indexes |
| Manifest round-trip | Write and read manifest, orphan cleanup, empty directory pruning |
| Concurrent access | Race-safe `syncResult` mutations, concurrent `recordFile` |
| Peribolos integration | Governance registry fetch, repo validation, missing org handling |

All integration tests use `net/http/httptest` to mock the GitHub API. No real API
calls are made during testing.

## File Inventory

```
cmd/sync-content/
├── main.go           # Entry point and orchestration (~440 lines)
├── config.go         # Config types and loading
├── github.go         # GitHub API client and types
├── transform.go      # Markdown transforms (links, badges, frontmatter)
├── hugo.go           # Hugo page and card generation
├── sync.go           # Sync logic, result tracking, repo processing
├── manifest.go       # Manifest I/O and state tracking
├── cleanup.go        # Orphan and stale content removal
├── path.go           # Path validation utilities
├── lock.go           # Content lockfile read/write/query
├── *_test.go         # Tests mirror source files (10 files)
└── README.md         # This file

sync-config.yaml      # Declarative sync config (repo root)
.content-lock.json    # Approved upstream SHAs per repo (committed)
go.mod                # Go module: github.com/complytime/website
go.sum                # Dependency checksums
```

### Generated Files (gitignored, not committed)

```
content/docs/projects/{repo}/_index.md    # Section index (metadata only)
content/docs/projects/{repo}/overview.md  # README content page
content/docs/projects/{repo}/*.md         # Auto-discovered doc pages
data/projects.json                        # Landing page card data
.sync-manifest.json                       # Orphan tracking manifest
```

## License

SPDX-License-Identifier: Apache-2.0
