package alerts

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chuongtrh/ai-quota/internal/model"
)

func TestEvaluateDeduplicatesPerWindow(t *testing.T) {
	now := time.Now()
	manager := NewManager(filepath.Join(t.TempDir(), "alerts.json"))
	status := model.ProviderStatus{
		Provider: model.ProviderCodex,
		Windows: []model.Window{
			{Kind: model.WindowSession, UsedPercent: 82, ResetsAt: now.Add(time.Hour)},
		},
	}
	if got := manager.Evaluate(status, now); len(got) != 1 {
		t.Fatalf("first notices = %d, want 1", len(got))
	}
	if got := manager.Evaluate(status, now); len(got) != 0 {
		t.Fatalf("second notices = %d, want 0", len(got))
	}
	status.Windows[0].UsedPercent = 96
	if got := manager.Evaluate(status, now); len(got) != 1 {
		t.Fatalf("critical notices = %d, want 1", len(got))
	}
}
