# Repository Guidelines

## Project Structure & Module Organization

AI Quota is a Go 1.24 macOS menu-bar application. Executables live in `cmd/`: `cmd/aiquota` is the application and Claude bridge entry point, while `cmd/icon-gen` generates bundle icons. Application code is grouped by responsibility under `internal/`, including provider connectors, tray UI, alerts, persistence, notifications, and normalized quota models. Keep provider-specific logic in `internal/provider/<name>/` and depend on the interface in `internal/provider/provider.go`. Tests sit beside their source as `*_test.go`. Packaging metadata is under `packaging/macos/`; build and release automation lives in `scripts/`. Generated artifacts in `build/` and `dist/` are ignored.

## Build, Test, and Development Commands

- `go mod download` downloads module dependencies.
- `make test` or `go test ./...` runs the complete unit test suite.
- `make run` starts the menu-bar app with CGO enabled.
- `make build` runs tests, generates the icon, builds and signs `dist/AIQuota.app`, and creates a ZIP. It requires macOS tools such as `iconutil` and `codesign`.
- `VERSION=0.2.0 BUILD_NUMBER=2 make build` produces a versioned local build.
- `make release VERSION=0.2.0` validates a clean, synchronized branch and publishes a GitHub release through authenticated `gh`.

## Coding Style & Naming Conventions

Use standard Go formatting: tabs, `gofmt`, short package names, exported identifiers in `PascalCase`, and unexported identifiers in `camelCase`. Keep packages cohesive and interfaces small. Wrap errors with useful operation context, and pass `context.Context` through provider calls. Before submitting, run `gofmt -w` on changed Go files and `go vet ./...`.

## Testing Guidelines

Use Go's `testing` package; no external test framework or coverage threshold is configured. Name tests `TestBehavior` and prefer table-driven cases for multiple inputs. Use `t.TempDir()` for filesystem tests and avoid real credentials, provider accounts, notifications, or user configuration. Add regression tests with bug fixes, then run `go test ./...`.

## Commit & Pull Request Guidelines

Recent history uses concise, imperative, sentence-case subjects such as `Improve provider tracking menu and reset countdowns`. Keep each commit focused; Conventional Commit prefixes are not required. Pull requests should explain user-visible behavior, list verification commands, link relevant issues, and include screenshots for tray-menu changes. Call out macOS-only testing, configuration migration, signing, or release implications.

## Security & Configuration

Never commit API keys, OAuth tokens, signing identities, or files from `~/Library/Application Support/AIQuota/`. Preserve the backup-and-restore safeguards around Claude Code's `~/.claude/settings.json`. Use `AIQUOTA_CODEX_PATH` only as a local development override.
