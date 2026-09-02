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

	status, err := NewCacheProvider(paths).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Provider != model.ProviderAntigravity || len(status.Windows) != 1 {
		t.Fatalf("status = %#v", status)
	}
}

func TestCacheProviderReportsNoQuotaData(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{DataDir: filepath.Join(root, "missing"), HomeDir: root}
	_, err := NewCacheProvider(paths).Fetch(context.Background())
	if !errors.Is(err, ErrNoQuotaData) {
		t.Fatalf("error = %v, want ErrNoQuotaData", err)
	}
}
