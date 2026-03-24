# Research: Go Content Sync Tool

**Date**: 2026-03-11
**Spec**: [specs/006-go-sync-tool/spec.md](/specs/006-go-sync-tool/spec.md)

## R1: Gitignore Patterns for Generated Content

**Decision**: Use directory-level gitignore patterns with `!` negation for hand-maintained files.

**Rationale**: The sync tool generates content at two paths:
- `content/docs/projects/{repo}/_index.md` (per-repo project pages)
- `data/projects.json` (landing page cards)

The hand-maintained `content/docs/projects/_index.md` must remain committed. The gitignore pattern `content/docs/projects/*/` excludes all subdirectories while preserving the section-level `_index.md` file.

**Alternatives considered**:
- Ignoring by filename pattern (e.g., `*/_index.md`) — too broad, would catch hand-maintained files.
- Committing generated content — rejected per Constitution XIII.

**Current `.gitignore` state**: Updated — `content/docs/projects/*/` and `data/projects.json` patterns are in place.

## R2: Hugo Template Integration for `data/projects.json`

**Decision**: The existing `layouts/home.html` already iterates over `data/projects.json` using Hugo's data template mechanism (`site.Data.projects`). No template changes needed for landing page cards.

**Rationale**: Verified by reading `layouts/home.html` in the current repo — it contains a projects section that reads from `site.Data.projects`. The `ProjectCard` JSON structure (name, language, type, description, url, repo, stars) matches what the template expects.

**Alternatives considered**: None — the template is already compatible.

## R3: Docs Sidebar Integration

**Decision**: The existing `layouts/docs/list.html` and `config/_default/menus/menus.en.toml` already provide sidebar navigation for docs pages. Each synced repo produces a section index (`_index.md`, frontmatter only) and an overview page (`overview.md`, README content). The `_index.md` makes the repo a Hugo section so child pages appear in the sidebar. The `overview.md` is the first navigable child page (weight 1).

**Rationale**: Separating the section index from the README content enables the Doks sidebar to render the repo as a collapsible section heading with child pages underneath (overview + doc sub-pages). The `menus.en.toml` already has a `[[docs]]` entry for Projects pointing to `/docs/projects/`.

**Alternatives considered**:
- Single `_index.md` with README body — rejected because Doks renders `_index.md` body inline at the section level, preventing collapsible sidebar sections with separate child pages.
- Custom sidebar template — unnecessary, Hugo's built-in section discovery handles it.

### R3a: Sidebar Collapsing via Hugo Cascade

**Decision**: Use Hugo's `cascade` frontmatter in `content/docs/projects/_index.md` to push `sidebar.collapsed: true` to repo-level section pages, rather than stamping it into each generated `_index.md`.

**Implementation**: Added to `content/docs/projects/_index.md` frontmatter:
```yaml
cascade:
  - sidebar:
      collapsed: true
    _target:
      kind: section
      path: "{/docs/projects/*}"
```

**Rationale**: The Doks template (`render-section-menu.html` line 89) reads `$node.Page.Params.sidebar.collapsed`. Hugo cascade makes cascaded values accessible through `.Params`, so no template changes are needed. The `_target` uses `kind: section` + single `*` glob to match only repo-level sections (e.g., `complyctl`, `complyscribe`) but not their sub-folders (e.g., `complyctl/man`).

**Why cascade over sync tool frontmatter**:
- Sidebar behavior is defined once in the content hierarchy, not repeated per-page
- The sync tool stays focused on content, not UI concerns
- New repos automatically inherit collapse behavior without sync tool changes
- Sub-folders remain expanded by design (they don't match the cascade target)

**Alternatives considered**:
- Stamping `sidebar.collapsed: true` in each generated `_index.md` via `buildSectionIndex` — rejected because it couples UI behavior to the sync tool, violates separation of concerns, and requires sync tool changes when sidebar behavior needs to change.

## R4: Deploy Workflow Adaptation

**Decision**: Update `.github/workflows/deploy-gh-pages.yml` to add a sync step before Hugo build:
```
go run ./cmd/sync-content --org complytime --config sync-config.yaml --lock .content-lock.json --write
```

**Rationale**: The current deploy workflow (`deploy-gh-pages.yml`) does `npm ci` then `hugo --minify --gc`. The sync step must run after Go is available and before Hugo builds. Go setup can use `actions/setup-go@v5`.

**Current workflow state**: Updated — `deploy-gh-pages.yml` includes Go setup, sync step with `--lock .content-lock.json --write`, `GITHUB_TOKEN`, push-to-main trigger, and `workflow_dispatch`. The original daily cron was removed in favour of the PR-gated content approval model (Constitution XV v1.3.0): a separate `sync-content-check.yml` workflow runs weekly to detect upstream changes and opens a PR to update `.content-lock.json`.

**Alternatives considered**:
- Separate sync and build workflows — rejected because the deploy must always use fresh synced content. A single pipeline ensures consistency.
- Daily cron deploy — rejected because it would silently propagate broken upstream content. PR-gated review is safer.

## R5: CI Workflow for PR Validation

**Decision**: Add or update a CI workflow that runs on pull requests with: `go vet`, `gofmt` check, `go test -race`, sync dry-run, and full Hugo build.

**Rationale**: Constitution X (Go Code Quality) requires vet/fmt checks. Constitution XII (Dry-Run by Default) means CI can safely preview sync output. The existing `sync-content.yml` only handles the sync step — a separate CI workflow is needed for PR validation.

**Alternatives considered**:
- Combining CI and deploy into one workflow — rejected for clarity and to avoid deploy-specific steps running on PRs.

## R6: Unit Testing Strategy

**Decision**: Write tests in `cmd/sync-content/*_test.go` using `net/http/httptest` to mock the GitHub API. Key test areas:
1. `loadConfig` — valid YAML, malformed YAML, missing file, default values
2. `injectFrontmatter` — prepend new, replace existing, empty content
3. `stripBadges` — badge lines removed, inline badges preserved
4. `shiftHeadings` — all headings bumped down one level (H1→H2, H2→H3, …)
5. `rewriteRelativeLinks` — relative→absolute conversion, absolute URLs preserved
6. `buildSectionIndex` / `buildOverviewPage` — frontmatter schema, deterministic output
7. `processRepo` integration — mock API server, verify page + card output
8. `syncConfigSource` — transforms applied, dry-run respected

**Rationale**: The test-website has `main_test.go` with similar tests. These can be ported and adapted. HTTP test server avoids real API calls.

**Alternatives considered**:
- Interface-based mocking (inject mock HTTP client) — adds abstraction for no benefit in a single-file tool. `httptest.Server` is simpler and more Go-idiomatic.

## R7: `data/` Directory Bootstrap

**Decision**: The `data/` directory must exist for Hugo to find `projects.json`. The sync tool creates it via `os.MkdirAll` when writing `data/projects.json`. For a fresh clone without running the sync tool, Hugo handles the missing `data/` gracefully — `site.Data.projects` returns nil, and the template renders zero cards.

**Rationale**: Verified — Hugo does not error on missing data files; the template's `range` simply iterates zero items. No empty placeholder file needed.

**Alternatives considered**:
- Committing an empty `data/projects.json` (`[]`) — rejected per Constitution XIII.
- Adding a `.gitkeep` in `data/` — unnecessary since Hugo handles the absence gracefully.

## R8: Diagram Code Block Rewriting (Kroki vs Client-Side Mermaid)

**Decision**: Rewrite all upstream diagram code blocks (mermaid, plantuml, d2, graphviz/dot, ditaa, and 12 other languages) from their native fenced format (`` ```mermaid ``) to Kroki format (`` ```kroki {type=mermaid} ``) during content sync. The `dot` alias is normalised to `graphviz` for Kroki compatibility.

**Rationale**: Doks ships two relevant render hooks:
- `render-codeblock-mermaid.html` — renders mermaid diagrams via client-side JavaScript
- `render-codeblock-kroki.html` — renders diagrams server-side via the Kroki API (`krokiURL` in `params.toml`)

Routing mermaid through Kroki instead of the native mermaid hook avoids adding client-side JavaScript to diagram rendering, upholding Constitution V (No Runtime JavaScript Frameworks). Kroki also supports 17 diagram languages (plantuml, d2, graphviz, ditaa, etc.) that have no Doks render hook at all, so a single rewrite handles all diagram types uniformly.

The transform is applied unconditionally in `processRepo` and `syncRepoDocPages` (all org-discovered content), and conditionally in `syncConfigSource` (gated by `rewrite_diagrams` config flag).

**Infrastructure dependency**: Requires `krokiURL = "https://kroki.io"` in `config/_default/params.toml` and the `@thulite/doks-core` module mount (which provides `render-codeblock-kroki.html`). Both are already configured.

**Alternatives considered**:
- Leaving native `` ```mermaid `` blocks as-is — rejected because Doks' `render-codeblock-mermaid.html` uses client-side JS (violates Constitution V), and non-mermaid diagrams (plantuml, d2, etc.) would not render at all.
- Adding custom Hugo render hooks per diagram language — rejected per Constitution I (no theme forking) and XIV (simplicity). Kroki handles all languages through one hook.
- Client-side Kroki rendering via JavaScript — rejected per Constitution V.
