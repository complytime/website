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
