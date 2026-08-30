package alerts

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/chuongtrh/ai-quota/internal/model"
	"github.com/chuongtrh/ai-quota/internal/storage"
)

type Notice struct {
	Title string
	Body  string
}

type Manager struct {
	path string
	sent map[string]int64
}

func NewManager(path string) *Manager {
	manager := &Manager{path: path, sent: make(map[string]int64)}
	if err := storage.ReadJSON(path, &manager.sent); err != nil && !errors.Is(err, os.ErrNotExist) {
		manager.sent = make(map[string]int64)
	}
	return manager
}

func (m *Manager) Evaluate(status model.ProviderStatus, now time.Time) []Notice {
	m.prune(now)
	if status.Error != "" {
		return nil
	}
	notices := make([]Notice, 0, 2)
	for _, window := range status.Windows {
		if window.ResetsAt.Before(now) {
			continue
		}
		threshold := highestThreshold(window.UsedPercent)
		if threshold == 0 {
			continue
		}
		key := fmt.Sprintf("%s:%s:%d:%d", status.Provider, window.Kind, window.ResetsAt.Unix(), threshold)
		if _, exists := m.sent[key]; exists {
			continue
		}
		m.sent[key] = window.ResetsAt.Unix()
		notices = append(notices, Notice{
			Title: noticeTitle(threshold, status.Provider.DisplayName()),
			Body: fmt.Sprintf(
				"%s has %d%% remaining · %s",
				window.Kind.DisplayName(),
				window.RoundedRemainingPercent(),
				model.FormatReset(now, window.ResetsAt),
			),
		})
	}
	if len(notices) > 0 {
		_ = storage.WriteJSON(m.path, m.sent, 0o600)
	}
	return notices
}

func highestThreshold(used float64) int {
	switch {
	case used >= 100:
		return 100
	case used >= 95:
		return 95
	case used >= 80:
		return 80
	default:
		return 0
	}
}

func noticeTitle(threshold int, provider string) string {
	switch threshold {
	case 100:
		return "⛔ " + provider + " quota exhausted"
	case 95:
		return "🔴 " + provider + " quota almost exhausted"
	default:
		return "🟡 " + provider + " quota running low"
	}
}

func (m *Manager) prune(now time.Time) {
	cutoff := now.Add(-24 * time.Hour).Unix()
	for key, resetAt := range m.sent {
		if resetAt < cutoff {
			delete(m.sent, key)
			continue
		}
	}
}
