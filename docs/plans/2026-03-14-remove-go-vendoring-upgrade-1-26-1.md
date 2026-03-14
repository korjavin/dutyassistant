# Remove Go Vendoring and Upgrade to Go 1.26.1

## Overview
This plan removes Go vendoring from the project, upgrades the Go version to 1.26.1 across all configurations (go.mod, Docker, GitHub Actions), and updates the documentation and build scripts accordingly.

## Context
- Files involved:
  - `go.mod`, `go.sum`: Core Go module configuration
  - `Dockerfile`: Multi-stage build configuration
  - `.github/workflows/ci-cd.yml`: CI/CD pipeline
  - `.gitignore`, `.dockerignore`: Ignore patterns
  - Markdown documentation files
- Related patterns: The project currently uses `-mod=vendor` for reproducible builds in Docker and CI, which will be replaced by standard module resolution and caching.

## Development Approach
- **Testing approach**: Regular (Verify builds and run tests).
- Delete the `vendor/` directory early to ensure no hidden dependencies on it.
- Update Go version consistently everywhere to avoid toolchain mismatches.
- **CRITICAL: all tests must pass after the upgrade.**

## Implementation Steps

### Task 1: Upgrade Go Version and Remove Vendoring

**Files:**
- Modify: `go.mod`
- Delete: `vendor/` directory

- [ ] Update `go` version to `1.26.1` in `go.mod`
- [ ] Update `toolchain` version to `go1.26.1` in `go.mod`
- [ ] Delete the `vendor/` directory: `rm -rf vendor/`
- [ ] Run `go mod tidy` to refresh `go.sum` and verify dependencies
- [ ] Run `go build ./...` to ensure the project still compiles without vendoring
- [ ] run project test suite - must pass before task 2

### Task 2: Update Docker Configuration

**Files:**
- Modify: `Dockerfile`, `.dockerignore`

- [ ] Update builder image to `golang:1.26.1-alpine` in `Dockerfile`
- [ ] Add `RUN go mod download` after copying `go.mod` and `go.sum` for better layer caching in `Dockerfile`
- [ ] Remove `-mod=vendor` flag from all `go build` commands in `Dockerfile`
- [ ] Add `vendor/` to `.dockerignore` to ensure it's not sent to the Docker daemon
- [ ] run project test suite - must pass before task 3

### Task 3: Update GitHub Actions

**Files:**
- Modify: `.github/workflows/ci-cd.yml`

- [ ] Update `go-version` to `1.26.1` in all jobs (including commented out `test` job)
- [ ] Remove any `-mod=vendor` flags from `go` commands in the workflow (check `go test`)
- [ ] Ensure `actions/setup-go` cache is working correctly without the vendor folder
- [ ] run project test suite - must pass before task 4

### Task 4: Update Documentation and Ignore Files

**Files:**
- Modify: `.gitignore` and markdown documentation

- [ ] Update `.gitignore`: add `vendor/` and remove any instructions to commit it
- [ ] Update Go version requirements and build instructions in markdown documentation
- [ ] Update technology stack references in markdown documentation to reflect Go 1.26.1
- [ ] run project test suite - must pass before task 5

### Task 5: Verify Build and Tests

- [ ] Run `go mod tidy` one last time
- [ ] Run `go test ./...` - all tests must pass
- [ ] Run a local Docker build to verify the `Dockerfile` changes: `docker build -t roster-bot-test .`
- [ ] Verify the binary size and ensure no regressions in build performance

### Task 6: Verify acceptance criteria

- [ ] manual test: verify app starts after build
- [ ] run full test suite (go test ./...)
- [ ] run linter (if any, e.g., golangci-lint)
- [ ] verify test coverage meets 80%+

### Task 7: Update documentation

- [ ] update README.md if user-facing changes (Go version requirements)
- [ ] update CLAUDE.md if internal patterns changed
- [ ] move this plan to `docs/plans/completed/`
