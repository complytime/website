# SPDX-License-Identifier: Apache-2.0

# ---------------------------------------------------------------------------
# Makefile — ComplyTime website developer workflow
#
# Quick reference:
#   make help            — list all targets
#   make sync-dry        — dry-run content sync (reads GitHub, writes nothing)
#   make sync            — apply content sync to disk
#   make dev             — start Hugo dev server (after syncing content)
#   make check           — vet + fmt-check + race tests
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

# Common flags passed to every sync invocation
SYNC_FLAGS := --org $(ORG) --config $(CONFIG) --output $(OUTPUT) --workers $(WORKERS) --timeout $(TIMEOUT)
ifdef REPO
SYNC_FLAGS += --repo $(REPO)
endif

.DEFAULT_GOAL := help

# ---------------------------------------------------------------------------
# Prerequisite checks — verify required tools before running targets
# ---------------------------------------------------------------------------

REQUIRED_GO_VERSION   := 1.25
REQUIRED_NODE_VERSION := 22
REQUIRED_HUGO_VERSION := 0.164.0

.PHONY: _check-go _check-npm _check-hugo _check-node-version _check-go-version _check-hugo-version

_check-go:
	@command -v go >/dev/null 2>&1 || { \
		echo "Error: 'go' is not installed (need >= $(REQUIRED_GO_VERSION))."; \
		echo "  See https://go.dev/dl/"; \
		exit 1; \
	}

_check-go-version: _check-go
	@GO_VER=$$(go version | sed -E 's/.*go([0-9]+\.[0-9]+).*/\1/'); \
	GO_MAJOR=$$(echo "$$GO_VER" | cut -d. -f1); \
	GO_MINOR=$$(echo "$$GO_VER" | cut -d. -f2); \
	REQ_MAJOR=$$(echo "$(REQUIRED_GO_VERSION)" | cut -d. -f1); \
	REQ_MINOR=$$(echo "$(REQUIRED_GO_VERSION)" | cut -d. -f2); \
	if [ "$$GO_MAJOR" -lt "$$REQ_MAJOR" ] || \
	   { [ "$$GO_MAJOR" -eq "$$REQ_MAJOR" ] && [ "$$GO_MINOR" -lt "$$REQ_MINOR" ]; }; then \
		echo "Error: Go $$GO_VER found, but >= $(REQUIRED_GO_VERSION) is required."; \
		exit 1; \
	fi

_check-npm:
	@command -v npm >/dev/null 2>&1 || { \
		echo "Error: 'npm' is not installed. Install Node.js >= $(REQUIRED_NODE_VERSION) from https://nodejs.org/"; \
		exit 1; \
	}

_check-node-version: _check-npm
	@NODE_MAJOR=$$(node --version | sed -E 's/^v([0-9]+)\..*/\1/'); \
	if [ "$$NODE_MAJOR" -lt "$(REQUIRED_NODE_VERSION)" ]; then \
		echo "Error: Node.js v$$NODE_MAJOR found, but >= v$(REQUIRED_NODE_VERSION) is required."; \
		exit 1; \
	fi

_check-hugo:
	@command -v hugo >/dev/null 2>&1 || { \
		echo "Error: 'hugo' is not installed (need >= $(REQUIRED_HUGO_VERSION), extended edition)."; \
		echo "  Install the extended edition:"; \
		echo "    brew install hugo                                                     # macOS (extended by default)"; \
		echo "    sudo snap install hugo --channel=extended                              # Ubuntu/Debian"; \
		echo "    CGO_ENABLED=1 go install -tags extended github.com/gohugoio/hugo@v$(REQUIRED_HUGO_VERSION)  # from source"; \
		echo "  Or download hugo_extended_* from https://github.com/gohugoio/hugo/releases"; \
		exit 1; \
	}

_check-hugo-version: _check-hugo
	@HUGO_RAW=$$(hugo version); \
	HUGO_VER=$$(echo "$$HUGO_RAW" | sed -E 's/.*v([0-9]+\.[0-9]+\.[0-9]+).*/\1/'); \
	HUGO_MAJOR=$$(echo "$$HUGO_VER" | cut -d. -f1); \
	HUGO_MINOR=$$(echo "$$HUGO_VER" | cut -d. -f2); \
	HUGO_PATCH=$$(echo "$$HUGO_VER" | cut -d. -f3); \
	REQ_MAJOR=$$(echo "$(REQUIRED_HUGO_VERSION)" | cut -d. -f1); \
	REQ_MINOR=$$(echo "$(REQUIRED_HUGO_VERSION)" | cut -d. -f2); \
	REQ_PATCH=$$(echo "$(REQUIRED_HUGO_VERSION)" | cut -d. -f3); \
	if [ "$$HUGO_MAJOR" -lt "$$REQ_MAJOR" ] || \
	   { [ "$$HUGO_MAJOR" -eq "$$REQ_MAJOR" ] && [ "$$HUGO_MINOR" -lt "$$REQ_MINOR" ]; } || \
	   { [ "$$HUGO_MAJOR" -eq "$$REQ_MAJOR" ] && [ "$$HUGO_MINOR" -eq "$$REQ_MINOR" ] && [ "$$HUGO_PATCH" -lt "$$REQ_PATCH" ]; }; then \
		echo "Error: Hugo $$HUGO_VER found, but >= $(REQUIRED_HUGO_VERSION) is required."; \
		echo "  See https://github.com/gohugoio/hugo/releases"; \
		exit 1; \
	fi; \
	if ! echo "$$HUGO_RAW" | grep -qi 'extended'; then \
		echo "Error: Hugo $$HUGO_VER is installed, but the extended edition is required for SCSS support."; \
		echo "  Your version: $$HUGO_RAW"; \
		echo "  Install the extended edition:"; \
		echo "    brew install hugo                                                     # macOS (extended by default)"; \
		echo "    sudo snap install hugo --channel=extended                              # Ubuntu/Debian"; \
		echo "    CGO_ENABLED=1 go install -tags extended github.com/gohugoio/hugo@v$(REQUIRED_HUGO_VERSION)  # from source"; \
		echo "  Or download hugo_extended_* from https://github.com/gohugoio/hugo/releases"; \
		exit 1; \
	fi

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
build: _check-go-version ## Compile the sync-content binary
	go build -o $(SYNC_BIN) ./cmd/sync-content

.PHONY: test
test: _check-go-version ## Run all Go unit tests
	go test $(SYNC_PKG)

.PHONY: test-race
test-race: _check-go-version ## Run Go tests with the race detector
	go test -race $(SYNC_PKG)

.PHONY: vet
vet: _check-go-version ## Run go vet
	go vet $(SYNC_PKG)

.PHONY: fmt
fmt: _check-go-version ## Format Go source files with gofmt
	gofmt -w cmd/sync-content/

.PHONY: fmt-check
fmt-check: _check-go-version ## Check Go formatting (non-destructive)
	@out=$$(gofmt -l cmd/sync-content/); \
	if [ -n "$$out" ]; then \
		echo "The following files need formatting:"; \
		echo "$$out"; \
		exit 1; \
	fi

.PHONY: check
check: vet fmt-check test-race ## Run vet + fmt-check + race tests (CI equivalent)

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
# Hugo / Node — site build and dev server
# ---------------------------------------------------------------------------

.PHONY: node-install
node-install: _check-node-version ## Install Node dependencies (npm install)
	npm install

.PHONY: dev
dev: _check-node-version _check-hugo-version ## Start the Hugo dev server  (runs: npm run dev)
	npm run dev

.PHONY: site-build
site-build: _check-node-version _check-hugo-version ## Build the Hugo site  (runs: hugo --minify --gc)
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
clean-build: _check-node-version _check-hugo-version ## Remove Hugo output + resource cache, then rebuild (CI match)
	rm -rf public/ resources/
	npm run build

.PHONY: clean-nuclear
clean-nuclear: _check-node-version _check-hugo-version ## Full wipe (Hugo output, Hugo cache, node_modules) + fresh npm ci + rebuild
	rm -rf public/ resources/ /tmp/hugo_cache/ node_modules/
	npm ci
	npm run build

.PHONY: reset-sync
reset-sync: _check-go-version _check-node-version _check-hugo-version ## Clear all generated sync content + full rebuild (use when sync logic or upstream changes)
	rm -f .sync-manifest.json data/projects.json
	rm -rf content/docs/projects/*/
	rm -rf public/ resources/
	go run ./cmd/sync-content $(SYNC_FLAGS) --lock $(LOCK) --write
	npm run build
