package model

import (
	"testing"
	"time"
)

func TestRemainingAndSeverity(t *testing.T) {
	window := Window{UsedPercent: 82.4}
	if got := window.RoundedRemainingPercent(); got != 18 {
		t.Fatalf("remaining = %d, want 18", got)
	}
	if got := SeverityForRemaining(window.RemainingPercent()); got != SeverityWarning {
		t.Fatalf("severity = %v, want warning", got)
	}
}

func TestClassifyWindow(t *testing.T) {
	if got := ClassifyWindow(5 * 60); got != WindowSession {
		t.Fatalf("5h = %q, want session", got)
	}
	if got := ClassifyWindow(7 * 24 * 60); got != WindowWeekly {
		t.Fatalf("7d = %q, want weekly", got)
	}
}

func TestNormalizeKeepsNewestWindow(t *testing.T) {
	now := time.Now()
	status := ProviderStatus{Windows: []Window{
		{Kind: WindowSession, UsedPercent: 20, ResetsAt: now.Add(time.Hour)},
		{Kind: WindowSession, UsedPercent: 30, ResetsAt: now.Add(2 * time.Hour)},
	}}
	status.Normalize()
	if len(status.Windows) != 1 || status.Windows[0].UsedPercent != 30 {
		t.Fatalf("unexpected normalized windows: %#v", status.Windows)
	}
}
