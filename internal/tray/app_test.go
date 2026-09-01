package tray

import (
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
