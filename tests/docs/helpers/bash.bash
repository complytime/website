# SPDX-License-Identifier: Apache-2.0

# Helper for running extracted bash snippets.
# Usage in a @test block:
#   run_snippet "getting-started/01-install-complyctl.bash"

run_snippet() {
    local snippet="$SNIPPETS_DIR/$1"
    [[ -f "$snippet" ]] || { echo "Snippet not found: $snippet" >&2; return 1; }
    run bash -- "$snippet"
}
