package claude

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chuongtrh/ai-quota/internal/config"
	"github.com/chuongtrh/ai-quota/internal/model"
	"github.com/chuongtrh/ai-quota/internal/storage"
)

func TestParseStatusLine(t *testing.T) {
	raw := []byte(`{
  "model": {"display_name": "Opus"},
  "rate_limits": {
    "five_hour": {"used_percentage": 23.5, "resets_at": 1738425600},
    "seven_day": {"used_percentage": 41.2, "resets_at": 1738857600}
  }
}`)
	status, err := ParseStatusLine(raw, time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	session, ok := status.Window(model.WindowSession)
	if !ok || session.UsedPercent != 23.5 {
		t.Fatalf("unexpected session: %#v", session)
	}
	weekly, ok := status.Window(model.WindowWeekly)
	if !ok || weekly.UsedPercent != 41.2 {
		t.Fatalf("unexpected weekly: %#v", weekly)
	}
}

func TestInstallerConnectAndDisconnectRestoresStatusLine(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		DataDir: filepath.Join(root, "data"),
		HomeDir: filepath.Join(root, "home"),
	}
	if err := os.MkdirAll(filepath.Dir(paths.ClaudeSettings()), 0o700); err != nil {
		t.Fatal(err)
	}
	originalStatusLine := map[string]any{"type": "command", "command": "old-status-line"}
	originalSettings := map[string]any{
		"theme":      "dark",
		"statusLine": originalStatusLine,
	}
	if err := storage.WriteJSON(paths.ClaudeSettings(), originalSettings, 0o600); err != nil {
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
	connected, err := installer.IsConnected()
	if err != nil || !connected {
		t.Fatalf("connected = %v, err = %v", connected, err)
	}
	if err := installer.Disconnect(); err != nil {
		t.Fatal(err)
	}
	var restored map[string]json.RawMessage
	if err := storage.ReadJSON(paths.ClaudeSettings(), &restored); err != nil {
		t.Fatal(err)
	}
	var statusLine map[string]any
	if err := json.Unmarshal(restored["statusLine"], &statusLine); err != nil {
		t.Fatal(err)
	}
	if statusLine["command"] != "old-status-line" {
		t.Fatalf("restored command = %#v", statusLine["command"])
	}
}

func TestBridgeCachesQuota(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{DataDir: root, HomeDir: root}
	input := bytes.NewBufferString(`{"rate_limits":{"five_hour":{"used_percentage":80,"resets_at":1738425600}}}`)
	var output bytes.Buffer
	if err := RunBridge(paths, input, &output); err != nil {
		t.Fatal(err)
	}
	var status model.ProviderStatus
	if err := storage.ReadJSON(paths.ClaudeCache(), &status); err != nil {
		t.Fatal(err)
	}
	if len(status.Windows) != 1 || status.Windows[0].UsedPercent != 80 {
		t.Fatalf("unexpected cached status: %#v", status)
	}
}
