# Code Quality Guidelines

This project uses `golangci-lint` to enforce code quality, security, and consistent formatting.

## Pull Request Requirements

All pull requests targeting the `master` branch must pass the `quality` check workflow. This workflow automatically runs:
1. `golangci-lint` (which includes `gofumpt` for strict formatting, `gosec` for security scanning, and other essential linters like `errcheck` and `govet`).
2. Unit tests using `go test -mod=vendor -race -count=1 ./...`

If either step fails, the pull request cannot be merged.

## Running Linters Locally

You should format and lint your code before submitting a pull request to avoid CI failures:

1. Install `golangci-lint` following the [official instructions](https://golangci-lint.run/usage/install/).
2. Run the linter in the root of the repository:
   ```bash
   golangci-lint run
   ```
   Or specifically with the 5-minute timeout as defined in CI:
   ```bash
   golangci-lint run --timeout 5m
   ```

If you see formatting errors from `gofumpt`, you can either configure your editor to use `gofumpt` as your formatting tool or run `gofumpt -l -w .` locally (after installing it).

All configurations for the linters are defined in `.golangci.yml`.
