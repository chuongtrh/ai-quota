# Background Efficiency Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reduce background wake-ups and refresh latency without reducing the 60-second quota freshness or changing user-visible behavior.

**Architecture:** Throttle configuration reads behind a small time-based cache, run independent provider fetches concurrently, and retain a successfully resolved Codex executable path until launch failure. Preserve existing synchronization, timeout, and error handling boundaries.

**Tech Stack:** Go 1.24, standard library concurrency and testing, `fyne.io/systray`.

---

### Task 1: Cache Claude connection checks

**Files:**
- Modify: `internal/tray/app.go`
- Test: `internal/tray/app_test.go`

1. Add failing tests showing repeated reads within 60 seconds reuse state and forced reads bypass the cache.
2. Run `go test ./internal/tray -run Connection -count=1` and confirm failure because the cache API is absent.
3. Add the minimal cache, wire it into `updateMenu` and `toggleClaudeTracking`, and change the UI ticker from two to 30 seconds.
4. Run the focused tests and `go test ./internal/tray`.

### Task 2: Fetch providers concurrently

**Files:**
- Modify: `internal/appcore/service.go`
- Create: `internal/appcore/service_test.go`

1. Add a failing test with two blocking fake providers and assert both enter `Fetch` before either is released.
2. Run `go test ./internal/appcore -run Concurrent -count=1` and confirm the sequential implementation times out.
3. Fetch into a buffered result channel using one goroutine per provider, then apply results through existing setters.
4. Run the focused test and package tests.

### Task 3: Cache Codex command discovery

**Files:**
- Modify: `internal/provider/codex/client.go`
- Modify: `internal/provider/codex/client_test.go`

1. Add failing tests proving repeated resolution calls discovery once and invalidation triggers a new lookup.
2. Run `go test ./internal/provider/codex -run CommandPath -count=1` and confirm the helper is absent.
3. Add mutex-protected resolution and invalidate the path after a launch failure.
4. Run focused and package tests.

### Task 4: Verify the complete change

1. Run `gofmt` on changed Go files.
2. Run `go test ./...`, `go vet ./...`, and `go test -race ./...`.
3. Inspect `git diff --check` and the final diff for unintended changes.
