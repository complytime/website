# Contributing to the ComplyTime Website

Welcome! This guide covers everything you need to know to contribute to the
[ComplyTime website](https://complytime.dev). Whether you're fixing a typo,
adding a docs page, or tweaking the homepage layout, this document will get you
oriented quickly.

---

## Table of Contents

- [Quick Reference](#quick-reference)
- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [Project Structure](#project-structure)
- [Common Tasks](#common-tasks)
  - [Add or Edit a Documentation Page](#add-or-edit-a-documentation-page)
  - [Add a New Project Page](#add-a-new-project-page)
  - [Change Navigation Menus](#change-navigation-menus)
  - [Modify the Homepage](#modify-the-homepage)
  - [Change Colors or Typography](#change-colors-or-typography)
  - [Add Images](#add-images)
- [Hugo and Doks Theme Basics](#hugo-and-doks-theme-basics)
  - [Template Lookup Order](#template-lookup-order)
  - [Custom Overrides Already in Place](#custom-overrides-already-in-place)
  - [The Module Mount System](#the-module-mount-system)
- [CI/CD and Deployment](#cicd-and-deployment)
- [Coding Conventions](#coding-conventions)
- [Pull Request Process](#pull-request-process)
- [Development Workflow](#development-workflow)
- [Troubleshooting](#troubleshooting)

---

## Quick Reference

| Task                       | What to Edit                                      |
|----------------------------|---------------------------------------------------|
| Edit a docs page           | `content/docs/<section>/<page>.md`                |
| Add a new docs page        | Create a `.md` file under `content/docs/`         |
| Change site navigation     | `config/_default/menus/menus.en.toml`             |
| Change theme colors        | `assets/scss/common/_variables-custom.scss`        |
| Add custom CSS             | `assets/scss/common/_custom.scss`                  |
| Edit the homepage          | `layouts/home.html` + `content/_index.md`          |
| Site-wide Hugo settings    | `config/_default/hugo.toml`                        |
| Theme/feature parameters   | `config/_default/params.toml`                      |
| Sync project content       | `make sync` (or `go run ./cmd/sync-content --org complytime --config sync-config.yaml --write`) |
| Sync single repo           | `make sync-single REPO=complytime/<name>`         |
| Dry-run sync (no writes)   | `make sync-dry`                                   |
| Configure file sync        | `sync-config.yaml`                                |
| Run sync tool tests        | `make test-race` (or `go test -race ./cmd/sync-content/...`) |
| Run all Go checks (CI)     | `make check`                                      |

---

## Prerequisites

| Tool       | Version   | Notes                                     |
|------------|-----------|-------------------------------------------|
| **Node.js** | ≥ 22      | Required by Hugo/Doks pipeline            |
| **npm**    | (bundled) | Comes with Node.js                        |
| **Hugo**   | ≥ 0.155.1 | Extended edition; installed via npm scripts |
| **Go**     | ≥ 1.25    | Required for the content sync tool        |
| **Git**    | Any recent | For cloning and version control            |

> **Tip:** If you only want to edit Markdown content, Node.js + npm is all you
> need. Go is only required if you need to run the sync tool or its tests.

---

## Getting Started

```bash
# 1. Clone the repository
git clone https://github.com/complytime/website.git
cd website

# 2. Install dependencies
npm install

# 3. Sync project content from GitHub (generates project pages and cards)
make sync
# Equivalent without make: go run ./cmd/sync-content --org complytime --config sync-config.yaml --write

# 4. Start the dev server (live reload)
npm run dev
```

The site will be available at **http://localhost:1313/**. Hugo's dev server
watches for file changes and rebuilds automatically.

> **Note:** Step 3 fetches README content and metadata from the `complytime`
> GitHub org. Set the `GITHUB_TOKEN` environment variable for higher API rate
> limits. Without it, unauthenticated requests are limited to 60/hour.

---

## Project Structure

```
website/
├── assets/                          # Processed assets (SCSS, JS, images)
│   ├── js/custom.js                 #   Custom JavaScript (currently empty)
│   └── scss/common/
│       ├── _variables-custom.scss   #   Theme colors, fonts, card styles
│       └── _custom.scss             #   Additional custom CSS
│
├── cmd/sync-content/                # Go content sync tool (package main, 10 source files)
│   ├── main.go                      #   Entry point and orchestration (~440 lines)
│   ├── config.go                    #   Config types and loading (incl. Peribolos types)
│   ├── github.go                    #   GitHub API client and types (incl. peribolos fetch)
│   ├── transform.go                 #   Markdown transforms (links, badges, headings)
│   ├── hugo.go                      #   Hugo page and card generation
│   ├── sync.go                      #   Sync logic, result tracking, repo processing
│   ├── manifest.go                  #   Manifest I/O and state tracking
│   ├── cleanup.go                   #   Orphan and stale content removal
│   ├── path.go                      #   Path validation utilities
│   ├── lock.go                      #   Content lockfile read/write/query
│   └── *_test.go                    #   Tests mirror source files (10 files, ~2300 lines)
│
├── config/_default/                 # Hugo configuration (TOML)
│   ├── hugo.toml                    #   Core Hugo settings
│   ├── params.toml                  #   Doks theme parameters
│   ├── markup.toml                  #   Markdown rendering settings
│   ├── module.toml                  #   Hugo module mounts
│   └── menus/menus.en.toml          #   Navigation menu definitions
│
├── content/                         # Markdown content (this is what you edit!)
│   ├── _index.md                    #   Homepage frontmatter
│   ├── privacy.md                   #   Privacy policy
│   └── docs/
│       ├── getting-started/         #   Getting started guide
│       └── projects/                #   Project pages (generated by sync tool)
│           ├── _index.md            #   Hand-maintained section index (committed)
│           └── {repo}/              #   Generated per-repo content (gitignored)
│
├── data/
│   └── projects.json                #   Generated landing page cards (gitignored)
│
├── layouts/                         # Custom Hugo layout overrides
│   ├── home.html                    #   Homepage template (hero + features)
│   ├── docs/list.html               #   Docs section listing template
│   └── _default/_markup/
│       └── render-image.html        #   Custom image render hook
│
├── sync-config.yaml                 #   Declarative file sync manifest
├── .content-lock.json               #   Approved upstream SHAs per repo (committed)
├── go.mod                           #   Go module (sync tool)
├── static/                          # Static assets (copied as-is to output)
├── images/                          # Project logos and illustrations
└── package.json                     # Node.js dependencies and scripts
```

### What's What

| Directory/File              | Purpose | How Often You'll Touch It |
|-----------------------------|---------|---------------------------|
| `content/docs/**/*.md`      | All documentation pages | **Very often** |
| `config/_default/menus/`    | Navigation menus | Occasionally |
| `assets/scss/common/`       | Styling (colors, layout) | Occasionally |
| `layouts/home.html`         | Homepage layout | Rarely |
| `layouts/docs/list.html`    | Docs section template | Rarely |
| `config/_default/params.toml` | Theme feature flags | Rarely |

---

## Common Tasks

### Add or Edit a Documentation Page

All documentation lives in `content/docs/`. Each `.md` file needs **YAML
frontmatter** at the top:

```markdown
---
title: "Your Page Title"
description: "A short description for SEO and previews."
lead: "A brief intro shown at the top of the page."
date: 2024-01-01T00:00:00+00:00
lastmod: 2024-12-24T00:00:00+00:00
draft: false
weight: 100
toc: true
---

Your Markdown content goes here...
```

**Key fields:**
- **`weight`** controls page ordering in sidebar navigation (lower = higher).
- **`toc: true`** generates a table of contents from your headings.
- **`draft: true`** hides the page from the built site.

**Section index pages** (the landing page for a folder) must be named
`_index.md`.

### Add a New Project Page

Project pages under `content/docs/projects/` are **automatically generated** by
the sync tool from GitHub org repositories. You do not need to create them
manually. To add a new project:

1. Create the repository in the `complytime` GitHub org
2. Run the sync tool: `make sync` (or `go run ./cmd/sync-content --org complytime --config sync-config.yaml --write`)
3. The repo will automatically get a section index, overview page, and landing
   page card

For repos needing custom file sync with transforms (e.g., specific doc pages
with injected frontmatter), add a source entry in `sync-config.yaml`. See
[cmd/sync-content/README.md](cmd/sync-content/README.md#configuration) for the
config format.

### Change Navigation Menus

Edit `config/_default/menus/menus.en.toml`:

```toml
# Top-level docs sidebar navigation
[[docs]]
  name = "New Section"
  weight = 50          # Controls ordering
  identifier = "new-section"
  url = "/docs/new-section/"

# Main navbar links
[[main]]
  name = "New Link"
  url = "/docs/new-section/"
  weight = 50
```

### Modify the Homepage

The homepage is built from two files:

1. **`content/_index.md`** — Frontmatter only (title, description, lead text)
2. **`layouts/home.html`** — The full HTML template with three sections:
   - `{{ define "main" }}` — Hero section (badge, title, CTA buttons)
   - `{{ define "sidebar-prefooter" }}` — Feature cards + project cards + CTA
   - `{{ define "sidebar-footer" }}` — Bottom CTA banner

The homepage uses Bootstrap 5 grid classes and inline SVG icons. To add or
remove a feature card, look for the `<!-- Features Section -->` comment in
`layouts/home.html` and copy/modify an existing card `div`.

### Change Colors or Typography

Edit `assets/scss/common/_variables-custom.scss`:

```scss
// Brand colors — change these to re-theme the sitee
$cyan-600: #0891b2;   // Primary color
$primary: $cyan-600;

// Typography
$font-family-sans-serif: "DM Sans", system-ui, ...;
$font-family-monospace: "JetBrains Mono", ...;
```

For additional CSS overrides, use `assets/scss/common/_custom.scss`.

### Add Images

- **Project logos and illustrations:** Place in `images/` (processed by Hugo)
- **Static assets (favicons, icons):** Place in `static/` (copied as-is)
- **Page-specific images:** Place alongside the Markdown file in a
  [page bundle](https://gohugo.io/content-management/page-bundles/)

The site includes a **custom image render hook**
(`layouts/_default/_markup/render-image.html`) that:
- Passes remote/absolute URLs straight through as `<img>` tags
- Processes local images through Hugo's image pipeline (resize, webp conversion)
- Handles SVGs without attempting raster processing

This means standard Markdown image syntax works for both local and remote images:

```markdown
![Alt text](./local-image.png)
![Badge](https://img.shields.io/badge/status-passing-green)
```

---

## Hugo and Doks Theme Basics

This site uses [Hugo](https://gohugo.io/) with the
[Doks theme](https://getdoks.org/) (via `@thulite/doks-core`). Understanding
a few Hugo concepts will help if you need to go beyond content editing.

### Template Lookup Order

Hugo resolves templates by specificity. The site uses this layered approach:

1. **`layouts/`** (this repo) — Highest priority, overrides everything
2. **`node_modules/@thulite/doks-core/layouts/`** — Doks theme layouts
3. **`node_modules/@thulite/core/layouts/`** — Thulite base layouts
4. **Other module mounts** (SEO, images, inline-svg)

To override a theme template, copy it from `node_modules/@thulite/doks-core/layouts/`
into the same relative path under `layouts/` and modify it.

### Custom Overrides Already in Place

| File | What It Overrides | Why |
|------|-------------------|-----|
| `layouts/home.html` | Doks default homepage | Custom hero, features, and project cards |
| `layouts/docs/list.html` | Doks docs list | Custom section listing layout |
| `layouts/_default/_markup/render-image.html` | `@thulite/images` render hook | Fixes broken remote SVG/badge images |
| `layouts/_partials/footer/script-footer-custom.html` | (empty hook) | Available for custom footer scripts |

### The Module Mount System

`config/_default/module.toml` defines how Hugo merges files from
`node_modules/` with local directories. **Local files always win** (they appear
first in the mount order). This is how overrides work without modifying
`node_modules/`.

Key detail: `home.html` is explicitly **excluded** from the Doks core mount so
the local `layouts/home.html` is used instead.

---

## CI/CD and Deployment

Three GitHub Actions workflows automate the pipeline:

### PR Validation (`ci.yml`)

- **Trigger:** Pull requests targeting `main`
- **What it does:** Runs `go test -race`, syncs content with `--lock` and
  `--write` (at approved SHAs), and Hugo build to validate changes before merge

### Content Sync Check (`sync-content-check.yml`)

- **Trigger:** Weekly (Monday 06:00 UTC)
- **What it does:** Runs `--update-lock` to detect upstream SHA changes.
  Opens/updates a PR with `.content-lock.json` changes for review

### GitHub Pages Deployment (`deploy-gh-pages.yml`)

- **Trigger:** Push to `main`
- **What it does:** Sets up Go + Node.js + Hugo, runs the sync tool with
  `--lock .content-lock.json --write` to generate content at approved SHAs,
  then builds with `hugo --minify --gc` and uploads to GitHub Pages
- **Pinned actions:** All GitHub Actions use SHA-pinned versions for security

> **Note:** Upstream content changes require a reviewed PR (opened by the weekly
> check workflow) before reaching production. No unreviewed content is deployed.

---

## Coding Conventions

### Markdown Content

- Use **ATX-style headings** (`## Heading`, not underlines)
- Start with `## ` (H2) inside content files — H1 is generated from `title` frontmatter
- Use **fenced code blocks** with language identifiers (` ```bash `, ` ```yaml `, etc.)
- Keep lines at a readable length (no hard limit, but ~100 chars is reasonable)
- Use relative links for internal pages: `[Getting Started](/docs/getting-started/)`

### SCSS

- Custom variables go in `_variables-custom.scss`
- Custom rules go in `_custom.scss`
- Use Bootstrap CSS variables (`var(--bs-*)`) where possible for dark mode
  compatibility

### Hugo Templates

- Use Hugo's `{{ with }}` for nil-safe access
- Prefer `partial` calls for reusable components
- Comment complex template logic with `{{/* ... */}}`

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add new tutorial for C2P mapping
fix: correct broken link in complyctl docs
docs: update getting started prerequisites
chore: sync upstream content
style: fix indentation in home template
```

---

## Pull Request Process

1. **Fork** the repository and create a feature branch
2. Make your changes and test locally (`npm run dev`)
3. Run `npm run build` to verify the production build succeeds
4. **Sign your commits** with DCO: `git commit -s -m "docs: add my page"`
5. Open a Pull Request with a clear description of your changes
6. Ensure the CI checks pass

### PR Checklist

- [ ] Pages render correctly on the dev server
- [ ] No broken links or missing images
- [ ] Frontmatter includes all required fields (`title`, `description`, `weight`)
- [ ] If Go code was changed: `make check` passes (`go vet`, `gofmt`, and `go test -race`)
- [ ] If documentation code blocks were changed: `make test-docs` passes
- [ ] Commit messages follow conventional format
- [ ] DCO sign-off is present

---

## Development Workflow

### Day-to-Day

```bash
npm run dev          # Dev server with live reload
npm run build        # Production build (output → public/)
npm run preview      # Preview the production build
npm run format       # Format files with Prettier
```

### Clean Build (CI Match)

`hugo server` and `hugo --minify --gc` can produce different results. Always
validate with a production build before trusting the dev server:

```bash
rm -rf public/ resources/
npm run build
```

### Full Nuclear Clean

When something looks wrong and a clean build isn't enough — removes cached
Hugo resources, node_modules, and reinstalls from the lockfile:

```bash
rm -rf public/ resources/ /tmp/hugo_cache/ node_modules/
npm ci
npm run build
```

### Testing the Sync Tool

The Makefile provides handy targets for all common sync and Go operations
(run `make help` to see all available targets):

```bash
make sync-dry        # dry-run — reads GitHub, writes nothing
make sync            # apply changes to disk
make sync-single REPO=complytime/complyctl  # single-repo dry-run
make check           # go vet + fmt-check + race tests + doc coverage (CI equivalent)
make test-race       # tests with race detector only
```

The equivalent raw commands, if you prefer not to use make:

Dry-run (preview without writing):

```bash
go run ./cmd/sync-content --org complytime --config sync-config.yaml
```

Full reset and resync (useful when upstream content or sync logic changes):

```bash
rm -f .sync-manifest.json data/projects.json
rm -rf content/docs/projects/*/
rm -rf public/ resources/
go run ./cmd/sync-content --org complytime --config sync-config.yaml --lock .content-lock.json --write
npm run build
```

Run Go tests:

```bash
go test -race ./cmd/sync-content/...
```

If you encounter missing token errors, verify your `GITHUB_TOKEN`:

```bash
export GITHUB_TOKEN="$(gh auth token)"
echo "Token set, length: ${#GITHUB_TOKEN}, prefix: ${GITHUB_TOKEN:0:4}"
go run ./cmd/sync-content --org complytime --config sync-config.yaml --write
```

### Testing Documentation

Documentation pages with shell commands use **testable code blocks** — fenced
code blocks annotated with a `{test="..."}` attribute that links them to
automated tests.

**Annotating a code block:**

````markdown
```bash {test="install-complyctl"}
go install github.com/complytime/complyctl@latest
```
````

The `test` value must be lowercase alphanumeric with hyphens (`[a-z0-9-]+`).
It becomes both the extracted snippet filename and the Bats test reference.
Each value must be unique within a page.

**Writing the corresponding test:**

Create or update a `.bats` file in `tests/docs/` matching the page name:

```bash
# tests/docs/getting-started.bats

@test "install-complyctl" {
    run_snippet "getting-started/01-install-complyctl.bash"
    assert_success
}
```

**Opting out a page:** Add `testable_docs: false` to the page's YAML frontmatter
to skip it entirely from extraction and coverage reporting.

**Make targets:**

| Target | What it does |
|--------|-------------|
| `make test-docs-extract` | Extract annotated code blocks to `/tmp/doctest-snippets` |
| `make test-docs` | Extract + run Bats tests |
| `make test-docs-coverage` | Report untested executable code blocks (warnings only) |

Coverage warnings are non-blocking — they show which blocks could benefit from
test annotations but do not fail the build.

### Testing Tips

- Always test with **browser cache disabled** (DevTools → Network →
  "Disable cache").
- When in doubt, run a [clean build](#clean-build-ci-match) — the dev server
  can mask issues that show up in production.

---

## Troubleshooting

### `npm run dev` fails with Hugo errors

Make sure you're on Node.js ≥ 22. If Hugo isn't found, try a
[full nuclear clean](#full-nuclear-clean).

### Changes not showing up in the browser

1. Check the terminal for build errors
2. Try stopping and restarting `npm run dev`
3. Hard refresh (Cmd+Shift+R / Ctrl+Shift+R) for SCSS changes
4. Test with browser cache disabled (DevTools → Network → "Disable cache")

### Image processing errors

If you see errors about image processing (especially for remote images or SVGs),
the custom render hook in `layouts/_default/_markup/render-image.html` should
handle most cases. If it doesn't:

- Remote images: Use absolute URLs (`https://...`)
- Local SVGs: Place them in `assets/` or a page bundle
- The error level is set to `ignore` in `params.toml`, so most issues are
  silently skipped
e
### Build output confusion: `public/` vs `docs/`

- **`public/`** — Hugo's build output directory (generated, gitignored)
- **`docs/`** — Legacy build output (gitignored in `.gitignore`)
- **`content/docs/`** — The actual source Markdown files you should edit

Only edit files in `content/`. Never edit `public/` or `docs/` directly.

---

## Getting Help

- **Hugo docs:** https://gohugo.io/documentation/
- **Doks theme docs:** https://getdoks.org/
- **ComplyTime community:** https://github.com/complytime/community
- **GitHub Discussions:** https://github.com/orgs/complytime/discussions

Thank you for contributing to the ComplyTime website!
