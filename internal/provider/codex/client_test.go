package codex

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chuongtrh/ai-quota/internal/model"
)

func TestParseRateLimitResult(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	raw := json.RawMessage(`{
  "rateLimits": {
    "limitId": "codex",
    "planType": "plus",
    "primary": {"usedPercent": 25, "windowDurationMins": 300, "resetsAt": 1700010000},
    "secondary": {"usedPercent": 42, "windowDurationMins": 10080, "resetsAt": 1700600000}
  }
}`)
	status, err := parseRateLimitResult(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if status.Metadata["plan"] != "plus" {
		t.Fatalf("plan = %q, want plus", status.Metadata["plan"])
	}
	session, ok := status.Window(model.WindowSession)
	if !ok || session.UsedPercent != 25 {
		t.Fatalf("unexpected session window: %#v", session)
	}
	weekly, ok := status.Window(model.WindowWeekly)
	if !ok || weekly.UsedPercent != 42 {
		t.Fatalf("unexpected weekly window: %#v", weekly)
	}
}

func TestReadRateLimitResponseIgnoresNotifications(t *testing.T) {
	input := "{\"method\":\"account/rateLimits/updated\",\"params\":{}}\n" +
		"{\"id\":2,\"result\":{\"rateLimits\":null}}\n"
	response, err := readRateLimitResponse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != 2 {
		t.Fatalf("id = %d, want 2", response.ID)
	}
}
