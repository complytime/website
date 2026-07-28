# SPDX-License-Identifier: Apache-2.0

# Documentation tests for content/docs/getting-started/_index.md
#
# Tests will be added here as code blocks in the getting started guide
# are annotated with {test="..."} attributes. Each @test name must
# match a test attribute value in the Markdown source.

setup() {
    load 'helpers/bash'
    load '../../node_modules/bats-support/load'
    load '../../node_modules/bats-assert/load'
}

# Placeholder: add @test blocks as snippets are annotated.
# Example:
#
# @test "install-complyctl" {
#     run_snippet "getting-started/01-install-complyctl.bash"
#     assert_success
# }
