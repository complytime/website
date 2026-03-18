# ComplyTime Website Constitution

## Core Principles

### I. Hugo + Doks

The site framework is [Hugo](https://gohugo.io/) (extended) with the [Thulite Doks](https://getdoks.org/) theme. No alternative static site generators, themes, or frontend frameworks are permitted. All theme customization is done through SCSS overrides and Hugo layout overrides — not by forking or vendoring the theme.

### II. Go Tooling

All custom tooling (content sync, CLI utilities, build helpers) MUST be written in Go. The Go module in `go.mod` is shared with Hugo Modules. Third-party Go dependencies MUST be minimized; new dependencies require documented justification.

### III. Single Source of Truth

Every piece of project content on the site MUST trace back to a canonical source — a repo README, a `docs/` directory, or the GitHub API. The org's governance registry (`peribolos.yaml` in the `.github` repo) is the authoritative source for which repositories exist. Automated tooling is the sole mechanism for pulling upstream content. Manual content duplication is prohibited. If the source changes, the site updates on the next sync.

### IV. Governance-Driven Discovery with Config Overlay

The sync tool derives the set of eligible repositories from the org's governance registry rather than ad-hoc API discovery. Per-repo metadata (stars, language, topics) is fetched from the GitHub API. For repos requiring precise control (frontmatter, transforms, specific files), a declarative config overlay adds file-level syncs on top. The governance registry is the baseline; config is the precision layer.

### V. No Runtime JavaScript Frameworks

The site is statically generated. Client-side interactivity is limited to what Doks provides (FlexSearch, dark mode toggle, navigation). Custom JavaScript MUST be minimal and progressive — the site MUST function fully without JavaScript except for search.

### VI. Match the ComplyTime Brand

The site's visual design uses the established color palette, typography, and dark-theme-first aesthetic defined in the SCSS variables. Visual changes MUST maintain brand consistency and MUST NOT introduce new design systems or CSS frameworks beyond what Doks provides.

### VII. Responsive and Accessible

All pages MUST meet WCAG 2.1 AA. The site MUST be fully usable on mobile, tablet, and desktop viewports. Color contrast, keyboard navigation, alt text, and ARIA labels are mandatory for all new content and layouts.

### VIII. Performance

Hugo builds MUST complete in under two seconds for the current content volume. Pages MUST achieve a Lighthouse performance score of 90+. PurgeCSS is configured via PostCSS to eliminate unused styles from production builds.

## Development Standards

### IX. SPDX License Headers

Every Go source file MUST include `// SPDX-License-Identifier: Apache-2.0` as the first comment line.

### X. Go Code Quality

All Go code MUST pass `go vet`, `gofmt`, and any configured linter checks before merge. Errors MUST always be checked and returned — never silently discarded.

### XI. Structured Logging

The Go sync tool MUST use `log/slog` for all logging. All log entries MUST include relevant structured fields (`repo`, `path`, `sha`, `error`). No `fmt.Println` or `log.Printf` for operational output.

### XII. Dry-Run by Default

The sync tool MUST default to dry-run mode. The `--write` flag is required for any disk I/O. This protects contributors from accidentally overwriting their local working tree.

### XIII. Generated Content Is Not Committed

All sync tool output (project pages, card data) is derived from the GitHub API and MUST be gitignored. The repository tracks only source files: Go code, config, templates, hand-authored content, and styling. CI generates all derived content from scratch on every build. Control files that gate what is generated (e.g. content lockfiles) ARE committed because they represent reviewed approval state, not derived content.

### XIV. Simplicity

Start simple, apply YAGNI. No abstractions without proven need. Tooling favors flat, domain-organised source files over deep package hierarchies. Complexity MUST be justified against a simpler alternative.

## Operations

### XV. GitHub Actions CI/CD

Build, sync, and deployment are fully automated via GitHub Actions. No manual deployment steps. The workflow model includes:

1. **CI** — validates PRs with dry-run sync, Go checks, and Hugo build.
2. **Content Sync Check** — runs periodically to detect upstream changes and open a PR for human review.
3. **Deploy** — on push to the default branch, syncs content at approved SHAs, builds Hugo, and publishes to GitHub Pages.

Upstream content changes MUST be reviewed via a content sync PR before reaching production. No unreviewed content is deployed.

### XVI. GitHub Pages Hosting

The site is hosted on GitHub Pages. No other hosting platforms are permitted without an amendment to this constitution.

## Licensing

### XVII. Apache 2.0

All website code, tooling, and original content is licensed under Apache License 2.0. Synced content retains its upstream license.

## Governance

This constitution supersedes all other practices for the complytime-website repository. Amendments require:

1. A documented proposal explaining the change and its rationale.
2. Update to this file with version increment per semantic versioning (MAJOR for principle removal/redefinition, MINOR for additions, PATCH for clarifications).
3. Propagation check across any dependent specs, plans, or task files.

All PRs and reviews MUST verify compliance with these principles.

**Version**: 1.5.0 | **Ratified**: 2026-03-11 | **Last Amended**: 2026-03-16
