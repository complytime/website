# SPDX-License-Identifier: Apache-2.0

# Helper for running extracted bash snippets.
# Usage in a @test block:
#   run_snippet "getting-started/01-install-complyctl.bash"

# snippet_origin resolves an extracted snippet (e.g. "getting-started/01-x.bash")
# back to the documentation file and line it came from, using the per-page
# manifest.json the extractor writes alongside the snippets. Prints
# "<page>:<line>" on success. Falls back to the raw snippet path when no
# manifest exists (e.g. the harness's own throwaway self-tests) or when jq is
# unavailable.
snippet_origin() {
    local rel="$1"
    local slug="${rel%/*}"
    local base="${rel##*/}"
    local manifest="$SNIPPETS_DIR/$slug/manifest.json"

    if [[ "$slug" != "$rel" ]] && [[ -f "$manifest" ]] && command -v jq >/dev/null 2>&1; then
        local page line
        page="$(jq -r '.page' "$manifest" 2>/dev/null)"
        line="$(jq -r --arg f "$base" \
            '.snippets[] | select(.file == $f) | .source_line' \
            "$manifest" 2>/dev/null)"
        if [[ -n "$page" && "$page" != "null" && -n "$line" && "$line" != "null" ]]; then
            echo "$page:$line"
            return 0
        fi
    fi
    echo "$SNIPPETS_DIR/$rel"
}

run_snippet() {
    local snippet="$SNIPPETS_DIR/$1"
    [[ -f "$snippet" ]] || { echo "Snippet not found: $snippet" >&2; return 1; }
    run bash -- "$snippet"
    # Surface the originating documentation source (file:line) only when the
    # snippet failed, so a red test is easy to trace back to the doc it came
    # from. Emit on stderr (not fd 3): Bats hides stderr for passing tests and
    # for `run`-wrapped calls, but prints it as failure context for a genuinely
    # failing test, keeping normal suite output clean. Return the snippet's own
    # exit status so callers using `run run_snippet ...` capture it.
    # $status/$output are set by Bats' `run` above.
    # shellcheck disable=SC2154
    if [[ "$status" -ne 0 ]]; then
        echo "# source: $(snippet_origin "$1")" >&2
    fi
    # shellcheck disable=SC2154
    return "$status"
}
