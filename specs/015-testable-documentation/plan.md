# Testable Documentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build testable documentation infrastructure so fenced code blocks in Markdown can be extracted, tested with Bats, and coverage-reported.

**Architecture:** A Go extraction tool (`cmd/doctest/`) parses Markdown with goldmark, finds fenced code blocks annotated with `{test="..."}`, writes them to disk. Hand-written Bats tests run those snippets. Coverage reporting flags untested executable blocks. Everything integrates into existing Makefile targets and CI workflows.

**Tech Stack:** Go 1.25+ (goldmark, go-yaml), Bats-core + bats-support + bats-assert (git submodules), Hugo 0.155.1 extended (existing), GNU Make.

**Spec:** `specs/015-testable-documentation/spec.md`

---

## File Map

### New files

| Path | Purpose |
| ------ | --------- |
| `cmd/doctest/main.go` | CLI entry point: `extract` and `coverage` subcommands |
| `cmd/doctest/extract.go` | Markdown walker, attribute parser, snippet writer, manifest generator |
| `cmd/doctest/extract_test.go` | Table-driven unit tests for extraction and coverage |
| `tests/docs/setup_suite.bash` | Bats suite setup: sets `SNIPPETS_DIR` |
| `tests/docs/helpers/bash.bash` | `run_snippet()` helper function |
| `tests/docs/getting-started.bats` | Skeleton Bats test file (placeholder for future snippet tests) |
| `tests/libs/bats-core/` | Git submodule |
| `tests/libs/bats-support/` | Git submodule |
| `tests/libs/bats-assert/` | Git submodule |

### Modified files

| Path | Change |
| ------ | -------- |
| `go.mod` | Add `github.com/yuin/goldmark` dependency |
| `go.sum` | Updated by `go mod tidy` |
| `Makefile` | Add `test-docs-extract`, `test-docs`, `test-docs-coverage` targets; update `check` |
| `.gitignore` | Add `/doctest` binary and `.test-output/` directory |
| `.github/workflows/ci.yml` | Add `checkout submodules`, doc tests step after Hugo build |
| `.github/workflows/deploy-gh-pages.yml` | Add `checkout submodules`, doc tests step before artifact upload |
| `CONTRIBUTING.md` | New "Testing Documentation" section |
| `README.md` | Add `cmd/doctest/` and `tests/docs/` to project structure; mention `make test-docs` |

---

## Task Dependency Graph

```text
Task 1 (Go extractor core + coverage)
  └─> Task 2 (Go extractor tests)
        └─> Task 3 (Bats submodules + harness)
              └─> Task 4 (Makefile targets)
                    └─> Task 5 (CI integration)
                          └─> Task 6 (Documentation updates)
                                └─> Task 7 (.gitignore + housekeeping)
                                      └─> Task 8 (End-to-end verification)
```

Tasks 3 and 7 can be parallelized with adjacent tasks if using subagent-driven-development (they have no code dependencies on each other beyond ordering).

---

### Task 1: Go Extraction Tool — Core Implementation

**Files:**
- Create: `cmd/doctest/main.go`
- Create: `cmd/doctest/extract.go`
- Modify: `go.mod` (add goldmark dependency)

**Context:** The existing `cmd/sync-content/` tool is a `package main` with all files in one directory. Follow the same pattern. The project already uses `github.com/goccy/go-yaml` for YAML parsing. Use goldmark (`github.com/yuin/goldmark`) for Markdown AST parsing — the same parser Hugo uses internally.

- [ ] **Step 1: Add goldmark dependency**

```bash
go get github.com/yuin/goldmark@latest
go mod tidy
```

Verify `go.mod` now contains `github.com/yuin/goldmark`.

- [ ] **Step 2: Create `cmd/doctest/main.go`**

Create `cmd/doctest/main.go` with the CLI entry point:

See the implemented source in [`cmd/doctest/main.go`](../../cmd/doctest/main.go) for the authoritative version; the design intent is described in the prose above.

- [ ] **Step 3: Create `cmd/doctest/extract.go`**

Create `cmd/doctest/extract.go` with the extraction and coverage logic:

See the implemented source in [`cmd/doctest/extract.go`](../../cmd/doctest/extract.go) for the authoritative version; the design intent is described in the prose above.

- [ ] **Step 4: Verify compilation**

```bash
go build ./cmd/doctest/
```

Expected: no errors, no output. A `doctest` binary appears in the workspace root.

```bash
rm -f doctest
```

- [ ] **Step 5: Smoke test against existing content**

```bash
go run ./cmd/doctest extract --content-dir content/docs --output-dir .test-output/doctest-snippets
```

Expected: completes with exit 0. Since no blocks have `{test="..."}` yet, `.test-output/doctest-snippets` should be empty or not created.

```bash
go run ./cmd/doctest coverage --content-dir content/docs
```

Expected: lists untested bash/sh blocks from `getting-started/_index.md`.

- [ ] **Step 6: Commit**

```bash
git add cmd/doctest/main.go cmd/doctest/extract.go go.mod go.sum
git commit -m "feat: add doctest extraction tool

Go tool using goldmark to parse Markdown AST and extract fenced code
blocks annotated with {test=\"...\"} attributes. Supports extract and
coverage subcommands.

Part of testable documentation infrastructure (spec 015)."
```

---

### Task 2: Go Extraction Tool — Unit Tests

**Files:**
- Create: `cmd/doctest/extract_test.go`

**Context:** Follow the existing test patterns from `cmd/sync-content/path_test.go` — table-driven tests, `t.TempDir()` for filesystem tests, `testing` package only (no external test framework). The file under test is `cmd/doctest/extract.go` which exports `parseFrontmatter`, `isOptedOut`, `parseInfoString`, `extractBlocks`, `pageSlug`, `lineNumber`, `langExtension`, `runExtract`, `runCoverage`.

- [ ] **Step 1: Create `cmd/doctest/extract_test.go`**

See the implemented source in [`cmd/doctest/extract_test.go`](../../cmd/doctest/extract_test.go) for the authoritative version; the design intent is described in the prose above.

- [ ] **Step 2: Run tests to verify they pass**

```bash
go test -v ./cmd/doctest/...
```

Expected: all tests pass.

- [ ] **Step 3: Run tests with race detector**

```bash
go test -race ./cmd/doctest/...
```

Expected: passes with zero race warnings.

- [ ] **Step 4: Commit**

```bash
git add cmd/doctest/extract_test.go
git commit -m "test: add unit tests for doctest extraction tool

Table-driven tests covering info string parsing, frontmatter opt-out,
block extraction, ordering, duplicate detection, page slug derivation,
language extensions, manifest generation, and coverage reporting."
```

---

### Task 3: Bats Test Harness Setup

**Files:**
- Create: `tests/docs/setup_suite.bash`
- Create: `tests/docs/helpers/bash.bash`
- Create: `tests/docs/getting-started.bats`
- Create: `tests/libs/` (git submodules)
- Modify: `.gitmodules` (created by `git submodule add`)

**Context:** Bats-core, bats-support, and bats-assert are installed as git submodules. This is the standard distribution pattern — no brew/npm dependency. The `tests/` directory does not exist yet. The project has no `.gitmodules` file yet.

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p tests/docs/helpers tests/libs
```

- [ ] **Step 2: Add Bats git submodules**

```bash
git submodule add https://github.com/bats-core/bats-core.git tests/libs/bats-core
git submodule add https://github.com/bats-core/bats-support.git tests/libs/bats-support
git submodule add https://github.com/bats-core/bats-assert.git tests/libs/bats-assert
```

This creates `.gitmodules` and clones the repos into `tests/libs/`.

- [ ] **Step 3: Create `tests/docs/setup_suite.bash`**

```bash
# SPDX-License-Identifier: Apache-2.0

# Suite-level setup for documentation tests.
# Sets the default snippets directory. Individual .bats files load
# their own libraries in setup().

export SNIPPETS_DIR="${SNIPPETS_DIR:-.test-output/doctest-snippets}"
```

- [ ] **Step 4: Create `tests/docs/helpers/bash.bash`**

```bash
# SPDX-License-Identifier: Apache-2.0

# Helper for running extracted bash snippets.
# Usage in a @test block:
#   run_snippet "getting-started/01-install-complyctl.bash"

run_snippet() {
    local snippet="$SNIPPETS_DIR/$1"
    [[ -f "$snippet" ]] || { echo "Snippet not found: $snippet" >&2; return 1; }
    run bash "$snippet"
}
```

- [ ] **Step 5: Create `tests/docs/getting-started.bats`**

```bash
# SPDX-License-Identifier: Apache-2.0

# Documentation tests for content/docs/getting-started/_index.md
#
# Tests will be added here as code blocks in the getting started guide
# are annotated with {test="..."} attributes. Each @test name must
# match a test attribute value in the Markdown source.

setup() {
    load 'helpers/bash'
    load '../libs/bats-support/load'
    load '../libs/bats-assert/load'
}

# Placeholder: add @test blocks as snippets are annotated.
# Example:
#
# @test "install-complyctl" {
#     run_snippet "getting-started/01-install-complyctl.bash"
#     assert_success
# }
```

- [ ] **Step 6: Verify Bats runs (expect no tests)**

```bash
tests/libs/bats-core/bin/bats tests/docs/
```

Expected: `0 tests, 0 failures` or similar output indicating no test functions found. The skeleton file has no `@test` blocks, so Bats should report zero tests.

- [ ] **Step 7: Commit**

```bash
git add .gitmodules tests/libs/bats-core tests/libs/bats-support tests/libs/bats-assert
git add tests/docs/setup_suite.bash tests/docs/helpers/bash.bash tests/docs/getting-started.bats
git commit -m "feat: add Bats test harness for documentation tests

Install bats-core, bats-support, and bats-assert as git submodules.
Set up test directory structure with helpers and a skeleton test file
for the getting-started guide.

Part of testable documentation infrastructure (spec 015)."
```

---

### Task 4: Makefile Targets

**Files:**
- Modify: `Makefile`

**Context:** The Makefile is 153 lines. Sections are separated by comment headers. The `check` target is on line 77. New doc test targets go after the "Content sync" section (after line 108). The convention uses `##` for help comments, `.PHONY` before each target, and `@` prefix for quiet execution.

- [ ] **Step 1: Add documentation test section to Makefile**

Insert after line 108 (after the `sync-single` target), before the "Hugo / Node" section:

<!-- markdownlint-disable MD010 -->
```makefile
# ---------------------------------------------------------------------------
# Documentation tests — extract, validate, and test code blocks
# ---------------------------------------------------------------------------

.PHONY: test-docs-extract
test-docs-extract: ## Extract testable code blocks from documentation
	@go run ./cmd/doctest extract --content-dir content/docs --output-dir .test-output/doctest-snippets

.PHONY: test-docs
test-docs: test-docs-extract ## Run documentation tests (Bats)
	@tests/libs/bats-core/bin/bats tests/docs/

.PHONY: test-docs-coverage
test-docs-coverage: ## Report untested code blocks in documentation
	@go run ./cmd/doctest coverage --content-dir content/docs
```
<!-- markdownlint-enable MD010 -->

- [ ] **Step 2: Update `check` meta-target**

Change line 77 from:

```makefile
check: vet fmt-check test-race ## Run vet + fmt-check + race tests (CI equivalent)
```

to:

```makefile
check: vet fmt-check test-race test-docs-coverage ## Run vet + fmt-check + race tests + doc coverage (CI equivalent)
```

Also update the quick reference comment at the top of the Makefile (line 11) to mention doc tests:

```makefile
#   make check           — vet + fmt-check + race tests + doc coverage
```

- [ ] **Step 3: Update Go targets to include doctest**

The existing `test` and `test-race` targets only test `./cmd/sync-content/...`. Update them to also test `./cmd/doctest/...`:

Change `test` (line 53):
<!-- markdownlint-disable MD010 -->
```makefile
test: ## Run all Go unit tests
	go test $(SYNC_PKG) ./cmd/doctest/...
```
<!-- markdownlint-enable MD010 -->

Change `test-race` (line 57):
<!-- markdownlint-disable MD010 -->
```makefile
test-race: ## Run Go tests with the race detector
	go test -race $(SYNC_PKG) ./cmd/doctest/...
```
<!-- markdownlint-enable MD010 -->

Also update `vet` (line 61) and `fmt`/`fmt-check` to cover the new package:

Change `vet`:
<!-- markdownlint-disable MD010 -->
```makefile
vet: ## Run go vet
	go vet $(SYNC_PKG) ./cmd/doctest/...
```
<!-- markdownlint-enable MD010 -->

Change `fmt`:
<!-- markdownlint-disable MD010 -->
```makefile
fmt: ## Format Go source files with gofmt
	gofmt -w cmd/sync-content/ cmd/doctest/
```
<!-- markdownlint-enable MD010 -->

Change `fmt-check`:
<!-- markdownlint-disable MD010 -->
```makefile
fmt-check: ## Check Go formatting (non-destructive)
	@out=$$(gofmt -l cmd/sync-content/ cmd/doctest/); \
	if [ -n "$$out" ]; then \
	echo "The following files need formatting:"; \
	echo "$$out"; \
	exit 1; \
	fi
```
<!-- markdownlint-enable MD010 -->

- [ ] **Step 4: Verify targets work**

```bash
make help
```

Expected: new targets `test-docs-extract`, `test-docs`, `test-docs-coverage` appear in help output.

```bash
make test-docs-extract
```

Expected: exit 0 (no annotated blocks to extract yet).

```bash
make test-docs-coverage
```

Expected: lists untested bash blocks from getting-started page.

```bash
make test
```

Expected: runs both sync-content and doctest Go tests.

```bash
make check
```

Expected: runs vet, fmt-check, race tests, and doc coverage.

- [ ] **Step 5: Commit**

```bash
git add Makefile
git commit -m "feat: add Makefile targets for documentation testing

Add test-docs-extract, test-docs, and test-docs-coverage targets.
Include doctest package in existing Go test/vet/fmt targets.
Add test-docs-coverage to check meta-target."
```

---

### Task 5: CI Integration

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/deploy-gh-pages.yml`

**Context:** Both workflows use `actions/checkout` with SHA-pinned versions and `persist-credentials: false`. Neither currently checks out submodules. The Bats submodules in `tests/libs/` must be available for `make test-docs` to work. The `ci.yml` checkout is at line 16-18, `deploy-gh-pages.yml` at line 22-24.

- [ ] **Step 1: Update `ci.yml` — add submodule checkout and doc tests**

In `.github/workflows/ci.yml`, update the checkout step to include submodules, and add a doc tests step after the Hugo build:

Update the Checkout step (lines 16-18) to:
```yaml
      - name: Checkout
        uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0
        with:
          persist-credentials: false
          submodules: true
```

Add after the "Build site" step (after line 48):
```yaml
      - name: Run documentation tests
        run: make test-docs
```

- [ ] **Step 2: Update `deploy-gh-pages.yml` — add submodule checkout and doc tests**

In `.github/workflows/deploy-gh-pages.yml`, update the checkout step to include submodules, and add a doc tests step after the Hugo build:

Update the Checkout step (lines 22-24) to:
```yaml
      - name: Checkout
        uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0
        with:
          persist-credentials: false
          submodules: true
```

Add after the "Build" step (after line 66):
```yaml
      - name: Run documentation tests
        run: make test-docs
```

- [ ] **Step 3: Also update Go test step in `ci.yml` to include doctest**

Update line 40 from:
```yaml
      - name: Run tests
        run: go test -race ./cmd/sync-content/...
```

to:
```yaml
      - name: Run tests
        run: go test -race ./cmd/sync-content/... ./cmd/doctest/...
```

And in `deploy-gh-pages.yml`, update line 57-58 from:
```yaml
      - name: Run tests
        run: go test -race ./cmd/sync-content/...
```

to:
```yaml
      - name: Run tests
        run: go test -race ./cmd/sync-content/... ./cmd/doctest/...
```

- [ ] **Step 4: Verify YAML validity**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml')); print('ci.yml OK')"
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/deploy-gh-pages.yml')); print('deploy-gh-pages.yml OK')"
```

Expected: both print OK.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/ci.yml .github/workflows/deploy-gh-pages.yml
git commit -m "ci: add documentation test steps to CI workflows

Check out git submodules (bats-core, bats-support, bats-assert) and
run make test-docs in both ci.yml and deploy-gh-pages.yml. Include
cmd/doctest in Go race-test steps."
```

---

### Task 6: Documentation Updates

**Files:**
- Modify: `CONTRIBUTING.md`
- Modify: `README.md`
- Create: `AGENTS.md` (if the project wants one; otherwise skip)

**Context:** `CONTRIBUTING.md` is 537 lines with a clear section structure. The spec calls for a new "Testing Documentation" section after existing testing sections. `README.md` is 49 lines. The project has no `AGENTS.md` file (only `.agents/skills/.gitkeep`).

- [ ] **Step 1: Add "Testing Documentation" section to CONTRIBUTING.md**

Insert after the "Testing the Sync Tool" section (after line 476, before the "### Testing Tips" section). Add:

```markdown
### Testing Documentation

Documentation pages with shell commands use **testable code blocks** — fenced
code blocks annotated with a `{test="..."}` attribute that links them to
automated tests.

**Annotating a code block:**

<!-- markdownlint-disable MD040 -->
````markdown
```bash {test="install-complyctl"}
go install github.com/complytime/complyctl@latest
```
````
<!-- markdownlint-enable MD040 -->


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
| `make test-docs-extract` | Extract annotated code blocks to `.test-output/doctest-snippets` |
| `make test-docs` | Extract + run Bats tests |
| `make test-docs-coverage` | Report untested executable code blocks (warnings only) |

Coverage warnings are non-blocking — they show which blocks could benefit from
test annotations but do not fail the build.
```

- [ ] **Step 2: Update CONTRIBUTING.md table of contents**

Add "Testing Documentation" to the table of contents under "Common Tasks":

After the line `  - [Add Images](#add-images)` (line 23), the existing TOC does not have a "Testing Documentation" entry. Find the "Development Workflow" section entry and add the new entry in the appropriate location. Insert under the existing testing items at the right nesting level.

Actually, looking at the TOC structure, "Testing the Sync Tool" is under "Development Workflow". Add "Testing Documentation" after it:

Find the line:
```
- [Troubleshooting](#troubleshooting)
```

And add before it (under Development Workflow):
```
  - [Testing Documentation](#testing-documentation)
```

Wait — looking more carefully, the TOC items for Development Workflow sub-sections are not listed in the TOC. The "Testing the Sync Tool" section is not in the TOC either. So just add the section content and it will be discoverable by scrolling or heading search.

- [ ] **Step 3: Update CONTRIBUTING.md PR checklist**

Add a checklist item for documentation testing. Find the PR checklist section (around line 399) and add:

```markdown
- [ ] If documentation code blocks were changed: `make test-docs` passes
```

- [ ] **Step 4: Update README.md project structure**

In `README.md`, update the project structure tree (lines 23-33) to include the new directories:

```
website/
├── cmd/sync-content/      # Go content sync tool (10 source files, package main)
├── cmd/doctest/           # Go documentation test extraction tool
├── config/_default/       # Hugo configuration (TOML)
├── content/docs/          # Markdown content (projects/ is generated by sync tool)
├── data/projects.json     # Generated landing page cards (gitignored)
├── layouts/               # Custom Hugo layout overrides
├── tests/docs/            # Bats documentation tests
├── tests/libs/            # Bats test libraries (git submodules)
├── sync-config.yaml       # Declarative sync configuration
├── .content-lock.json     # Approved upstream SHAs per repo (committed)
└── .github/workflows/     # CI, deploy, weekly content check
```

- [ ] **Step 5: Update README.md quick start / development info**

After the "Production build" line (line 19), add:

```markdown
**Documentation tests**: `make test-docs` extracts annotated code blocks and runs Bats tests against them.
```

- [ ] **Step 6: Create AGENTS.md**

Create `AGENTS.md` in the project root with guidance for AI agents:

```markdown
# Agent Instructions

## Documentation with Shell Commands

When editing documentation that contains shell commands:

1. **Always add `{test="..."}` attributes** to fenced code blocks that contain
   runnable shell commands. Use lowercase alphanumeric identifiers with hyphens.

2. **Write a corresponding Bats test** in `tests/docs/` before fixing a snippet
   (TDD for docs). The test name must match the `test` attribute value.

3. **Run `make test-docs-coverage`** to check for untested code blocks.

4. **Run `make test-docs`** to verify all annotated snippets pass their tests.

## Go Code

- Run `make check` before committing (includes `go vet`, `gofmt`, race tests,
  and doc coverage).
- Follow existing patterns in `cmd/sync-content/` for test structure.
```

- [ ] **Step 7: Commit**

```bash
git add CONTRIBUTING.md README.md AGENTS.md
git commit -m "docs: add testable documentation workflow to contributor guides

Add Testing Documentation section to CONTRIBUTING.md with annotation
convention, Bats test examples, and make targets. Update README.md
project structure. Create AGENTS.md with documentation testing guidance."
```

---

### Task 7: .gitignore and Housekeeping

**Files:**
- Modify: `.gitignore`

**Context:** The `.gitignore` is 60 lines. Extracted snippets go to `.test-output/doctest-snippets`, an in-repo directory that must be gitignored so generated snippets never get committed. The `doctest` binary (if built locally) should also be ignored, similar to the existing `/sync-content` ignore on line 13.

- [ ] **Step 1: Add doctest binary to .gitignore**

After the existing line `/sync-content` (line 13), add:

```gitignore
/doctest
```

Also add a test output directory entry:

```gitignore
# ─── Test output ─────────────────────────────────────────────────────
.test-output/
```

- [ ] **Step 2: Commit**

```bash
git add .gitignore
git commit -m "chore: gitignore doctest binary and test output directory"
```

---

### Task 8: End-to-End Verification

**Files:** None created or modified. This task verifies the full pipeline.

**Context:** At this point all code is written and committed. This task runs through the complete workflow to verify everything works together.

- [ ] **Step 1: Run Go tests for doctest**

```bash
go test -v ./cmd/doctest/...
```

Expected: all tests pass.

- [ ] **Step 2: Run Go tests with race detector**

```bash
go test -race ./cmd/doctest/...
```

Expected: passes with zero data race warnings.

- [ ] **Step 3: Run full check**

```bash
make check
```

Expected: vet, fmt-check, race tests (sync-content + doctest), and doc coverage all pass. Coverage reports untested bash blocks in getting-started page.

- [ ] **Step 4: Run extraction**

```bash
make test-docs-extract
```

Expected: exit 0. `.test-output/doctest-snippets` should be empty (no annotated blocks yet).

- [ ] **Step 5: Run doc tests**

```bash
make test-docs
```

Expected: exit 0. Bats reports 0 tests (skeleton file has no `@test` blocks).

- [ ] **Step 6: Run coverage**

```bash
make test-docs-coverage
```

Expected: lists untested bash/sh blocks from getting-started page with file, line, language.

- [ ] **Step 7: Verify Hugo still renders correctly**

```bash
hugo --minify --gc 2>&1 | head -5
```

Expected: Hugo builds successfully. The `{test="..."}` attributes don't exist on any blocks yet, but verify Hugo configuration is still valid.

- [ ] **Step 8: Verify Go formatting**

```bash
make fmt-check
```

Expected: no unformatted files.

- [ ] **Step 9: Review all commits**

```bash
git log --oneline feat/testable-documentaiton ^main
```

Expected: 7 commits (Tasks 1-7) in logical order with conventional commit messages.
