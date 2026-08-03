# SPDX-License-Identifier: Apache-2.0

# ---------------------------------------------------------------------------
# Makefile — ComplyTime website developer workflow
#
# Quick reference:
#   make help            — list all targets
#   make sync-dry        — dry-run content sync (reads GitHub, writes nothing)
#   make sync            — apply content sync to disk
#   make dev             — start Hugo dev server (after syncing content)
#   make check           — vet + fmt-check + race tests + doc coverage
# ---------------------------------------------------------------------------

# Overridable variables
ORG        ?= complytime
CONFIG     ?= sync-config.yaml
LOCK       ?= .content-lock.json
OUTPUT     ?= .
WORKERS    ?= 5
TIMEOUT    ?= 3m
REPO       ?=

SYNC_BIN   := cmd/sync-content/sync-content
SYNC_PKG   := ./cmd/sync-content/...
DOCTEST_DIR ?= /tmp/doctest-snippets

# Common flags passed to every sync invocation
SYNC_FLAGS := --org $(ORG) --config $(CONFIG) --output $(OUTPUT) --workers $(WORKERS) --timeout $(TIMEOUT)
ifdef REPO
SYNC_FLAGS += --repo $(REPO)
endif

.DEFAULT_GOAL := help

# ---------------------------------------------------------------------------
# Help
# ---------------------------------------------------------------------------

.PHONY: help
help: ## Show this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
	     /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ---------------------------------------------------------------------------
# Go — build, test, lint
# ---------------------------------------------------------------------------

.PHONY: build
build: ## Compile the sync-content binary
	go build -o $(SYNC_BIN) ./cmd/sync-content

.PHONY: test
test: ## Run all tests (Go unit + doc tests + doc coverage)
	go test $(SYNC_PKG) ./cmd/doctest/...
	-$(MAKE) test-docs
	-$(MAKE) test-docs-coverage

.PHONY: test-race
test-race: ## Run Go tests with the race detector
	go test -race $(SYNC_PKG) ./cmd/doctest/...

.PHONY: vet
vet: ## Run go vet
	go vet $(SYNC_PKG) ./cmd/doctest/...

.PHONY: fmt
fmt: ## Format Go source files with gofmt
	gofmt -w cmd/sync-content/ cmd/doctest/

.PHONY: fmt-check
fmt-check: ## Check Go formatting (non-destructive)
	@out=$$(gofmt -l cmd/sync-content/ cmd/doctest/); \
	if [ -n "$$out" ]; then \
		echo "The following files need formatting:"; \
		echo "$$out"; \
		exit 1; \
	fi

.PHONY: check
check: vet fmt-check test-race ## Run vet + fmt-check + race tests + doc coverage (CI equivalent)
	-$(MAKE) test-docs-coverage

# ---------------------------------------------------------------------------
# Content sync — uses GITHUB_TOKEN from the environment
# ---------------------------------------------------------------------------

.PHONY: sync-dry
sync-dry: build ## Dry-run content sync — reads GitHub, writes nothing to disk
	./$(SYNC_BIN) $(SYNC_FLAGS)

.PHONY: sync
sync: build ## Apply content sync to disk (--write)
	./$(SYNC_BIN) $(SYNC_FLAGS) --write

.PHONY: sync-locked
sync-locked: build ## Apply content sync at approved SHAs from .content-lock.json
	./$(SYNC_BIN) $(SYNC_FLAGS) --lock $(LOCK) --write

.PHONY: sync-update-lock
sync-update-lock: build ## Refresh .content-lock.json with current upstream SHAs (no content write)
	./$(SYNC_BIN) $(SYNC_FLAGS) --lock $(LOCK) --update-lock

.PHONY: sync-single-dry
sync-single-dry: ## Dry-run sync for one repo  (REPO=complytime/complyctl)
	@if [ -z "$(REPO)" ]; then echo "Usage: make sync-single-dry REPO=complytime/<name>"; exit 1; fi
	$(MAKE) sync-dry REPO=$(REPO)

.PHONY: sync-single
sync-single: ## Apply sync for one repo  (REPO=complytime/complyctl)
	@if [ -z "$(REPO)" ]; then echo "Usage: make sync-single REPO=complytime/<name>"; exit 1; fi
	$(MAKE) sync REPO=$(REPO)

# ---------------------------------------------------------------------------
# Documentation tests — extract, validate, and test code blocks
# ---------------------------------------------------------------------------

.PHONY: test-docs-extract
test-docs-extract: ## Extract testable code blocks from documentation
	@go run ./cmd/doctest extract --content-dir content/docs --output-dir $(DOCTEST_DIR)

.PHONY: test-docs
test-docs: test-docs-extract ## Run documentation tests (Bats)
	@SNIPPETS_DIR=$(DOCTEST_DIR) node_modules/.bin/bats --formatter pretty tests/docs/

.PHONY: test-docs-coverage
test-docs-coverage: ## Report untested code blocks in documentation
	@go run ./cmd/doctest coverage --content-dir content/docs

# ---------------------------------------------------------------------------
# Hugo / Node — site build and dev server
# ---------------------------------------------------------------------------

.PHONY: node-install
node-install: ## Install Node dependencies (npm install)
	npm install

.PHONY: dev
dev: ## Start the Hugo dev server  (runs: npm run dev)
	npm run dev

.PHONY: site-build
site-build: ## Build the Hugo site  (runs: hugo --minify --gc)
	npm run build

.PHONY: preview
preview: sync site-build ## Full preview: sync content then build the site

# ---------------------------------------------------------------------------
# Housekeeping
# ---------------------------------------------------------------------------

.PHONY: clean
clean: ## Remove the compiled sync-content binary
	rm -f $(SYNC_BIN)

.PHONY: clean-build
clean-build: ## Remove Hugo output + resource cache, then rebuild (CI match)
	rm -rf public/ resources/
	npm run build

.PHONY: clean-nuclear
clean-nuclear: ## Full wipe (Hugo output, Hugo cache, node_modules) + fresh npm ci + rebuild
	rm -rf public/ resources/ /tmp/hugo_cache/ node_modules/
	npm ci
	npm run build

.PHONY: reset-sync
reset-sync: ## Clear all generated sync content + full rebuild (use when sync logic or upstream changes)
	rm -f .sync-manifest.json data/projects.json
	rm -rf content/docs/projects/*/
	rm -rf public/ resources/
	go run ./cmd/sync-content $(SYNC_FLAGS) --lock $(LOCK) --write
	npm run build
