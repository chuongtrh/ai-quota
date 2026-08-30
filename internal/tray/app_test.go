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
