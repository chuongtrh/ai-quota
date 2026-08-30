# AI quota

A lightweight macOS menu bar app for tracking Codex and Claude Code subscription quotas. It shows the remaining percentage, reset time, and low-quota alerts with a deliberately simple interface.

## Features

- Tracks Codex **session** and **weekly** quota windows.
- Tracks Claude Code **5-hour** and **7-day** quota windows.
- Shows the most urgent remaining quota in the menu bar.
- Sends native macOS alerts at 20%, 5%, and 0% remaining.
- Requires no server, database, or separate account.
- Never reads or stores Codex or Claude Code authentication tokens.
- Provides a `Provider` interface for adding more AI providers.

## Official data sources

AI quota uses only integration surfaces documented by each provider:

- **Codex:** starts the local `codex app-server` process and calls `account/rateLimits/read` over JSON-RPC. See [Codex App Server](https://developers.openai.com/codex/app-server).
- **Claude Code:** receives `rate_limits.five_hour` and `rate_limits.seven_day` through the official status line JSON payload. See [Claude Code status line](https://code.claude.com/docs/en/statusline).

The app does not call undocumented endpoints, scrape web pages, or proxy AI requests through another server.

## Requirements

- macOS 12 or later.
- Go 1.24 or later.
- Xcode Command Line Tools:

  ```bash
  xcode-select --install
  ```

- An installed and authenticated Codex CLI for Codex tracking.
- Claude Code 2.1.251 or later with a Claude Pro or Max plan for Claude Code tracking.

## Build the macOS app

From the project directory, run:

```bash
./scripts/build.sh
```

The script will:

1. Download Go dependencies.
2. Run the full test suite.
3. Generate the `.icns` app icon.
4. Build the CGO application.
5. Create and locally sign the app bundle.
6. Create a ZIP archive for distribution.

Outputs:

```text
dist/AIQuota.app
dist/AIQuota-0.1.0-macos.zip
```

Launch the app:

```bash
open dist/AIQuota.app
```

Set a custom version:

```bash
VERSION=0.2.0 BUILD_NUMBER=2 ./scripts/build.sh
```

If you have an Apple Developer certificate, sign with your identity:

```bash
CODESIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)" ./scripts/build.sh
```

The default signing identity is `-`, which creates an ad-hoc signature suitable for running the app on the Mac that built it.

## Development

```bash
go mod download
go test ./...
CGO_ENABLED=1 go run ./cmd/aiquota
```

Or use the Makefile:

```bash
make test
make run
make build
```

## Usage

### Codex

When the Codex CLI is installed and authenticated, AI quota discovers the `codex` executable and refreshes its quota once per minute. It checks common Homebrew, npm, nvm, and `~/.local/bin` locations because apps launched from Finder often receive a smaller `PATH` than Terminal sessions.

For a custom Codex location during development, set `AIQUOTA_CODEX_PATH` before launching the app.

### Claude Code

1. Open AI quota from the macOS menu bar.
2. Select **🔌 Connect Claude Code**.
3. Open Claude Code and send at least one prompt.

When connecting, AI quota:

- Creates a timestamped backup of `~/.claude/settings.json`.
- Separately saves the current `statusLine` value.
- Installs its bridge at `~/Library/Application Support/AIQuota/bin/aiquota-bridge`.
- Passes through output from an existing status line command so the current Claude Code display remains unchanged.

Selecting **Disconnect Claude Code** restores the previous `statusLine` exactly. If another app changed the setting after AI quota connected, AI quota stops without overwriting the newer configuration.

## Local data

AI quota stores local state under:

```text
~/Library/Application Support/AIQuota/
```

This directory contains the latest quota snapshots, alert deduplication state, the Claude Code bridge, and the status line backup. It never contains conversation content, project source code, API keys, or OAuth tokens.

## Status indicators

- 🟢 More than 20% remaining.
- 🟡 6% to 20% remaining.
- 🔴 1% to 5% remaining.
- ⛔ Exhausted.
- ⚪ The window reset and is waiting for fresh provider data.

Remaining quota is calculated as `100 - used_percentage` from provider-reported data.

## Current limitations

- Claude Code sends quota fields only after the first API response in a session.
- Claude documents these status line fields for Pro and Max plans; Team and Enterprise accounts may not provide them.
- Version 0.1 targets macOS and builds for the architecture of the Mac running the script.
- AI quota displays provider-reported values and does not estimate quota usage.

## Project structure

```text
cmd/aiquota                 App entry point and Claude bridge mode
cmd/icon-gen                macOS iconset generator
internal/appcore            Refresh orchestration and in-memory state
internal/provider/codex     Codex App Server connector
internal/provider/claude    Status line connector and settings backup
internal/alerts             Alert thresholds and deduplication
internal/notify             Native macOS UserNotifications bridge
internal/tray               Menu bar interface
scripts/build.sh            Test, build, package, and sign the .app
```

## License

MIT
