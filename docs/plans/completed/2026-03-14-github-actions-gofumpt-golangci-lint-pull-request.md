---
# Add GitHub Actions PR Quality Checks

## Overview
Create a dedicated PR check workflow and golangci-lint config to enforce code quality on every pull request. Checks include golangci-lint (which subsumes gofumpt formatting and gosec security scanning), plus test execution. Target: all checks complete within 10 minutes.

## Context
- Files involved: `.github/workflows/pr-checks.yml` (new), `.golangci.yml` (new)
- Existing: `.github/workflows/ci-cd.yml` runs only on push to master (build + deploy, no linting)
- Go version: 1.26.1, vendor directory removed
- golangci-lint natively runs gofumpt (via `gofumpt` linter) and gosec (via `gosec` linter), so one tool covers all three requirements

## Development Approach
- No application code changes, only CI configuration and linter config
- Testing approach: validate by checking YAML syntax and linter config correctness
- CRITICAL: every task MUST include new/updated tests (N/A here - CI config only)

## Implementation Steps

### Task 1: Create golangci-lint configuration

**Files:**
- Create: `.golangci.yml`

- [ ] Create `.golangci.yml` with the following enabled linters:
  - `gofumpt` — strict formatting (superset of gofmt)
  - `gosec` — security scanning
  - `errcheck` — unchecked errors
  - `govet` — suspicious constructs
  - `staticcheck` — comprehensive static analysis
  - `unused` — unused code
  - `misspell` — typos in comments/strings
  - `nolintlint` — proper use of nolint directives
- [ ] Set `run.timeout: 5m` to ensure golangci-lint itself doesn't exceed budget
- [ ] Set `linters.enable-all: false` and list only the chosen linters explicitly
- [ ] Configure `linters-settings.gofumpt.extra-rules: true` for maximum strictness
- [ ] Configure `linters-settings.gosec` with standard ruleset (no exclusions)
- [ ] Add `issues.exclude-rules` to suppress known false positives; vendor is excluded by default via `run.skip-dirs`

### Task 2: Create PR check workflow

**Files:**
- Create: `.github/workflows/pr-checks.yml`

- [ ] Create workflow that triggers on `pull_request` targeting `master` (and `workflow_dispatch` for manual runs)
- [ ] Define a single job `quality` with these steps in order:
  1. `actions/checkout@v4`
  2. `actions/setup-go@v5` with `go-version: '1.26.1'` and `cache: true` (built-in Go module cache)
  3. `golangci/golangci-lint-action@v8` — uses its own integrated cache, runs against vendor, passes `--timeout 5m`
  4. Run `go test -race -count=1 ./...`
- [ ] Set `permissions: contents: read` (minimal permissions for a read-only check job)
- [ ] Do NOT add `needs:` between steps (sequential within one job is sufficient and simpler)
- [ ] Expected wall-clock time: golangci-lint ~2-3 min cached, tests ~1-2 min, total well under 10 min

### Task 3: Verify acceptance criteria

- [ ] Open a test PR and confirm the `quality` job appears as a required check
- [ ] Confirm golangci-lint correctly rejects a file with wrong formatting (gofumpt violation)
- [ ] Confirm gosec flags a known issue (e.g., `G304` file path from variable)
- [ ] Confirm tests run and failures block the PR
- [ ] Confirm total workflow duration is under 10 minutes on a cold cache run

### Task 4: Update documentation

- [ ] Update CLAUDE.md to note that PRs require `quality` check to pass
- [ ] Move this plan to `docs/plans/completed/`
