package codex

import (
	"encoding/json"
	"errors"
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

func TestCommandPathCachesSuccessfulLookup(t *testing.T) {
	lookups := 0
	client := &Client{
		Command: "codex",
		lookupCommand: func(command string) (string, error) {
			lookups++
			return "/test/bin/" + command, nil
		},
	}

	for range 2 {
		path, err := client.commandPath()
		if err != nil || path != "/test/bin/codex" {
			t.Fatalf("commandPath() = %q, %v; want /test/bin/codex, nil", path, err)
		}
	}
	if lookups != 1 {
		t.Fatalf("lookups = %d; want 1", lookups)
	}
}

func TestCommandPathInvalidationTriggersLookup(t *testing.T) {
	lookups := 0
	client := &Client{
		Command: "codex",
		lookupCommand: func(string) (string, error) {
			lookups++
			if lookups == 1 {
				return "/first/codex", nil
			}
			return "/second/codex", nil
		},
	}

	first, err := client.commandPath()
	if err != nil {
		t.Fatal(err)
	}
	client.invalidateCommandPath(first)
	second, err := client.commandPath()
	if err != nil {
		t.Fatal(err)
	}
	if second != "/second/codex" || lookups != 2 {
		t.Fatalf("second path = %q, lookups = %d; want /second/codex, 2", second, lookups)
	}
}

func TestCommandPathDoesNotCacheLookupError(t *testing.T) {
	lookups := 0
	client := &Client{
		lookupCommand: func(string) (string, error) {
			lookups++
			if lookups == 1 {
				return "", errors.New("not found")
			}
			return "/test/bin/codex", nil
		},
	}

	if _, err := client.commandPath(); err == nil {
		t.Fatal("first commandPath() error = nil; want lookup error")
	}
	if _, err := client.commandPath(); err != nil {
		t.Fatalf("second commandPath() error = %v; want nil", err)
	}
	if lookups != 2 {
		t.Fatalf("lookups = %d; want 2", lookups)
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
