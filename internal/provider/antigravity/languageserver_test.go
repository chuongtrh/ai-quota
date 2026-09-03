package antigravity

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/chuongtrh/ai-quota/internal/model"
)

func TestParseLanguageServerQuotaValid(t *testing.T) {
	now := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	raw := []byte(`{
  "response": {
    "groups": [
      {
        "displayName": "Gemini Models",
        "buckets": [
          {
            "bucketId": "gemini-weekly",
            "displayName": "Weekly Limit Remaining",
            "window": "weekly",
            "remainingFraction": 0.8,
            "resetTime": "2026-09-10T10:00:00Z"
          },
          {
            "bucketId": "gemini-5h",
            "displayName": "Five Hour Limit Remaining",
            "window": "5h",
            "remainingFraction": 0.5,
            "resetTime": "2026-09-03T15:00:00Z"
          }
        ]
      },
      {
        "displayName": "Claude and GPT models",
        "buckets": [
          {
            "bucketId": "3p-weekly",
            "displayName": "Weekly Limit Remaining",
            "window": "weekly",
            "remainingFraction": 1.0,
            "resetTime": "2026-09-10T10:00:00Z"
          },
          {
            "bucketId": "3p-5h",
            "displayName": "Five Hour Limit Remaining",
            "window": "5h",
            "remainingFraction": 0.25,
            "resetTime": "2026-09-03T15:00:00Z"
          }
        ]
      }
    ]
  }
}`)

	status, err := ParseLanguageServerQuota(raw, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Provider != model.ProviderAntigravity {
		t.Fatalf("provider = %s, want %s", status.Provider, model.ProviderAntigravity)
	}
	if len(status.Windows) != 4 {
		t.Fatalf("got %d windows, want 4", len(status.Windows))
	}

	buckets := make(map[string]model.Window)
	for _, w := range status.Windows {
		buckets[string(w.Kind)] = w
	}

	gw, ok := buckets["gemini-weekly"]
	if !ok || gw.Label != "Gemini Weekly" || math.Abs(gw.UsedPercent-20) > 0.0001 {
		t.Fatalf("unexpected gemini-weekly: %#v", gw)
	}

	g5, ok := buckets["gemini-5h"]
	if !ok || g5.Label != "Gemini 5-Hour" || math.Abs(g5.UsedPercent-50) > 0.0001 {
		t.Fatalf("unexpected gemini-5h: %#v", g5)
	}

	cw, ok := buckets["3p-weekly"]
	if !ok || cw.Label != "Claude/GPT Weekly" || math.Abs(cw.UsedPercent-0) > 0.0001 {
		t.Fatalf("unexpected 3p-weekly: %#v", cw)
	}

	c5, ok := buckets["3p-5h"]
	if !ok || c5.Label != "Claude/GPT 5-Hour" || math.Abs(c5.UsedPercent-75) > 0.0001 {
		t.Fatalf("unexpected 3p-5h: %#v", c5)
	}
}

func TestParseLanguageServerQuotaInvalid(t *testing.T) {
	now := time.Now()
	_, err := ParseLanguageServerQuota([]byte(`{"response":{"groups":[]}}`), now)
	if !errors.Is(err, ErrLanguageServerNoQuota) {
		t.Fatalf("expected ErrLanguageServerNoQuota, got %v", err)
	}

	_, err = ParseLanguageServerQuota([]byte(`invalid json`), now)
	if err == nil {
		t.Fatal("expected json decode error, got nil")
	}
}

func TestFormatBucketLabel(t *testing.T) {
	tests := []struct {
		groupName  string
		bucketName string
		bucketID   string
		want       string
	}{
		{"Gemini Models", "Weekly Limit Remaining", "gemini-weekly", "Gemini Weekly"},
		{"Gemini Models", "Five Hour Limit Remaining", "gemini-5h", "Gemini 5-Hour"},
		{"Claude and GPT models", "Weekly Limit Remaining", "3p-weekly", "Claude/GPT Weekly"},
		{"Claude and GPT models", "Five Hour Limit Remaining", "3p-5h", "Claude/GPT 5-Hour"},
		{"", "", "custom-bucket", "Custom Bucket"},
	}

	for _, tt := range tests {
		got := formatBucketLabel(tt.groupName, tt.bucketName, tt.bucketID)
		if got != tt.want {
			t.Errorf("formatBucketLabel(%q, %q, %q) = %q, want %q", tt.groupName, tt.bucketName, tt.bucketID, got, tt.want)
		}
	}
}

func TestLiveLanguageServerIfRunning(t *testing.T) {
	status, err := FetchFromLanguageServer(context.Background())
	if err != nil {
		t.Skipf("no live language server: %v", err)
	}
	t.Logf("Found live status: %d windows", len(status.Windows))
	for _, w := range status.Windows {
		t.Logf("Window: %s (%s) - %.1f%% used, resets %v", w.Label, w.Kind, w.UsedPercent, w.ResetsAt)
	}
	if len(status.Windows) == 0 {
		t.Fatal("expected windows from live language server")
	}
}
