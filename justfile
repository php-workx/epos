# epos project quality gate
# Single source of truth for "my code is clean" — hooks and CI delegate here.

# Developer tools are pinned in tools.mod and invoked through go tool.
go_tool := "go tool -modfile=tools.mod"

version := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
commit := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
build_date := `date -u +"%Y-%m-%dT%H:%M:%SZ"`
ldflags := "-X main.version=" + version + " -X main.gitCommit=" + commit + " -X main.buildDate=" + build_date

default:
    @just --list

# --- Quality gates ---

# Pre-commit: fast local checks + fresh non-race tests
pre-commit: format-check vet lint-check build-check mod-tidy-check actionlint betterleaks test-fast

# Pre-push: pre-commit + race tests + vulnerability scan + semgrep
pre-push: pre-commit test-race vuln semgrep

# Full quality gate: same as pre-push
check: pre-push

# Full dev suite: quality gate + sonar
dev: check
    @echo "All checks passed!"

# --- Static analysis ---

# Cache pinned Go tool dependencies before checks that parse tool output.
tools-ready:
    go mod download -modfile=tools.mod

# Check formatting via golangci-lint gofumpt (detect-only, no auto-fix)
format-check: tools-ready
    @test -z "$({{go_tool}} golangci-lint fmt --diff 2>&1)" || (echo "gofumpt: unformatted files" && {{go_tool}} golangci-lint fmt --diff 2>&1 && exit 1)

# Go vet
vet:
    go vet ./...

# Lint with golangci-lint
lint:
    {{go_tool}} golangci-lint run --fix

# Verify lint with golangci-lint
lint-check: tools-ready
    {{go_tool}} golangci-lint run

# Lint GitHub Actions workflows
actionlint:
    @if [ -d .github/workflows ]; then \
        {{go_tool}} actionlint .github/workflows/*.yml; \
    fi

# --- Security ---

# Scan for leaked secrets
betterleaks:
    @if command -v betterleaks >/dev/null 2>&1; then \
        betterleaks git --no-banner; \
    else \
        echo "warning: betterleaks not installed, skipping secret scan"; \
    fi

# Semantic code scan (optional, skip if not installed)
semgrep:
    @if command -v semgrep >/dev/null 2>&1; then \
        semgrep --config auto .; \
    else \
        echo "warning: semgrep not installed, skipping semantic scan"; \
    fi

# Scan for known vulnerabilities in dependencies
vuln:
    {{go_tool}} govulncheck ./...

# --- Testing ---

# Verify the project compiles (fast, no binary output)
build-check:
    go build ./...

# Verify go.mod/go.sum and tools.mod/tools.sum are tidy (detect-only)
mod-tidy-check:
    @cp go.mod go.mod.bak
    @if [ -f go.sum ]; then cp go.sum go.sum.bak; fi
    @go mod tidy
    @DIRTY=0; \
        diff -q go.mod go.mod.bak >/dev/null 2>&1 || DIRTY=1; \
        if [ -f go.sum.bak ]; then diff -q go.sum go.sum.bak >/dev/null 2>&1 || DIRTY=1; \
        elif [ -f go.sum ]; then DIRTY=1; fi; \
        mv go.mod.bak go.mod; \
        if [ -f go.sum.bak ]; then mv go.sum.bak go.sum; elif [ -f go.sum ]; then rm go.sum; fi; \
        if [ "$DIRTY" = "1" ]; then echo "go.mod/go.sum not tidy — run 'go mod tidy'" && exit 1; fi
    @cp tools.mod tools.mod.bak
    @if [ -f tools.sum ]; then cp tools.sum tools.sum.bak; fi
    @go mod tidy -modfile=tools.mod
    @DIRTY=0; \
        diff -q tools.mod tools.mod.bak >/dev/null 2>&1 || DIRTY=1; \
        if [ -f tools.sum.bak ]; then diff -q tools.sum tools.sum.bak >/dev/null 2>&1 || DIRTY=1; \
        elif [ -f tools.sum ]; then DIRTY=1; fi; \
        mv tools.mod.bak tools.mod; \
        if [ -f tools.sum.bak ]; then mv tools.sum.bak tools.sum; elif [ -f tools.sum ]; then rm tools.sum; fi; \
        if [ "$DIRTY" = "1" ]; then echo "tools.mod/tools.sum not tidy — run 'go mod tidy -modfile=tools.mod'" && exit 1; fi

# Tidy go.mod/go.sum and tools.mod/tools.sum in-place.
mod-tidy:
    go mod tidy
    go mod tidy -modfile=tools.mod

# Run all tests without race detector (fresh)
test-fast:
    go test -count=1 ./...

# Run all tests with race detector (fresh)
test-race:
    go test -race -count=1 ./...

# Run tests with coverage report
cover:
    go test -race -coverprofile=coverage.out -covermode=atomic ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report: coverage.html"

# --- Build targets ---

# Build the epos binary with version info
build:
    mkdir -p bin
    go build -ldflags '{{ldflags}}' -o bin/epos ./cmd/epos

# Install epos to $GOPATH/bin (or $GOBIN)
install:
    go install -ldflags '{{ldflags}}' ./cmd/epos

# --- Setup ---

# Format all Go files in-place (use when format-check fails)
format:
    {{go_tool}} golangci-lint fmt

# Auto-fix formatting and lint issues, then verify
autofix: format
    {{go_tool}} golangci-lint run --fix ./...

# Set up git hooks and development environment
setup: install-dev install-hooks
    @echo "Development environment ready."

# Install local git hooks into .git/hooks.
install-hooks:
    bash scripts/install-hooks.sh

# Cache required development tools (pinned in tools.mod)
install-dev:
    @echo "Caching Go tool dependencies from tools.mod..."
    just tools-ready
    @echo "Done! Development tools are available through go tool -modfile=tools.mod."

# Remove build artifacts
clean:
    #!/usr/bin/env bash
    set -euo pipefail
    rm -rf bin/ coverage.out coverage.html
    go clean
