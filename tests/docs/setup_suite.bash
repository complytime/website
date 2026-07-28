# SPDX-License-Identifier: Apache-2.0

# Suite-level setup for documentation tests.
# Sets the default snippets directory. Individual .bats files load
# their own libraries in setup().

setup_suite() {
    export SNIPPETS_DIR="${SNIPPETS_DIR:-/tmp/doctest-snippets}"
}
