package antigravity

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/chuongtrh/ai-quota/internal/config"
	"github.com/chuongtrh/ai-quota/internal/model"
	"github.com/chuongtrh/ai-quota/internal/storage"
)

func TestCacheProviderReadsNormalizedStatus(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{DataDir: root, HomeDir: root}
	reset := time.Now().Add(time.Hour)
	if err := storage.WriteJSON(paths.AntigravityCache(), model.ProviderStatus{Windows: []model.Window{
		{Kind: "gemini-weekly", UsedPercent: 25, ResetsAt: reset},
	}}, 0o600); err != nil {
		t.Fatal(err)
	}

	p := &CacheProvider{paths: paths, fetchLS: func(context.Context) (model.ProviderStatus, error) {
		return model.ProviderStatus{}, errors.New("not running")
	}}
	status, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Provider != model.ProviderAntigravity || len(status.Windows) != 1 {
		t.Fatalf("status = %#v", status)
	}
}

func TestCacheProviderPrefersLanguageServer(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{DataDir: root, HomeDir: root}
	cachedReset := time.Now().Add(time.Hour)
	if err := storage.WriteJSON(paths.AntigravityCache(), model.ProviderStatus{Windows: []model.Window{
		{Kind: "gemini-weekly", UsedPercent: 50, ResetsAt: cachedReset},
	}}, 0o600); err != nil {
		t.Fatal(err)
	}

	liveReset := time.Now().Add(2 * time.Hour)
	p := &CacheProvider{
		paths: paths,
		fetchLS: func(context.Context) (model.ProviderStatus, error) {
			return model.ProviderStatus{
				Provider: model.ProviderAntigravity,
				Windows: []model.Window{
					{Kind: "gemini-weekly", UsedPercent: 10, ResetsAt: liveReset},
				},
			}, nil
		},
	}

	status, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Windows) != 1 || status.Windows[0].UsedPercent != 10 {
		t.Fatalf("unexpected status from LS: %#v", status)
	}

	// Verify it was written to cache
	var cached model.ProviderStatus
	if err := storage.ReadJSON(paths.AntigravityCache(), &cached); err != nil {
		t.Fatalf("failed to read cache: %v", err)
	}
	if len(cached.Windows) != 1 || cached.Windows[0].UsedPercent != 10 {
		t.Fatalf("cache was not updated with live quota: %#v", cached)
	}
}

func TestCacheProviderReportsNoQuotaData(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{DataDir: filepath.Join(root, "missing"), HomeDir: root}
	p := &CacheProvider{
		paths: paths,
		fetchLS: func(context.Context) (model.ProviderStatus, error) {
			return model.ProviderStatus{}, errors.New("offline")
		},
	}
	_, err := p.Fetch(context.Background())
	if !errors.Is(err, ErrNoQuotaData) {
		t.Fatalf("error = %v, want ErrNoQuotaData", err)
	}
}
