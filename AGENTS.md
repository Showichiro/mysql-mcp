# Repository Guidelines

## Project Structure & Module Organization

This repository is a small Go MCP server for read-only MySQL access.

- `cmd/mysql-mcp/main.go` contains the CLI entry point.
- `internal/mysqlmcp/` contains MCP server setup, tool handlers, query execution, and response shaping.
- `internal/config/` loads environment and `.env` configuration.
- `internal/sqlguard/` validates and rewrites allowed SQL; keep guard behavior covered by tests.
- `.env.example` documents local configuration. Do not commit real `.env` secrets.
- `.github/workflows/release.yml` and `.goreleaser.yaml` define tagged binary releases.

## Build, Test, and Development Commands

- `go test ./...` runs all package tests.
- `go test ./internal/sqlguard -run TestName` runs a focused guard test.
- `go build -o bin/mysql-mcp ./cmd/mysql-mcp` builds the local binary.
- `./bin/mysql-mcp` starts the stdio MCP server using environment variables or a local `.env`.
- `go mod tidy` updates module metadata after dependency changes.

Use a read-only MySQL user when running locally. The SQL guard is defense in depth, not the primary security boundary.

## Coding Style & Naming Conventions

Use standard Go formatting: run `gofmt` on changed Go files before committing. Keep package names short, lowercase, and aligned with directory names. Prefer clear, direct function names such as `Load`, `Validate`, or `RegisterTools` over abbreviations.

Keep exported identifiers documented when they become part of package-facing behavior. Avoid adding broad abstractions in this repository; small package-level helpers are usually enough.

## Testing Guidelines

Tests use Go's standard `testing` package. Place tests next to the implementation with `_test.go` suffixes, as in `internal/sqlguard/sqlguard_test.go`.

For SQL guard changes, include table-driven cases for allowed statements, rejected statements, and limit enforcement. Prefer deterministic unit tests over tests that require a live MySQL server unless the behavior truly depends on database integration.

## Commit & Pull Request Guidelines

Recent commits use short imperative messages, for example `Load local dotenv config` and `Keep MySQL metadata contexts alive`. Follow that style: one concise subject line that states the behavior change.

Pull requests should include:

- A short description of the change and why it is needed.
- Test results, usually `go test ./...`.
- Any configuration or security implications, especially changes to SQL validation, schema filtering, timeouts, or row limits.
- Linked issues when applicable.

## Security & Configuration Tips

Never commit `.env` or credentials. Keep `.env.example` in sync when adding `MYSQL_MCP_*` settings. When touching query execution, preserve read-only behavior, bounded result sizes, timeout handling, and cell truncation.
