package antigravity

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chuongtrh/ai-quota/internal/config"
	"github.com/chuongtrh/ai-quota/internal/model"
	"github.com/chuongtrh/ai-quota/internal/storage"
)

func TestParseStatusLineMultipleQuotaBuckets(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	raw := []byte(`{
  "email": "must-not-be-cached@example.com",
  "transcript_path": "/private/transcript.jsonl",
  "quota": {
    "gemini-weekly": {
      "remaining_fraction": 0.9378,
      "reset_time": "2026-09-09T10:00:00Z",
      "reset_in_seconds": 604800
    },
    "claude-daily": {
      "remaining_fraction": 0,
      "reset_in_seconds": 3600
    }
  }
}`)

	status, err := ParseStatusLine(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if status.Provider != model.ProviderAntigravity || len(status.Windows) != 2 {
		t.Fatalf("unexpected status: %#v", status)
	}
	claude := status.Windows[0]
	if claude.Kind != "claude-daily" || claude.Label != "Claude Daily" || claude.UsedPercent != 100 {
		t.Fatalf("unexpected Claude bucket: %#v", claude)
	}
	if !claude.ResetsAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("Claude reset = %v", claude.ResetsAt)
	}
	gemini := status.Windows[1]
	if gemini.Kind != "gemini-weekly" || gemini.Label != "Gemini Weekly" {
		t.Fatalf("unexpected Gemini bucket: %#v", gemini)
	}
	if diff := gemini.UsedPercent - 6.22; diff < -0.0001 || diff > 0.0001 {
		t.Fatalf("Gemini used = %v, want 6.22", gemini.UsedPercent)
	}
	if !gemini.ResetsAt.Equal(time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("Gemini reset = %v", gemini.ResetsAt)
	}
	if len(status.Metadata) != 0 {
		t.Fatalf("metadata leaked into cache status: %#v", status.Metadata)
	}
}

func TestParseStatusLineSkipsMalformedQuotaEntries(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	raw := []byte(`{
  "quota": {
    "missing-fraction": {"reset_in_seconds": 60},
    "missing-reset": {"remaining_fraction": 0.5},
    "bad-reset": {"remaining_fraction": 0.5, "reset_time": "tomorrow"},
    "valid": {"remaining_fraction": 0.25, "reset_in_seconds": 120}
  }
}`)

	status, err := ParseStatusLine(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Windows) != 1 || status.Windows[0].Kind != "valid" {
		t.Fatalf("windows = %#v", status.Windows)
	}
}

func TestParseStatusLineRejectsMissingQuota(t *testing.T) {
	_, err := ParseStatusLine([]byte(`{"product":"antigravity"}`), time.Now())
	if !errors.Is(err, ErrNoQuotaData) {
		t.Fatalf("error = %v, want ErrNoQuotaData", err)
	}
}

func TestBridgeCachesQuotaAndFormatsFallback(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{DataDir: filepath.Join(root, "data"), HomeDir: filepath.Join(root, "home")}
	input := bytes.NewBufferString(`{"quota":{"gemini-weekly":{"remaining_fraction":0.75,"reset_in_seconds":3600}}}`)
	var output bytes.Buffer

	if err := RunBridge(paths, input, &output); err != nil {
		t.Fatal(err)
	}
	var status model.ProviderStatus
	if err := storage.ReadJSON(paths.AntigravityCache(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Provider != model.ProviderAntigravity || len(status.Windows) != 1 || status.Windows[0].UsedPercent != 25 {
		t.Fatalf("cached status = %#v", status)
	}
	if got := output.String(); got != "[Antigravity] · Gemini Weekly: 75% left\n" {
		t.Fatalf("bridge output = %q", got)
	}
}

func TestInstallerConnectCreatesSettingsAndHelper(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{DataDir: filepath.Join(root, "data"), HomeDir: filepath.Join(root, "home")}
	executable := filepath.Join(root, "AIQuota")
	if err := os.WriteFile(executable, []byte("fake executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	installer := Installer{Paths: paths, Executable: executable}

	if err := installer.Connect(); err != nil {
		t.Fatal(err)
	}
	connected, err := installer.IsConnected()
	if err != nil || !connected {
		t.Fatalf("connected = %v, err = %v", connected, err)
	}
	helper, err := os.ReadFile(paths.AntigravityHelper())
	if err != nil || string(helper) != "fake executable" {
		t.Fatalf("helper = %q, err = %v", helper, err)
	}
	var settings map[string]json.RawMessage
	if err := storage.ReadJSON(paths.AntigravitySettings(), &settings); err != nil {
		t.Fatal(err)
	}
	if !isOurStatusLine(settings["statusLine"]) {
		t.Fatalf("statusLine = %s", settings["statusLine"])
	}
}

func TestInstallerDisconnectRestoresExactStatusLineValue(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{DataDir: filepath.Join(root, "data"), HomeDir: filepath.Join(root, "home")}
	if err := os.MkdirAll(filepath.Dir(paths.AntigravitySettings()), 0o700); err != nil {
		t.Fatal(err)
	}
	originalStatusLine := map[string]any{
		"type":               "command",
		"command":            "old-status-line --color=always",
		"padding":            float64(2),
		"enabled":            false,
		"stack_with_default": false,
	}
	originalSettings := map[string]any{"colorScheme": "dark", "statusLine": originalStatusLine}
	if err := storage.WriteJSON(paths.AntigravitySettings(), originalSettings, 0o600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "AIQuota")
	if err := os.WriteFile(executable, []byte("fake executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	installer := Installer{Paths: paths, Executable: executable}

	if err := installer.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := installer.Disconnect(); err != nil {
		t.Fatal(err)
	}
	var restored map[string]any
	if err := storage.ReadJSON(paths.AntigravitySettings(), &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored["statusLine"], originalStatusLine) {
		t.Fatalf("restored statusLine = %#v, want %#v", restored["statusLine"], originalStatusLine)
	}
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(paths.AntigravitySettings()), "settings.aiquota-backup-*.json"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("settings backups = %#v, err = %v", backups, err)
	}
}

func TestInstallerDisconnectRejectsChangedSettings(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{DataDir: filepath.Join(root, "data"), HomeDir: filepath.Join(root, "home")}
	executable := filepath.Join(root, "AIQuota")
	if err := os.WriteFile(executable, []byte("fake executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	installer := Installer{Paths: paths, Executable: executable}
	if err := installer.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteJSON(paths.AntigravitySettings(), map[string]any{
		"statusLine": map[string]any{"type": "command", "command": "changed-elsewhere"},
	}, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := installer.Disconnect(); !errors.Is(err, ErrSettingsChanged) {
		t.Fatalf("Disconnect error = %v, want ErrSettingsChanged", err)
	}
}

func TestBridgePassesOriginalPayloadToPreviousStatusLine(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{DataDir: filepath.Join(root, "data"), HomeDir: filepath.Join(root, "home")}
	captured := filepath.Join(root, "captured.json")
	script := filepath.Join(root, "previous-statusline.sh")
	scriptBody := "#!/bin/sh\ncat > \"$1\"\nprintf 'previous status\\n'\n"
	if err := os.WriteFile(script, []byte(scriptBody), 0o700); err != nil {
		t.Fatal(err)
	}
	backup := StatusLineBackup{
		HadValue: true,
		Value: mustJSON(map[string]any{
			"type":    "command",
			"command": shellQuote(script) + " " + shellQuote(captured),
		}),
	}
	if err := storage.WriteJSON(paths.AntigravityStatusLineBackup(), backup, 0o600); err != nil {
		t.Fatal(err)
	}
	raw := `{"quota":{"gemini-weekly":{"remaining_fraction":0.75,"reset_in_seconds":3600}}}`
	var output bytes.Buffer

	if err := RunBridge(paths, strings.NewReader(raw), &output); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "previous status\n" {
		t.Fatalf("bridge output = %q", got)
	}
	forwarded, err := os.ReadFile(captured)
	if err != nil {
		t.Fatal(err)
	}
	if string(forwarded) != raw {
		t.Fatalf("forwarded payload = %q, want %q", forwarded, raw)
	}
}
