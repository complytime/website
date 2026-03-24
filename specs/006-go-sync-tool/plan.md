# Implementation Plan: Go Content Sync Tool

**Branch**: `006-go-sync-tool` | **Date**: 2026-03-04 | **Spec**: [specs/006-go-sync-tool/spec.md](/specs/006-go-sync-tool/spec.md)
**Input**: Feature specification from `/specs/006-go-sync-tool/spec.md` (consolidated)

## Summary

Replace the config-only `cmd/sync-content` tool with the production-quality governance-driven hybrid sync tool ported from the test-website repository. The tool derives the set of eligible repositories from the org's governance registry (`peribolos.yaml` in the `.github` repo), fetches per-repo metadata via the GitHub REST API, applies Markdown transforms (heading level shifting, acronym-aware Title Case with ALL CAPS normalisation, duplicate H1 removal, badge stripping, relative link rewriting, diagram code block rewriting to Kroki format), and generates Hugo-compatible pages and landing page card data. A declarative config overlay layers file-level syncs on top. The surrounding infrastructure (gitignore, directory scaffolding, CI workflows, Hugo layouts including a render heading hook) must be adapted to consume the new tool's output.

## Technical Context

**Language/Version**: Go 1.25 (sync tool), Hugo 0.155.1 extended (site generator), Node.js 22 (Doks theme build)
**Primary Dependencies**: `gopkg.in/yaml.v3` (only third-party Go dep), `@thulite/doks-core` (Hugo theme), Hugo Modules
**Storage**: Filesystem — generated Markdown files and JSON; no database
**Testing**: `go test` with `net/http/httptest` for mock API server, `-race` flag for concurrency safety
**Target Platform**: Linux (CI), macOS/Linux (local dev)
**Project Type**: CLI tool (Go) embedded in a static website repo (Hugo)
**Performance Goals**: Full org sync < 60s with token; Hugo build < 2s
**Constraints**: All code in `package main` within `cmd/sync-content/` (Constitution XIV: Simplicity); third-party deps minimized — `gopkg.in/yaml.v3` is the sole dep (Constitution II)
**Scale/Scope**: 10 eligible repos in org, 10 Go source files, 10 test files

## Constitution Check (Pre-Design)

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Pre-Design Gate Result**: PASS — all 17 principles checked; all now satisfied. X (`go vet` + `gofmt` in `ci.yml`) and XV (three-workflow CI/CD model) resolved. See Post-Design Re-Check below for detailed table.

## Project Structure

### Documentation (this feature)

```text
specs/006-go-sync-tool/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0 output
└── tasks.md             # Implementation tasks
```

### Source Code (repository root)

```text
complytime-website/
├── cmd/
│   └── sync-content/
│       ├── main.go          # Entry point and orchestration (~440 lines)
│       ├── config.go        # Config types and loading (incl. Peribolos types)
│       ├── github.go        # GitHub API client and types (incl. peribolos fetch)
│       ├── transform.go     # Markdown transforms
│       ├── hugo.go          # Hugo page and card generation
│       ├── sync.go          # Sync logic and repo processing
│       ├── manifest.go      # Manifest I/O and state tracking
│       ├── cleanup.go       # Orphan and stale content removal
│       ├── path.go          # Path validation utilities
│       ├── lock.go          # Content lockfile read/write/query
│       └── *_test.go        # Tests mirror source files (10 files)
├── config/
│   └── _default/
│       ├── hugo.toml        # Site config
│       ├── module.toml      # Hugo module mounts (existing)
│       ├── params.toml      # Doks theme params (existing)
│       └── menus/
│           └── menus.en.toml # Navigation menus (Projects entry exists at weight 20)
├── content/
│   ├── docs/
│   │   ├── projects/
│   │   │   ├── _index.md    # Hand-maintained section index (committed, has cascade for sidebar collapsing)
│   │   │   └── {repo}/      # Generated per-repo content (gitignored)
│   │   │       ├── _index.md    # Section index (frontmatter only, no body)
│   │   │       ├── overview.md  # README content as child page
│   │   │       └── {doc}.md     # Doc pages from discovery.scan_paths
│   │   └── getting-started/ # Hand-maintained (committed)
├── data/
│   └── projects.json        # Generated landing page cards (gitignored)
├── .sync-manifest.json      # Tracks written files for orphan cleanup (gitignored)
├── layouts/
│   ├── home.html            # Landing page (reads data/projects.json dynamically)
│   ├── shortcodes/
│   │   └── project-cards.html # Project cards shortcode (type-grouped, reads data/projects.json)
│   └── docs/
│       └── list.html        # Docs list with sidebar (already exists)
├── .github/
│   └── workflows/
│       ├── deploy-gh-pages.yml      # Deploy pipeline (sync at locked SHAs, Hugo build, GitHub Pages)
│       ├── ci.yml                   # PR validation (test, sync with --lock, build)
│       └── sync-content-check.yml   # Weekly content check (--update-lock, PR creation)
├── sync-config.yaml         # Declarative file sync manifest (updated)
├── .content-lock.json       # Approved upstream SHAs per repo (committed)
├── go.mod                   # Go module (initialized fresh for the port)
├── go.sum                   # Go checksums (generated by go mod tidy)
└── .gitignore               # Updated with generated path exclusions
```

**Structure Decision**: Single-project layout. The sync tool is organized as multiple files within `package main` at `cmd/sync-content/` (10 source files). No separate packages, no `internal/`, no `pkg/`. Files are split by domain (config, GitHub API, transforms, Hugo pages, sync logic, manifest, cleanup, path utils, content lockfile, entry point). Tests mirror source files 1:1. This matches Constitution XIV (Simplicity) — no unnecessary abstractions while keeping each file focused.

## Constitution Re-Check (Post Phase 1 Design)

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Hugo + Doks | PASS | No changes to site framework. |
| II. Go Tooling | PASS | Third-party Go dependencies minimized; `gopkg.in/yaml.v3` is the sole dep (also used for peribolos parsing). |
| III. Single Source of Truth | PASS | Content sourced from GitHub API. Governance registry (`peribolos.yaml`) is authoritative for which repos exist (Constitution v1.5.0). |
| IV. Governance-Driven Discovery with Config Overlay | PASS | Repo list derived from `peribolos.yaml`; per-repo metadata from API; `sync-config.yaml` overlay for precision (Constitution v1.5.0). |
| V. No Runtime JS Frameworks | PASS | Diagram code blocks are rewritten to Kroki format (`render-codeblock-kroki.html`) for server-side rendering rather than using Doks' client-side `render-codeblock-mermaid.html`. No custom JavaScript added. |
| VI. Match ComplyTime Brand | PASS | `layouts/home.html` uses dynamic project cards from `data/projects.json`. Visual styling and brand consistency preserved. |
| VII. Responsive and Accessible | PASS | No layout changes required. |
| VIII. Performance | PASS | Hugo build < 2s, sync < 60s targets achievable. |
| IX. SPDX License Headers | PASS | Present in all `.go` source and test files. |
| X. Go Code Quality | PASS | `go vet` + `gofmt` checks run in `deploy-gh-pages.yml`; `go test -race` in both CI and deploy. |
| XI. Structured Logging | PASS | All `log/slog` with structured fields. |
| XII. Dry-Run by Default | PASS | `--write` required for disk I/O. Dry-run validated. |
| XIII. Generated Content Not Committed | PASS | `.gitignore` updated (T001). `.content-lock.json` is a committed control file, not derived content. |
| XIV. Simplicity | PASS | All code in `package main`, no unnecessary packages or abstractions. |
| XV. GitHub Actions CI/CD | PASS | Three-workflow model: CI, Content Sync Check, Deploy. |
| XVI. GitHub Pages Hosting | PASS | No hosting changes. |
| XVII. Apache 2.0 | PASS | SPDX headers present. |

**Post-Design Gate Result**: PASS. All 17 principles satisfied per Constitution v1.5.0. Principle IV updated from API-based org scan to governance-driven discovery via peribolos.yaml (IS-001).

## Hardening (Post-Audit)

A code audit of `cmd/sync-content/` identified 18 findings across security, logic, redundancy, performance, and flexibility. These were cross-referenced against the spec, plan, and existing tasks — none were previously tracked.

**In-scope for feature 006** (10 tasks, T028–T037):
- **Tier 1 — Security & Correctness**: Path traversal via config `dest` (T028), context-cancellation gap in retry sleep (T029), incomplete stale cleanup (T030)
- **Tier 2 — Defensive Coding**: Unbounded error body read (T031), URL escaping (T032), dry-run card building (T033)
- **Tier 3 — Redundancy Removal**: Duplicated card builder (T034), dead branch fallback (T035), hardcoded exclude list (T036)
- **Hardening Tests**: T037 covers path traversal rejection, ctx cancellation, stale cleanup completeness

**Deferred** (7 findings — design improvements, not bugs at current scale):
- Serial recursive API calls (#11), HTTP connection pooling (#12), hardcoded API URL (#14), public-only repos (#15), no log level control (#17), no config schema version (#18). Finding #13 (redundant README fetch in discovery) is N/A after discovery mode removal in T054.

## Complexity Tracking

No constitution violations. All design choices align with established principles. Hardening phase adds security and correctness fixes without introducing new dependencies or abstractions — consistent with Constitution XIV (Simplicity). Phase 13 (content transform improvements) adds heading casing normalisation, ALL CAPS normalisation, duplicate H1 removal, and a Hugo render heading hook — all within existing files, no new packages or dependencies. Phase 14 (diagram block rewriting) adds Kroki format conversion for upstream diagram code blocks, routing mermaid through server-side Kroki rather than client-side JS — consistent with Constitution V (No Runtime JavaScript Frameworks).
