# Google Antigravity CLI Provider Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a reversible Google Antigravity CLI quota provider based on Google's documented status-line JSON payload.

**Architecture:** Install an AI Quota bridge as the Antigravity CLI status-line command, parse its model/bucket quota map into dynamic normalized windows, cache only quota data, and pass the original payload through to any previous status-line command. Extend service and tray orchestration so users can enable, disable, refresh, view, and receive alerts for Antigravity quota buckets.

**Tech Stack:** Go 1.24, standard library JSON/process/filesystem packages, `fyne.io/systray`, Go `testing`.

---

### Task 1: Support named dynamic quota windows

**Files:**
- Modify: `internal/model/quota.go`
- Modify: `internal/model/quota_test.go`
- Modify: `internal/tray/app.go`
- Modify: `internal/tray/app_test.go`

1. Add failing model tests proving custom bucket kinds survive normalization and use an explicit display label.
2. Run `go test ./internal/model -run 'CustomWindow|Normalize' -count=1` and confirm the new display API is missing.
3. Add `Label` to `model.Window`, a `DisplayName` method that falls back to `WindowKind.DisplayName`, and preserve arbitrary kinds during normalization.
4. Add failing tray tests for formatting a custom named bucket and deriving sorted window rows from multiple buckets.
5. Run `go test ./internal/tray -run 'CustomWindow|WindowRows' -count=1` and confirm the row helper is missing.
6. Replace fixed session/weekly rendering helpers with sorted dynamic window-row helpers while preserving existing Codex and Claude labels.
7. Run both package test suites and commit.

### Task 2: Parse and cache the official Antigravity payload

**Files:**
- Create: `internal/provider/antigravity/statusline.go`
- Create: `internal/provider/antigravity/statusline_test.go`
- Create: `internal/provider/antigravity/provider.go`
- Modify: `internal/config/paths.go`
- Modify: `internal/model/quota.go`

1. Write failing table tests for a payload with multiple quota buckets, zero remaining quota, RFC3339 reset timestamps, fallback `reset_in_seconds`, malformed entries, and no quota data.
2. Run `go test ./internal/provider/antigravity -run ParseStatusLine -count=1` and confirm the package/API is absent.
3. Add `ProviderAntigravity`, Antigravity cache/config paths, input structs, stable bucket kinds, labels, fraction-to-used conversion, reset parsing, and empty-data errors.
4. Run the parser tests and confirm they pass.
5. Write a failing bridge test proving the cache contains only normalized provider status and that fallback output is emitted.
6. Implement `RunBridge`, cache provider `Fetch`, and compact fallback formatting.
7. Run `go test ./internal/provider/antigravity -count=1` and commit.

### Task 3: Install and restore Antigravity status-line configuration

**Files:**
- Create: `internal/provider/antigravity/installer.go`
- Modify: `internal/provider/antigravity/statusline.go`
- Modify: `internal/provider/antigravity/statusline_test.go`

1. Write failing tests proving connect creates settings and a helper, an existing command receives the original payload, disconnect restores the exact raw `statusLine`, and disconnect refuses settings changed after connection.
2. Run `go test ./internal/provider/antigravity -run 'Installer|PassesThrough|SettingsChanged' -count=1` and confirm the installer is missing.
3. Implement executable copying, atomic settings updates, timestamped full-file backup, raw status-line backup, bridge command detection, previous-command execution with timeout, and exact restoration.
4. Run the focused tests, then the entire Antigravity package suite, and commit.

### Task 4: Integrate provider refresh, cache, and errors

**Files:**
- Modify: `internal/appcore/service.go`
- Modify: `internal/appcore/service_test.go`

1. Write failing tests proving the service registers Antigravity, loads its cache, clears it, and reports a friendly no-data error without removing a previous snapshot.
2. Run `go test ./internal/appcore -run Antigravity -count=1` and confirm failures for the missing provider.
3. Register the cache provider, load/write/clear the Antigravity path, and map Antigravity errors to setup-friendly text.
4. Run `go test ./internal/appcore -count=1` and commit.

### Task 5: Add tray setup and dynamic quota rows

**Files:**
- Modify: `internal/tray/app.go`
- Modify: `internal/tray/app_test.go`
- Modify: `cmd/aiquota/main.go`

1. Write failing pure-helper tests for Antigravity provider ordering, connected/disabled/error menu state, visibility, and multiple bucket rows.
2. Run `go test ./internal/tray -run Antigravity -count=1` and confirm Antigravity is absent.
3. Add the Antigravity installer and connection cache to `App`, provider order and tooltip, enable/disable click handling, dynamic submenu rows, cache clearing, notification copy, and bridge dispatch in `main`.
4. Run tray and command-package tests, then `go test ./...`, and commit.

### Task 6: Document and verify the complete integration

**Files:**
- Modify: `README.md`

1. Update features, official data sources, requirements, setup instructions, local data, limitations, and project structure for Google Antigravity CLI.
2. Run `gofmt -w` on every changed Go file.
3. Run `git diff --check`, `go test ./...`, `go test -race ./...`, `go vet ./...`, and `go build ./cmd/aiquota`.
4. Audit the design requirements against code, tests, and documentation; fix any uncovered gap through a failing regression test first.
5. Commit the documentation and verification fixes.
