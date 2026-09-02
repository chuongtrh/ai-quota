package model

import (
	"fmt"
	"math"
	"sort"
	"time"
)

type Provider string

const (
	ProviderCodex       Provider = "codex"
	ProviderClaudeCode  Provider = "claude-code"
	ProviderAntigravity Provider = "antigravity"
)

func (p Provider) DisplayName() string {
	switch p {
	case ProviderCodex:
		return "Codex"
	case ProviderClaudeCode:
		return "Claude Code"
	case ProviderAntigravity:
		return "Google Antigravity"
	default:
		return string(p)
	}
}

type WindowKind string

const (
	WindowSession WindowKind = "session"
	WindowWeekly  WindowKind = "weekly"
)

func (k WindowKind) DisplayName() string {
	switch k {
	case WindowSession:
		return "Session"
	case WindowWeekly:
		return "Weekly"
	default:
		return string(k)
	}
}

type Window struct {
	Kind            WindowKind `json:"kind"`
	Label           string     `json:"label,omitempty"`
	UsedPercent     float64    `json:"used_percent"`
	DurationMinutes int64      `json:"duration_minutes,omitempty"`
	ResetsAt        time.Time  `json:"resets_at"`
}

func (w Window) DisplayName() string {
	if w.Label != "" {
		return w.Label
	}
	return w.Kind.DisplayName()
}

func (w Window) RemainingPercent() float64 {
	return clamp(100-w.UsedPercent, 0, 100)
}

func (w Window) RoundedRemainingPercent() int {
	return int(math.Round(w.RemainingPercent()))
}

func (w Window) Valid() bool {
	return w.Kind != "" && !w.ResetsAt.IsZero() && !math.IsNaN(w.UsedPercent)
}

type ProviderStatus struct {
	Provider  Provider          `json:"provider"`
	Windows   []Window          `json:"windows,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
	Error     string            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func (s ProviderStatus) Window(kind WindowKind) (Window, bool) {
	for _, window := range s.Windows {
		if window.Kind == kind {
			return window, true
		}
	}
	return Window{}, false
}

func (s *ProviderStatus) Normalize() {
	best := make(map[WindowKind]Window)
	for _, window := range s.Windows {
		if !window.Valid() {
			continue
		}
		window.UsedPercent = clamp(window.UsedPercent, 0, 100)
		previous, exists := best[window.Kind]
		if !exists || window.ResetsAt.After(previous.ResetsAt) {
			best[window.Kind] = window
		}
	}

	s.Windows = s.Windows[:0]
	for _, window := range best {
		s.Windows = append(s.Windows, window)
	}
	sort.SliceStable(s.Windows, func(i, j int) bool {
		leftRank := windowKindRank(s.Windows[i].Kind)
		rightRank := windowKindRank(s.Windows[j].Kind)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return s.Windows[i].Kind < s.Windows[j].Kind
	})
}

func windowKindRank(kind WindowKind) int {
	switch kind {
	case WindowSession:
		return 0
	case WindowWeekly:
		return 1
	default:
		return 2
	}
}

func ClassifyWindow(durationMinutes int64) WindowKind {
	if durationMinutes >= int64((24*time.Hour)/time.Minute) {
		return WindowWeekly
	}
	return WindowSession
}

type Severity int

const (
	SeverityHealthy Severity = iota
	SeverityWarning
	SeverityCritical
	SeverityExhausted
)

func SeverityForRemaining(remaining float64) Severity {
	switch {
	case remaining <= 0:
		return SeverityExhausted
	case remaining <= 5:
		return SeverityCritical
	case remaining <= 20:
		return SeverityWarning
	default:
		return SeverityHealthy
	}
}

func (s Severity) Emoji() string {
	switch s {
	case SeverityExhausted:
		return "⛔"
	case SeverityCritical:
		return "🔴"
	case SeverityWarning:
		return "🟡"
	default:
		return "🟢"
	}
}

func FormatReset(now, resetsAt time.Time) string {
	if resetsAt.IsZero() {
		return "reset time unavailable"
	}
	remaining := resetsAt.Sub(now)
	if remaining <= 0 {
		return "waiting for data after reset"
	}
	days := int(remaining / (24 * time.Hour))
	if days > 0 {
		hours := int((remaining % (24 * time.Hour)) / time.Hour)
		return fmt.Sprintf("resets in %dd %dh", days, hours)
	}
	hours := int(remaining / time.Hour)
	minutes := int((remaining % time.Hour) / time.Minute)
	if hours == 0 {
		return fmt.Sprintf("resets in %d min", max(minutes, 1))
	}
	return fmt.Sprintf("resets in %dh %02dm", hours, minutes)
}

func clamp(value, low, high float64) float64 {
	return math.Min(math.Max(value, low), high)
}
