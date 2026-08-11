# SPDX-License-Identifier: Apache-2.0

# End-to-end smoke test for the documentation test harness itself.
#
# This validates the run_snippet helper and the SNIPPETS_DIR wiring without
# depending on any doc-page annotations (those arrive in Phase 2). It creates
# a throwaway snippet, runs it, and asserts the harness reports success and
# failure correctly.

setup() {
    load 'helpers/bash'
    load '../../node_modules/bats-support/load'
    load '../../node_modules/bats-assert/load'

    HARNESS_TMP="$(mktemp -d)"
    export SNIPPETS_DIR="$HARNESS_TMP"
}

teardown() {
    rm -rf "$HARNESS_TMP"
}

@test "run_snippet succeeds for a passing snippet" {
    mkdir -p "$SNIPPETS_DIR/harness"
    printf 'echo hello\n' > "$SNIPPETS_DIR/harness/01-pass.bash"

    run_snippet "harness/01-pass.bash"
    assert_success
    assert_output "hello"
}

@test "run_snippet fails for a failing snippet" {
    mkdir -p "$SNIPPETS_DIR/harness"
    printf 'exit 3\n' > "$SNIPPETS_DIR/harness/02-fail.bash"

    # Wrap in `run` so the trace-on-failure diagnostic (emitted on fd 3 for a
    # failing snippet) is captured rather than printed, keeping suite output
    # clean; run_snippet returns the snippet's own exit status.
    run run_snippet "harness/02-fail.bash"
    assert_failure 3
}

@test "run_snippet reports missing snippet" {
    run run_snippet "harness/does-not-exist.bash"
    assert_failure
    assert_output --partial "Snippet not found"
}

@test "snippet_origin resolves source file:line from the manifest" {
    mkdir -p "$SNIPPETS_DIR/getting-started"
    printf 'echo hi\n' > "$SNIPPETS_DIR/getting-started/01-install.bash"
    cat > "$SNIPPETS_DIR/getting-started/manifest.json" <<'JSON'
{
  "page": "getting-started/_index.md",
  "snippets": [
    { "test": "install", "file": "01-install.bash", "source_line": 49, "language": "bash" }
  ]
}
JSON

    run snippet_origin "getting-started/01-install.bash"
    assert_success
    assert_output "getting-started/_index.md:49"
}

@test "snippet_origin falls back to the snippet path without a manifest" {
    run snippet_origin "harness/01-pass.bash"
    assert_success
    assert_output "$SNIPPETS_DIR/harness/01-pass.bash"
}
