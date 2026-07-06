# Feature Specification: Testable Documentation

**Feature ID**: `015-testable-documentation`
**Phase**: 3 (Documentation Quality)
**Created**: 2026-07-06
**Status**: Draft
**Input**: 11 open GitHub issues against Getting Started page (broken commands, missing prerequisites, nonexistent file references)

## Overview

The ComplyTime website's hand-maintained documentation contains shell commands that rot silently — broken install commands, references to nonexistent files, missing prerequisites. Eleven open issues against a single page demonstrate the problem. There is no mechanism to detect these failures before users hit them.

This feature adds testable documentation infrastructure: authors annotate fenced code blocks with a `{test="..."}` attribute, a Go extraction tool pulls those blocks out of Markdown, and hand-written Bats tests verify they work. Coverage reporting flags untested executable blocks so gaps are visible.

**Dependencies**: Go 1.25+ (extraction tool), goldmark (Markdown parsing), Bats-core + bats-support + bats-assert (test harness), Hugo 0.155.1 extended (existing). No new CI workflows — integrates into existing `ci.yml` and `deploy-gh-pages.yml`.

## Scope

### In Scope

| ID | Capability |
|----|-----------|
| IS-001 | Markdown authoring convention: `{test="<identifier>"}` attribute on fenced code blocks in the info string |
| IS-002 | Go extraction tool (`cmd/doctest/`) with `extract` and `coverage` subcommands |
| IS-003 | Goldmark-based AST parsing of fenced code blocks with `parser.ParseAttributes()` for info-string attribute extraction |
| IS-004 | Frontmatter-based opt-out: `testable_docs: false` skips entire page |
| IS-005 | Snippet output with numeric ordering: `<page-slug>/<NN>-<test-name>.<lang>` preserving document order |
| IS-006 | `manifest.json` per page mapping test names to source `file:line` for traceability |
| IS-007 | Coverage reporting: list untested executable-language blocks with file, line, and language |
| IS-008 | Duplicate `test` value detection within a page (extractor errors on duplicates) |
| IS-009 | Bats test harness with bats-core, bats-support, bats-assert (npm devDependencies) |
| IS-010 | Helper pattern for language-specific snippet execution (`tests/docs/helpers/bash.bash`) |
| IS-011 | Makefile targets: `test-docs-extract`, `test-docs`, `test-docs-coverage` |
| IS-012 | CI integration: `make test-docs` step in `ci.yml` and `deploy-gh-pages.yml` |
| IS-013 | `check` meta-target updated to include `test-docs-coverage` (non-blocking warnings) |
| IS-014 | Documentation updates: CONTRIBUTING.md, README.md, AGENTS.md |

### Out of Scope

- Fixing existing broken documentation snippets (tracked by individual issues, applied after this infrastructure ships)
- Testing synced/generated content (`content/docs/projects/*/`) — future goal
- Auto-generating Bats test files from extracted snippets
- Block-level opt-out attribute (e.g., `{skip=true}`) — use non-executable language instead
- Strict coverage enforcement in CI (Phase 2 — remove `-` prefix and `|| true` when ready)
- Non-shell language test execution (Go, Python helpers are future additions)

### Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Code block with no info string | Silently skipped — no language, not testable |
| Code block with language but no `{...}` | Flagged in coverage report if language is testable; silently skipped if non-testable |
| Code block with `{test="..."}` but non-testable language | Extracted and written to output dir (enables future expansion); not flagged in coverage |
| Duplicate `test` values within a page | `extract` exits 1 with error identifying both locations |
| Duplicate `test` values across different pages | Allowed — output dirs are per-page |
| Page with `testable_docs: false` | Entirely skipped — no extraction, no coverage warnings |
| Empty code block with `{test="..."}` | Extracted as empty file; Bats test determines pass/fail behavior |
| Info string with `{...}` but no `test` key | Attributes parsed but block treated as unannotated; flagged in coverage if testable language |
| Markdown file with no fenced code blocks | Silently skipped — nothing to extract or report |
| Nested fences (`` ```` `` inside `` ``` ``) | Goldmark handles nesting correctly; inner fences are content, not parsed as blocks |

## Markdown Authoring Convention

Authors annotate fenced code blocks with a `test` attribute in curly braces on the info string:

````markdown
```bash {test="install-complyctl"}
go install github.com/complytime/complyctl@latest
```
````

**Rules:**

- The `test` value must be a valid identifier: lowercase alphanumeric characters and hyphens (`[a-z0-9-]+`). It becomes both the snippet filename and the Bats `@test` reference.
- Each `test` value must be unique within a page. The extractor errors on duplicates.
- Any fenced code block in a testable language without `test="..."` is flagged in coverage reports.
- Pages can opt out entirely with `testable_docs: false` in frontmatter.
- Non-executable languages are silently skipped — no warnings, no extraction.

**Language classification:**

| Category | Languages |
|----------|-----------|
| Testable (flagged if untested) | `bash`, `sh`, `shell`, `zsh` |
| Non-testable (silently skipped) | `text`, `plaintext`, `console`, `yaml`, `toml`, `json`, `xml`, `csv`, `markdown`, `go`, `python`, and any language without a helper |

**Ordering:** Blocks within a page are extracted in document order with a numeric prefix (`01-install-complyctl.bash`, `02-run-scan.bash`) to preserve sequencing for tests that build on prior state.

**Page slug derivation:** The page slug is the relative path from `content-dir` to the Markdown file, with the filename stripped and path separators replaced by hyphens. For `content/docs/getting-started/_index.md` with `--content-dir content/docs`, the slug is `getting-started`. For `content/docs/guides/advanced/_index.md`, the slug is `guides-advanced`. Top-level `_index.md` uses `root` as the slug.

## Go Extraction Tool

### Location

`cmd/doctest/` — follows the existing pattern established by `cmd/sync-content/`.

### CLI Interface

```
doctest extract --content-dir <path> --output-dir <path>
doctest coverage --content-dir <path>
```

**`extract` subcommand:**
- Walks `content-dir` recursively for `.md` files.
- Parses each file with goldmark, finds `FencedCodeBlock` nodes.
- For blocks with `{test="..."}` in the info string: writes content to `output-dir/<page-slug>/<NN>-<test-name>.<lang>`.
- Generates `manifest.json` per page mapping test names to source `file:line`.
- Respects `testable_docs: false` frontmatter.
- Exit 0 on success. Exit 1 on errors (duplicate test names, malformed attributes).

**`coverage` subcommand:**
- Same file walk and parse as `extract`.
- Reports untested executable-language blocks: one line per block with file path, line number, and language.
- Exit 1 when untested executable-language blocks exist. Exit 0 when all blocks are covered.

### Implementation Details

- Estimated ~200 lines of Go (main + extractor + coverage).
- Uses `github.com/yuin/goldmark` as a direct dependency for AST parsing.
- Attribute parsing follows Hugo's approach: find `{...}` in `FencedCodeBlock.Info.Segment.Value(source)`, pass to `parser.ParseAttributes()`.
- Language extraction: split info string on `{`, take first part, trim whitespace. Handles both `bash {test="..."}` (space) and `bash{test="..."}` (no space) forms.
- Frontmatter parsing: scan for `---` delimiters, unmarshal YAML with `go-yaml` (already a direct dependency) to check `testable_docs` key.

### Output Structure

```
<output-dir>/
└── getting-started/
    ├── 01-install-complyctl.bash
    ├── 02-run-scan.bash
    └── manifest.json
```

`manifest.json` format:
```json
{
  "page": "content/docs/getting-started/_index.md",
  "snippets": [
    {
      "test": "install-complyctl",
      "file": "01-install-complyctl.bash",
      "source_line": 42,
      "language": "bash"
    }
  ]
}
```

### Tests

Unit tests following the existing `cmd/sync-content/` pattern — table-driven tests with temp dirs. Coverage:

| Test | What it verifies |
|------|-----------------|
| `TestExtractBasic` | Single annotated block extracted with correct filename and content |
| `TestExtractOrdering` | Multiple blocks get sequential numeric prefixes in document order |
| `TestExtractDuplicateError` | Duplicate `test` values within a page produce an error |
| `TestExtractFrontmatterOptOut` | `testable_docs: false` skips the entire page |
| `TestExtractNoInfoString` | Blocks without info strings are silently skipped |
| `TestExtractNonTestableLanguage` | Non-testable languages are skipped in coverage, extracted if annotated |
| `TestExtractManifest` | `manifest.json` contains correct mappings |
| `TestCoverageReport` | Untested executable blocks are listed with file, line, language |
| `TestCoverageOptOut` | Pages with `testable_docs: false` produce no coverage entries |
| `TestParseAttributes` | Info-string attribute parsing handles various formats |
| `TestLanguageExtraction` | Language correctly parsed from info strings with/without space before `{` |

## Bats Test Harness

### Installation

Bats-core, bats-support, and bats-assert installed as npm devDependencies:

```json
"bats": "^1.13.0",
"bats-support": "^0.3.0",
"bats-assert": "^2.2.4"
```

This leverages the project's existing npm/Node.js toolchain — `npm install` is already a prerequisite for Hugo/Thulite development.

### Directory Layout

```
tests/docs/
├── setup_suite.bash        # shared setup: set SNIPPETS_DIR, load libraries
├── helpers/
│   └── bash.bash           # run_snippet() function
└── getting-started.bats    # hand-written tests (initially a skeleton)
```

### `setup_suite.bash`

```bash
export SNIPPETS_DIR="${SNIPPETS_DIR:-/tmp/doctest-snippets}"
```

Suite-level setup sets the snippets directory. Library loading happens per-file in `setup()` — this is the standard Bats pattern where each `.bats` file declares its own dependencies.

### `helpers/bash.bash`

```bash
run_snippet() {
    local snippet="$SNIPPETS_DIR/$1"
    [[ -f "$snippet" ]] || { echo "Snippet not found: $snippet" >&2; return 1; }
    run bash -- "$snippet"
}
```

### Test File Pattern

```bash
# tests/docs/getting-started.bats

setup() {
    load 'helpers/bash'
    load '../../node_modules/bats-support/load'
    load '../../node_modules/bats-assert/load'
}

@test "install-complyctl" {
    run_snippet "getting-started/01-install-complyctl.bash"
    assert_success
}

@test "run-scan" {
    run_snippet "getting-started/02-run-scan.bash"
    assert_success
    assert_output --partial "Compliance scan complete"
}
```

**Key design choices:**
- Tests are hand-written to allow custom assertions per snippet.
- Test names correspond to `test="..."` attribute values for coverage cross-referencing.
- `setup()` per-file for flexibility — individual test files can override shared setup.
- Tests receive extracted snippets, not raw Markdown. The extract step must run first.

## Makefile Integration

New section after existing "Content sync" section:

```makefile
# ---------------------------------------------------------------------------
# Documentation tests — extract, validate, and test code blocks
# ---------------------------------------------------------------------------

.PHONY: test-docs-extract test-docs test-docs-coverage

test-docs-extract: ## Extract testable code blocks from documentation
	@go run ./cmd/doctest extract --content-dir content/docs --output-dir /tmp/doctest-snippets

test-docs: test-docs-extract ## Run documentation tests (Bats)
	@node_modules/.bin/bats --formatter pretty tests/docs/

test-docs-coverage: ## Report untested code blocks in documentation
	@go run ./cmd/doctest coverage --content-dir content/docs
```

**`check` meta-target update:**

```makefile
check: vet fmt-check test-race test-docs-coverage
```

Coverage exits non-zero when untested blocks exist. The `-` prefix in Make ignores the exit code so `check` does not fail the build (Phase 1). Remove the prefix to enforce (Phase 2).

## CI Integration

**No new workflow file.** Integrates into existing pipelines.

**`ci.yml`** — add step after Hugo build:
```yaml
- name: Run documentation tests (informational)
  run: make test-docs || true
```

**`deploy-gh-pages.yml`** — add step before deploy:
```yaml
- name: Run documentation tests (informational)
  run: make test-docs || true
```

`make test-docs` depends on `test-docs-extract`, so ordering is automatic. The `|| true` makes doc test failures non-blocking in CI (Phase 1). Remove to enforce (Phase 2).

## Coverage Enforcement Model

### Phase 1 (This Implementation)

- `doctest coverage` exits non-zero when untested executable blocks exist.
- `make test-docs-coverage` and `make test-docs` use `-` prefix (Make) or `|| true` (CI) to run without failing the build.
- Included in `check` and `test` meta-targets — developers see warnings during normal workflow.
- Coverage gaps are visible but non-blocking.

### Phase 2 (Future)

- Remove `-` prefix from Makefile `test-docs` / `test-docs-coverage` calls.
- Remove `|| true` from CI workflow steps.
- Coverage failures then block builds and PRs.

### Opt-Out Mechanism

- **Page-level:** `testable_docs: false` in frontmatter. For pages that are purely conceptual or where commands aren't meant to be runnable.
- **Block-level:** No opt-out attribute. If a block is in an executable language, it should either have `test="..."` or use a non-executable language (`console`, `text`) if it's illustrative rather than runnable. Keeps the rules simple.

## Documentation Updates

### CONTRIBUTING.md

New section "Testing Documentation" after the existing testing section:
- `{test="..."}` attribute convention with examples
- How to add a testable code block
- How to write the corresponding Bats test
- `testable_docs: false` frontmatter opt-out
- Three `make` targets and when to use them
- Note that coverage warnings are non-blocking

### README.md

- Add `cmd/doctest/` and `tests/docs/` to the project structure listing
- One-liner in development section: `make test-docs` runs documentation snippet tests

### AGENTS.md

- When editing documentation with shell commands, always add `{test="..."}` attributes
- When fixing a docs issue, write a corresponding Bats test before fixing the snippet (TDD for docs)
- Run `make test-docs-coverage` to check for untested blocks

## Success Criteria

| ID | Criterion |
|----|-----------|
| SC-001 | `go test ./cmd/doctest/...` passes all unit tests |
| SC-002 | `go test -race ./cmd/doctest/...` passes with zero data race warnings |
| SC-003 | `make test-docs-extract` produces snippet files from annotated Markdown blocks |
| SC-004 | `make test-docs` runs Bats tests against extracted snippets |
| SC-005 | `make test-docs-coverage` reports untested executable blocks (exits non-zero when gaps exist, non-blocking in CI/check via `-` prefix) |
| SC-006 | `make check` includes coverage reporting |
| SC-007 | CI pipelines (`ci.yml`, `deploy-gh-pages.yml`) run `make test-docs` |
| SC-008 | CONTRIBUTING.md documents the testable-docs workflow |
| SC-009 | Annotated code blocks in Markdown render normally in Hugo (no visual difference) |
