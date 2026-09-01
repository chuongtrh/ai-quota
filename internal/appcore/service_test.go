package appcore

import (
	"context"
	"testing"
	"time"

	"github.com/chuongtrh/ai-quota/internal/alerts"
	"github.com/chuongtrh/ai-quota/internal/config"
	"github.com/chuongtrh/ai-quota/internal/model"
	providerapi "github.com/chuongtrh/ai-quota/internal/provider"
)

type blockingProvider struct {
	id      model.Provider
	started chan<- model.Provider
	release <-chan struct{}
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
