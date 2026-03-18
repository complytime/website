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
| Sync project content       | `go run ./cmd/sync-content --org complytime --config sync-config.yaml --write` |
| Configure file sync        | `sync-config.yaml`                                |
| Run sync tool tests        | `go test -race ./cmd/sync-content/...`            |

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
go run ./cmd/sync-content --org complytime --config sync-config.yaml --write

# 4. Start the dev server (live reload)
npm run dev
```

The site will be available at **http://localhost:1313/**. Hugo's dev server
watches for file changes and rebuilds automatically.

> **Note:** Step 3 fetches README content and metadata from the `complytime`
> GitHub org. Set the `GITHUB_TOKEN` environment variable for higher API rate
> limits. Without it, unauthenticated requests are limited to 60/hour.

### Other Useful Commands

```bash
# Production build (output → public/)
npm run build

# Preview the production build
npm run preview

# Format files with Prettier
npm run format

# Sync content in dry-run mode (preview without writing)
go run ./cmd/sync-content --org complytime --config sync-config.yaml

# Run Go tests
go test -race ./cmd/sync-content/...
```

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
2. Run the sync tool: `go run ./cmd/sync-content --org complytime --config sync-config.yaml --write`
3. The repo will automatically get a section index, overview page, and landing
   page card

For repos needing custom file sync with transforms (e.g., specific doc pages
with injected frontmatter), add a source entry in `sync-config.yaml`. See
`specs/006-go-sync-tool/quickstart.md` for details.

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
// Brand colors — change these to re-theme the site
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
- [ ] If Go code was changed: `go vet ./...` and `gofmt -l ./cmd/sync-content/` are clean
- [ ] If Go code was changed: `go test -race ./cmd/sync-content/...` passes
- [ ] Commit messages follow conventional format
- [ ] DCO sign-off is present

---

## Troubleshooting

### `npm run dev` fails with Hugo errors

Make sure you're on Node.js ≥ 20.11.0:

```bash
node --version
```

If Hugo isn't found, it's installed as part of the npm dependencies. Try:

```bash
rm -rf node_modules
npm install
npm run dev
```

### Changes not showing up in the browser

Hugo's dev server uses live reload. If it's not picking up changes:

1. Check the terminal for build errors
2. Try stopping and restarting `npm run dev`
3. For SCSS changes, a hard refresh (Cmd+Shift+R / Ctrl+Shift+R) may be needed

### Image processing errors

If you see errors about image processing (especially for remote images or SVGs),
the custom render hook in `layouts/_default/_markup/render-image.html` should
handle most cases. If it doesn't:

- Remote images: Use absolute URLs (`https://...`)
- Local SVGs: Place them in `assets/` or a page bundle
- The error level is set to `ignore` in `params.toml`, so most issues are
  silently skipped

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
