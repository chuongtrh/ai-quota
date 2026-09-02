package antigravity

import (
	"bytes"
	"errors"
	"path/filepath"
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
