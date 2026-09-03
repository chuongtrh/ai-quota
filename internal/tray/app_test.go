package tray

import (
	"errors"
	"testing"
	"time"

	"github.com/chuongtrh/ai-quota/internal/model"
)

func TestMostUrgentRemainingIgnoresHiddenProviders(t *testing.T) {
	now := time.Now()
	statuses := map[model.Provider]model.ProviderStatus{
		model.ProviderCodex: {
			Provider: model.ProviderCodex,
			Windows: []model.Window{{
				Kind:        model.WindowSession,
				UsedPercent: 40,
				ResetsAt:    now.Add(time.Hour),
			}},
		},
		model.ProviderClaudeCode: {
			Provider: model.ProviderClaudeCode,
			Windows: []model.Window{{
				Kind:        model.WindowSession,
				UsedPercent: 99,
				ResetsAt:    now.Add(time.Hour),
			}},
		},
	}
	visible := map[model.Provider]bool{
		model.ProviderCodex:      true,
		model.ProviderClaudeCode: false,
	}
	remaining, ok := mostUrgentRemaining(statuses, visible, now)
	if !ok || remaining != 60 {
		t.Fatalf("remaining = %d, ok = %v; want 60, true", remaining, ok)
	}
}

func TestProviderOrderIncludesAntigravity(t *testing.T) {
	want := []model.Provider{model.ProviderCodex, model.ProviderClaudeCode, model.ProviderAntigravity}
	if len(providerOrder) != len(want) {
		t.Fatalf("providerOrder = %#v", providerOrder)
	}
	for index := range want {
		if providerOrder[index] != want[index] {
			t.Fatalf("providerOrder[%d] = %q, want %q", index, providerOrder[index], want[index])
		}
	}
}

func TestConnectionStateCacheReusesRecentCheck(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	checks := 0
	cache := connectionStateCache{
		check: func() (bool, error) {
			checks++
			return true, nil
		},
		interval: time.Minute,
		now:      func() time.Time { return now },
	}

	connected, err := cache.Get(false)
	if err != nil || !connected {
		t.Fatalf("first Get() = %v, %v; want true, nil", connected, err)
	}
	now = now.Add(59 * time.Second)
	connected, err = cache.Get(false)
	if err != nil || !connected {
		t.Fatalf("cached Get() = %v, %v; want true, nil", connected, err)
	}
	if checks != 1 {
		t.Fatalf("checks = %d; want 1", checks)
	}
}

func TestConnectionStateCacheForceRefreshes(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	checks := 0
	cache := connectionStateCache{
		check: func() (bool, error) {
			checks++
			return checks > 1, nil
		},
		interval: time.Minute,
		now:      func() time.Time { return now },
	}

	connected, _ := cache.Get(false)
	if connected {
		t.Fatal("first Get() = true; want false")
	}
	connected, err := cache.Get(true)
	if err != nil || !connected {
		t.Fatalf("forced Get() = %v, %v; want true, nil", connected, err)
	}
	if checks != 2 {
		t.Fatalf("checks = %d; want 2", checks)
	}
}

func TestFormatCustomWindowUsesLabel(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	window := model.Window{
		Kind:        "gemini-weekly",
		Label:       "Gemini Weekly",
		UsedPercent: 25,
		ResetsAt:    now.Add(2 * time.Hour),
	}
	if got := formatQuotaWindow(window, now); got != "🟢 Gemini Weekly · 75% left · resets in 2h 00m" {
		t.Fatalf("formatted window = %q", got)
	}
}

func TestWindowRowsReturnsNormalizedOrder(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	status := model.ProviderStatus{Windows: []model.Window{
		{Kind: "gemini-weekly", Label: "Gemini Weekly", UsedPercent: 25, ResetsAt: now.Add(2 * time.Hour)},
		{Kind: model.WindowSession, UsedPercent: 40, ResetsAt: now.Add(time.Hour)},
	}}
	status.Normalize()
	rows := windowRows(status, now)
	if len(rows) != 2 {
		t.Fatalf("rows = %#v; want two", rows)
	}
	if rows[0] != "🟢 Session · 60% left · resets in 1h 00m" {
		t.Fatalf("first row = %q", rows[0])
	}
	if rows[1] != "🟢 Gemini Weekly · 75% left · resets in 2h 00m" {
		t.Fatalf("second row = %q", rows[1])
	}
}

func TestAntigravityQuotaRowsIncludeEveryBucket(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	status := model.ProviderStatus{
		Provider: model.ProviderAntigravity,
		Windows: []model.Window{
			{Kind: "claude-daily", Label: "Claude Daily", UsedPercent: 50, ResetsAt: now.Add(time.Hour)},
			{Kind: "gemini-weekly", Label: "Gemini Weekly", UsedPercent: 25, ResetsAt: now.Add(7 * 24 * time.Hour)},
		},
	}
	status.Normalize()
	rows := quotaRowsForProvider(status, now)
	if len(rows) != 2 || rows[0] == rows[1] {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestStatusLineProviderVisibilityRequiresConnection(t *testing.T) {
	status := model.ProviderStatus{
		Provider: model.ProviderAntigravity,
		Windows:  []model.Window{{Kind: "gemini-weekly", ResetsAt: time.Now().Add(time.Hour)}},
	}
	if providerVisible(model.ProviderAntigravity, status, false, nil) {
		t.Fatal("disconnected Antigravity status is visible")
	}
	if providerVisible(model.ProviderAntigravity, status, true, errors.New("settings unavailable")) {
		t.Fatal("Antigravity status with settings error is visible")
	}
	if !providerVisible(model.ProviderAntigravity, status, true, nil) {
		t.Fatal("connected Antigravity status is hidden")
	}
	// Even with empty windows, connected provider should be visible in menu
	empty := model.ProviderStatus{Provider: model.ProviderAntigravity}
	if !providerVisible(model.ProviderAntigravity, empty, true, nil) {
		t.Fatal("connected Antigravity without windows is hidden")
	}
}

func TestQuotaRowsWaitingWhenEmpty(t *testing.T) {
	now := time.Now()
	status := model.ProviderStatus{Provider: model.ProviderAntigravity}
	rows := quotaRowsForProvider(status, now)
	if len(rows) != 1 || rows[0] != "⏳ Waiting for quota data" {
		t.Fatalf("rows = %#v; want ⏳ Waiting for quota data", rows)
	}
}

func TestAntigravityProviderMenuStates(t *testing.T) {
	tests := []struct {
		name        string
		status      model.ProviderStatus
		connected   bool
		settingsErr error
		wantStatus  string
		wantAction  string
	}{
		{name: "disabled", wantStatus: "⚪ Tracking disabled", wantAction: "Enable tracking"},
		{name: "settings error", settingsErr: errors.New("bad settings"), wantStatus: "⚠️ Settings unavailable", wantAction: "Enable tracking"},
		{name: "waiting", connected: true, wantStatus: "⏳ Waiting for quota data", wantAction: "Disable tracking"},
		{
			name:      "active",
			connected: true,
			status: model.ProviderStatus{Windows: []model.Window{
				{Kind: "gemini-weekly", ResetsAt: time.Now().Add(time.Hour)},
			}},
			wantStatus: "🟢 Tracking active",
			wantAction: "Disable tracking",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := statusLineProviderMenuState(model.ProviderAntigravity, test.status, test.connected, test.settingsErr)
			if state.statusTitle != test.wantStatus || state.actionTitle != test.wantAction {
				t.Fatalf("state = %#v", state)
			}
		})
	}
}
