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
For each eligible repo it:

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
6. Rewrites diagram code blocks (mermaid, plantuml, d2, etc.) to Kroki format for server-side rendering.
7. Rewrites relative Markdown links and images to absolute GitHub URLs.
8. Scans for doc pages under configurable `scan_paths` (e.g. `docs/`).
9. Builds a `ProjectCard` for the landing page.

After processing all repos, the tool writes `data/projects.json` — an array of
`ProjectCard` objects that Hugo templates use to render the "Our Projects" section.

### Phase 2: Config Sync (opt-in)

Reads `sync-config.yaml` and pulls specific files with per-file transforms.

Three transforms are **always applied** to every config-synced file, regardless
of transform flags: `stripLeadingH1`, `shiftHeadings`, and `titleCaseHeadings`.
These ensure consistent heading structure across all synced content.

The following transforms are opt-in per file:

- **Badge stripping** (`strip_badges`) — remove CI/status badge lines from the
  top of content (applied before the always-on transforms).
- **Link rewriting** (`rewrite_links`) — convert relative Markdown links to
  absolute GitHub blob URLs and relative images to raw.githubusercontent URLs.
- **Diagram rewriting** (`rewrite_diagrams`) — convert fenced diagram code
  blocks to Kroki format for server-side rendering.
- **Frontmatter injection** (`inject_frontmatter`) — prepend YAML frontmatter
  with arbitrary key-value pairs, or replace existing frontmatter.

Config sources can operate alongside or instead of the org scan per-repo:

| `skip_org_sync` | Org scan page | Config files | ProjectCard |
|-----------------|---------------|--------------|-------------|
| `false` (default) | Generated from README | Synced as additional content | Yes |
| `true` | Suppressed | Synced as primary content | Yes |

## Quick Reference

For setup, prerequisites, and day-to-day commands, see
[CONTRIBUTING.md](../../CONTRIBUTING.md#development-workflow). For the full CLI
flag reference, see the [spec](../../specs/006-go-sync-tool/spec.md#cli-interface).

The essentials (using the Makefile at the repo root — run `make help` for a full list):

```bash
make sync-dry                              # dry-run: reads GitHub, writes nothing
make sync                                  # apply changes to disk
make sync-single REPO=complytime/complyctl # single-repo apply
make test-race                             # run tests with race detector
make check                                 # vet + fmt-check + race tests
```

Equivalent raw commands:

```bash
go run ./cmd/sync-content --org complytime --config sync-config.yaml           # dry-run
go run ./cmd/sync-content --org complytime --config sync-config.yaml --write   # write mode
go run ./cmd/sync-content --repo complytime/complyctl --config sync-config.yaml --write  # single repo
go test -race ./cmd/sync-content/...                                           # run tests
```

## CLI Reference

| Flag | Default | Description |
|------|---------|-------------|
| `--org` | `complytime` | GitHub organization name |
| `--token` | `""` | GitHub API token (falls back to `GITHUB_TOKEN` env var) |
| `--output` | `.` | Hugo site root directory |
| `--config` | `""` | Path to `sync-config.yaml` for declarative file syncs |
| `--repo` | `""` | Sync only this repo (e.g. `complytime/complyctl`); must be in peribolos |
| `--write` | `false` | Apply changes to disk (default: dry-run) |
| `--workers` | `5` | Maximum concurrent repo processing goroutines |
| `--timeout` | `3m` | Overall timeout for all API operations |
| `--lock` | `""` | Path to `.content-lock.json` for content approval gating |
| `--update-lock` | `false` | Write updated upstream SHAs to the lockfile (requires `--lock`) |
| `--summary` | `""` | Write Markdown change summary to this file (for PR body generation) |
| `--include` | `""` | Comma-separated repo allowlist (empty = auto-discover all) |
| `--exclude` | `""` | Comma-separated repo names to skip (merged with `discovery.ignore_repos`) |

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
          rewrite_diagrams: true
          strip_badges: true

      - src: docs/QUICK_START.md
        dest: content/docs/projects/complyctl/quick-start.md
        transform:
          inject_frontmatter:
            title: "Quick Start"
            description: "Getting started with complyctl."
            weight: 20
          rewrite_links: true
          rewrite_diagrams: true
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
handles repos being renamed or removed from peribolos.

**Idempotent writes.** Before writing a file, the tool reads the existing content
and compares bytes. If identical, the write is skipped entirely. This means
running the tool twice in succession produces zero disk I/O on the second run.

**Provenance comments.** Config-declared files and auto-discovered doc pages
include an HTML comment after the frontmatter:

```
<!-- synced from complytime/complyctl/README.md@main (abc123def456) -->
```

This makes it trivial to trace synced pages back to their upstream source and
commit. Governance-generated pages (`_index.md` and `overview.md`) do not
include provenance comments — their `source_sha` and `readme_sha` frontmatter
fields serve the same purpose.

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

**Single package, single dependency.** The entire tool lives in `package main` within `cmd/sync-content/` — domain-organised source files, one third-party dependency (`github.com/goccy/go-yaml`). No separate packages, no interfaces, no abstractions beyond what the problem requires.

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
| `automation` topic, "automation" or "automat" in description | Automation |
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
| `rewriteDiagramBlocks` | Converts fenced diagram code blocks (mermaid, plantuml, d2, graphviz/dot, ditaa, and 13 other Kroki-supported languages) to `` ```kroki {type=…} `` format for server-side rendering; `dot` normalised to `graphviz` |
| `injectFrontmatter` | Prepends or replaces YAML frontmatter with declared key-value pairs |

## Operational Details

### Safety Guards

- **Zero-eligible-repo protection.** If the API returns zero eligible repos but
  previous state exists (suggesting an API outage or misconfiguration), the tool
  refuses to run cleanup and exits with an error. This prevents accidental
  deletion of all content.
- **Path traversal protection.** Every file write is validated by `isUnderDir`
  to ensure the destination is within the output directory. Paths that escape
  the site root are blocked and logged as errors.
- **Config source governance.** Config-declared repos (`sources` entries) must
  exist in the governance registry (`peribolos.yaml`). Repos not in the registry
  are rejected with an error. The `--repo` flag is similarly validated.

### HTTP Client Behaviour

- **Per-request timeout:** 30 seconds per API call (separate from `--timeout`).
- **Response body limit:** 10 MB safety ceiling on API responses.
- **Rate limiting:** Retries up to 3 times on HTTP 403 (when `X-RateLimit-Remaining`
  is 0) and 429, with exponential backoff respecting `Retry-After` and
  `X-RateLimit-Reset` headers. Reset times more than 5 minutes in the future
  fall back to exponential backoff.

### Content Processing

- **`index.md` files are skipped** during `scan_paths` discovery. Hugo treats
  `index.md` as a leaf bundle, which conflicts with the `_index.md` branch
  bundle the sync tool generates for every project directory.
- **Intermediate section pages.** When discovered doc files live in nested
  subdirectories under `content/docs/projects/{repo}/`, the tool auto-generates
  `_index.md` section pages for each intermediate directory so Hugo's sidebar
  renders the hierarchy correctly. Content outside `content/docs/projects/`
  (e.g. `getting-started/`, `privacy.md`) is hand-authored and committed
  normally — the sync tool never writes outside the projects tree.
- **Empty language fallback.** When GitHub reports no language for a repo, the
  `language` field in frontmatter and `ProjectCard` is set to `"Unknown"`.
- **Max directory scan depth.** Recursive `scan_paths` listing is bounded to 10
  levels to limit API calls on deeply nested repositories.
- **Manifest carry-forward.** When a repo is unchanged (fast-path skip), its
  entries from the previous manifest are preserved so orphan cleanup does not
  remove files that were simply not re-fetched.

## CI/CD Integration

The tool integrates with three GitHub Actions workflows. See
[CONTRIBUTING.md](../../CONTRIBUTING.md#cicd-and-deployment) for workflow details.

**Structured outputs** — when running in GitHub Actions, the tool writes to
`$GITHUB_OUTPUT` and `$GITHUB_STEP_SUMMARY` (Markdown sync report). The
`--summary` flag writes the same report to a file for PR body generation.

`GITHUB_OUTPUT` variables:

| Variable | Description |
|----------|-------------|
| `has_changes` | `true` if any repos were added, updated, or removed |
| `changed_count` | Number of repos added or updated |
| `changed_repos` | Comma-separated list of changed repo names |
| `changed_files_count` | Total number of individual files that changed |
| `files_processed` | Total number of files processed across all repos |
| `error_count` | Number of errors encountered during sync |

**Exit codes**: `0` = success, `1` = one or more errors (API failures, write errors).

## License

SPDX-License-Identifier: Apache-2.0
