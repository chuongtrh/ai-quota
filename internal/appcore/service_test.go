package appcore

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/chuongtrh/ai-quota/internal/alerts"
	"github.com/chuongtrh/ai-quota/internal/config"
	"github.com/chuongtrh/ai-quota/internal/model"
	providerapi "github.com/chuongtrh/ai-quota/internal/provider"
	"github.com/chuongtrh/ai-quota/internal/provider/antigravity"
	"github.com/chuongtrh/ai-quota/internal/storage"
)

type blockingProvider struct {
	id      model.Provider
	started chan<- model.Provider
	release <-chan struct{}
}

type errorProvider struct {
	id  model.Provider
	err error
}

func (p errorProvider) ID() model.Provider {
	return p.id
}

func (p errorProvider) Fetch(context.Context) (model.ProviderStatus, error) {
	return model.ProviderStatus{Provider: p.id}, p.err
}

func (p blockingProvider) ID() model.Provider {
	return p.id
}

func (p blockingProvider) Fetch(ctx context.Context) (model.ProviderStatus, error) {
	p.started <- p.id
	select {
	case <-p.release:
		return model.ProviderStatus{Provider: p.id}, nil
	case <-ctx.Done():
		return model.ProviderStatus{Provider: p.id}, ctx.Err()
	}
}

func TestRefreshFetchesProvidersConcurrently(t *testing.T) {
	started := make(chan model.Provider, 2)
	release := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	providers := []providerapi.Provider{
		blockingProvider{id: model.ProviderCodex, started: started, release: release},
		blockingProvider{id: model.ProviderClaudeCode, started: started, release: release},
	}
	paths := config.Paths{DataDir: t.TempDir(), HomeDir: t.TempDir()}
	service := &Service{
		paths:     paths,
		providers: providers,
		alerts:    alerts.NewManager(paths.AlertState()),
		statuses:  make(map[model.Provider]model.ProviderStatus),
	}
	done := make(chan struct{})
	go func() {
		service.Refresh(context.Background())
		close(done)
	}()

	for range providers {
		select {
		case <-started:
		case <-time.After(250 * time.Millisecond):
			t.Fatal("providers did not enter Fetch concurrently")
		}
	}
	close(release)
	released = true
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Refresh did not finish after providers were released")
	}
}

func TestNewRegistersAntigravityAndLoadsCache(t *testing.T) {
	paths := config.Paths{DataDir: t.TempDir(), HomeDir: t.TempDir()}
	reset := time.Now().Add(time.Hour)
	if err := storage.WriteJSON(paths.AntigravityCache(), model.ProviderStatus{Windows: []model.Window{
		{Kind: "gemini-weekly", UsedPercent: 25, ResetsAt: reset},
	}}, 0o600); err != nil {
		t.Fatal(err)
	}

	service := New(paths, "test")
	found := false
	for _, source := range service.providers {
		if source.ID() == model.ProviderAntigravity {
			found = true
		}
	}
	if !found {
		t.Fatal("Antigravity provider was not registered")
	}
	status := service.Statuses()[model.ProviderAntigravity]
	if len(status.Windows) != 1 || status.Windows[0].Kind != "gemini-weekly" {
		t.Fatalf("loaded status = %#v", status)
	}
}

func TestClearProviderRemovesAntigravityCache(t *testing.T) {
	paths := config.Paths{DataDir: t.TempDir(), HomeDir: t.TempDir()}
	if err := storage.WriteJSON(paths.AntigravityCache(), model.ProviderStatus{}, 0o600); err != nil {
		t.Fatal(err)
	}
	service := New(paths, "test")

	service.ClearProvider(model.ProviderAntigravity)
	if _, err := os.Stat(paths.AntigravityCache()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cache stat error = %v, want not exist", err)
	}
}

func TestRefreshPreservesAntigravitySnapshotOnNoData(t *testing.T) {
	paths := config.Paths{DataDir: t.TempDir(), HomeDir: t.TempDir()}
	reset := time.Now().Add(time.Hour)
	service := &Service{
		paths:     paths,
		providers: []providerapi.Provider{errorProvider{id: model.ProviderAntigravity, err: antigravity.ErrNoQuotaData}},
		alerts:    alerts.NewManager(paths.AlertState()),
		statuses: map[model.Provider]model.ProviderStatus{
			model.ProviderAntigravity: {
				Provider: model.ProviderAntigravity,
				Windows:  []model.Window{{Kind: "gemini-weekly", UsedPercent: 25, ResetsAt: reset}},
			},
		},
	}

	service.Refresh(context.Background())
	status := service.Statuses()[model.ProviderAntigravity]
	if len(status.Windows) != 1 {
		t.Fatalf("snapshot was removed: %#v", status)
	}
	if status.Error != antigravity.ErrNoQuotaData.Error() {
		t.Fatalf("error = %q", status.Error)
	}
}
