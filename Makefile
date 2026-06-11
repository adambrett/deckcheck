# Make
export

SHELL := /bin/bash -o errexit -o nounset -o pipefail

MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

VERBOSE ?= false
ifeq (${VERBOSE}, false)
	# --silent drops the need to prepend `@` to suppress command output
	MAKEFLAGS += --silent
endif

# Variables
GOBASE ?= $(shell pwd)
GOCACHE := ${GOBASE}/.cache/go-build
GOLANGCI_LINT_CACHE := ${GOBASE}/.cache/golangci-lint
WEBSITE_PORT ?= 8000
UNAME_S := $(shell uname -s)

APP_NAME ?= DeckCheck
APP_ID   ?= dev.adbr.deckcheck
APP_CMD  ?= ./cmd/gui/deckcheck

COVERPROFILE ?= coverage.out
COVER_MIN ?= 80
# Packages whose code is purely glue / composition (main, the
# dependencies wiring) and pure test support (the mockery-generated
# mocks, the fynetest widget-tree helpers) are excluded from the
# coverage bar.
# Deferred assignment: GO is defined below.
COVER_PACKAGES = $(shell $(GO) list \
	./internal/... \
	| grep -v -e '/mocks' -e '/fynetest' -e '/ui/dependencies')

# Applications
GO ?= go
PYTHON ?= python3

# Helpers
.PHONY: run
run: ## Run the application
	$(MAKE) build
	./bin/$(APP_NAME)

.PHONY: run-website
run-website: ## Serve the marketing website locally
	$(PYTHON) -m http.server ${WEBSITE_PORT} --directory ./website

.PHONY: fmt
fmt: ## Format source files
	find . -type f -name '*.go' -not -path "./vendor/*" | \
		xargs -I {} $(GO) tool goimports-reviser -company-prefixes="github.com/adambrett/" -project-name="github.com/adambrett/deckcheck" {}
	# In some cases you need to run gofumpt twice to resolve all formatting issues as one simplification
	# can allow another one, but gofumpt is not smart enough to apply both at the same time.
	find . -type f -name '*.go' -not -path "./vendor/*" | xargs $(GO) tool gofumpt -w
	find . -type f -name '*.go' -not -path "./vendor/*" | xargs $(GO) tool gofumpt -w

.PHONY: fmt-check
fmt-check: ## Verify source files are formatted (non-mutating, suitable for CI)
	@diff_output=$$(find . -type f -name '*.go' -not -path "./vendor/*" | xargs $(GO) tool gofumpt -l); \
	if [ -n "$$diff_output" ]; then \
		echo "The following files are not gofumpt-clean:"; \
		echo "$$diff_output"; \
		echo "Run 'make fmt' to fix."; \
		exit 1; \
	fi

.PHONY: generate
generate: ## Regenerate mocks and run the formatter
	$(GO) tool mockery
	$(MAKE) fmt

# Dependencies
.PHONY: deps
deps: ## Update project dependencies
	$(GO) mod tidy

# Linting
.PHONY: lint
lint: ## Run the linter
	$(GO) tool golangci-lint run ./...

.PHONY: screenshots
screenshots: ## Regenerate the README screenshots (software-rendered, no display needed)
	$(GO) run ./cmd/tools/screenshots

# Testing
.PHONY: test
test: unit-test integration-test ## Run all tests

.PHONY: unit-test
unit-test: ## Run untagged tests (pure domain, no Fyne driver)
	$(GO) test -race ./internal/... -v

.PHONY: integration-test
integration-test: ## Run integration-tagged widget/UI tests (headless Fyne driver)
	$(GO) test -race ./internal/ui/... ./internal/fyneui/... -tags=integration -v

.PHONY: coverage
coverage: ## Run coverage and require every package plus the total to meet COVER_MIN
	coverage_output="$$(mktemp)"; \
	trap 'rm -f "$$coverage_output"' EXIT; \
	set +e; \
	$(GO) test -race -tags=integration -coverprofile=$(COVERPROFILE) $(COVER_PACKAGES) > "$$coverage_output" 2>&1; \
	test_status=$$?; \
	set -e; \
	cat "$$coverage_output"; \
	if [ "$$test_status" -ne 0 ]; then exit "$$test_status"; fi; \
	awk -v min="$(COVER_MIN)" '/coverage:/ { pct=$$(NF-2); gsub("%", "", pct); if (pct + 0 < min) { printf "%s coverage %s%% is below COVER_MIN=%s%%\n", $$2, pct, min; failed = 1 } } END { exit failed }' "$$coverage_output"
	$(GO) tool cover -func=$(COVERPROFILE) | awk -v min="$(COVER_MIN)" '{ print } /^total:/ { gsub("%", "", $$3); if ($$3 + 0 < min) { printf "coverage %s%% is below COVER_MIN=%s%%\n", $$3, min; exit 1 } }'

# Building
.PHONY: build
build: ## Build the application
	mkdir -p ./bin
	$(GO) build -o ./bin/$(APP_NAME) $(APP_CMD)

.PHONY: package
package: ## Package the application for the current OS
	$(GO) tool fyne package -name $(APP_NAME) -app-id $(APP_ID) -icon ${CURDIR}/internal/assets/icon.png -src $(APP_CMD)
ifeq (${UNAME_S}, Darwin)
	codesign --force --deep --sign - ./$(APP_NAME).app
endif

# Cleaning
.PHONY: clean
clean: ## Clean build artifacts
	rm -rf ./bin/$(APP_NAME)

.PHONY: clean-all
clean-all: clean ## Clean all build artifacts
	rm -rf ./bin ./.cache

# Make Helpers
.PHONY: help
help: ## Print this help message
	grep -E '^[/a-zA-Z_-]+:.*?## .*$$' ${MAKEFILE_LIST} | sort | awk 'BEGIN {FS = ":|##"}; {printf "%-20s\033[36m%-20s \033[0m %s\n", $$1, $$2, $$4}'

print-%: ## Print the value of a variable
	echo $* = $($*)
