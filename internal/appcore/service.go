package appcore

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chuongtrh/ai-quota/internal/alerts"
	"github.com/chuongtrh/ai-quota/internal/config"
	"github.com/chuongtrh/ai-quota/internal/model"
	"github.com/chuongtrh/ai-quota/internal/notify"
	providerapi "github.com/chuongtrh/ai-quota/internal/provider"
	"github.com/chuongtrh/ai-quota/internal/provider/claude"
	"github.com/chuongtrh/ai-quota/internal/provider/codex"
	"github.com/chuongtrh/ai-quota/internal/storage"
)

type Service struct {
	paths      config.Paths
	providers  []providerapi.Provider
	alerts     *alerts.Manager
	mu         sync.RWMutex
	statuses   map[model.Provider]model.ProviderStatus
	refreshing atomic.Bool
}

func New(paths config.Paths, version string) *Service {
	service := &Service{
		paths: paths,
		providers: []providerapi.Provider{
			codex.New(version),
			claude.NewCacheProvider(paths),
		},
		alerts:   alerts.NewManager(paths.AlertState()),
		statuses: make(map[model.Provider]model.ProviderStatus),
	}
	service.loadCached(model.ProviderCodex, paths.CodexCache())
	service.loadCached(model.ProviderClaudeCode, paths.ClaudeCache())
	return service
}

func (s *Service) Refresh(ctx context.Context) {
	if !s.refreshing.CompareAndSwap(false, true) {
		return
	}
	defer s.refreshing.Store(false)

	for _, source := range s.providers {
		providerContext, cancel := context.WithTimeout(ctx, 15*time.Second)
		status, err := source.Fetch(providerContext)
		cancel()
		if err != nil {
			s.setError(source.ID(), friendlyProviderError(source.ID(), err))
			continue
		}
		if source.ID() == model.ProviderCodex {
			_ = storage.WriteJSON(s.paths.CodexCache(), status, 0o600)
		}
		s.setStatus(status)
	}

	for _, status := range s.Statuses() {
		for _, notice := range s.alerts.Evaluate(status, time.Now()) {
			_ = notify.Send(notice.Title, notice.Body)
		}
	}
}

func (s *Service) Statuses() map[model.Provider]model.ProviderStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[model.Provider]model.ProviderStatus, len(s.statuses))
	for provider, status := range s.statuses {
		status.Windows = append([]model.Window(nil), status.Windows...)
		result[provider] = status
	}
	return result
}

func (s *Service) MostUrgentRemaining(now time.Time) (int, bool) {
	statuses := s.Statuses()
	minimum := 101
	found := false
	for _, status := range statuses {
		for _, window := range status.Windows {
			if !window.ResetsAt.After(now) {
				continue
			}
			remaining := window.RoundedRemainingPercent()
			if remaining < minimum {
				minimum = remaining
				found = true
			}
		}
	}
	return minimum, found
}

// ClearProvider removes cached quota after tracking is disabled or reconfigured.
func (s *Service) ClearProvider(provider model.Provider) {
	s.mu.Lock()
	s.statuses[provider] = model.ProviderStatus{Provider: provider}
	s.mu.Unlock()

	path := ""
	switch provider {
	case model.ProviderCodex:
		path = s.paths.CodexCache()
	case model.ProviderClaudeCode:
		path = s.paths.ClaudeCache()
	}
	if path != "" {
		_ = os.Remove(path)
	}
}

func (s *Service) loadCached(provider model.Provider, path string) {
	var status model.ProviderStatus
	if err := storage.ReadJSON(path, &status); err != nil {
		s.statuses[provider] = model.ProviderStatus{Provider: provider}
		return
	}
	status.Provider = provider
	status.Normalize()
	s.statuses[provider] = status
}

func (s *Service) setStatus(status model.ProviderStatus) {
	status.Error = ""
	status.Normalize()
	s.mu.Lock()
	s.statuses[status.Provider] = status
	s.mu.Unlock()
}

func (s *Service) setError(provider model.Provider, message string) {
	s.mu.Lock()
	status := s.statuses[provider]
	status.Provider = provider
	status.Error = message
	s.statuses[provider] = status
	s.mu.Unlock()
}

func friendlyProviderError(id model.Provider, err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return id.DisplayName() + " timed out"
	}
	if id == model.ProviderCodex && errors.Is(err, codex.ErrNotInstalled) {
		return "Codex CLI is not installed or could not be found"
	}
	if id == model.ProviderClaudeCode && errors.Is(err, claude.ErrNoQuotaData) {
		return err.Error()
	}
	if id == model.ProviderClaudeCode {
		return "Could not read Claude Code quota data"
	}
	return err.Error()
}
