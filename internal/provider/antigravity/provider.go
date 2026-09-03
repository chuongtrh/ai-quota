package antigravity

import (
	"context"
	"errors"
	"os"

	"github.com/chuongtrh/ai-quota/internal/config"
	"github.com/chuongtrh/ai-quota/internal/model"
	"github.com/chuongtrh/ai-quota/internal/storage"
)

type CacheProvider struct {
	paths   config.Paths
	fetchLS func(context.Context) (model.ProviderStatus, error)
}

func NewCacheProvider(paths config.Paths) *CacheProvider {
	return &CacheProvider{
		paths:   paths,
		fetchLS: FetchFromLanguageServer,
	}
}

func (p *CacheProvider) ID() model.Provider {
	return model.ProviderAntigravity
}

func (p *CacheProvider) Fetch(ctx context.Context) (model.ProviderStatus, error) {
	select {
	case <-ctx.Done():
		return model.ProviderStatus{Provider: p.ID()}, ctx.Err()
	default:
	}

	// 1. Try to fetch live quota from running Language Server (IDE or Desktop app)
	if p.fetchLS != nil {
		if status, err := p.fetchLS(ctx); err == nil && len(status.Windows) > 0 {
			_ = storage.WriteJSON(p.paths.AntigravityCache(), status, 0o600)
			return status, nil
		}
	}

	// 2. Fall back to cached quota file (written by bridge or previous LS fetch)
	var status model.ProviderStatus
	if err := storage.ReadJSON(p.paths.AntigravityCache(), &status); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.ProviderStatus{Provider: p.ID()}, ErrNoQuotaData
		}
		return model.ProviderStatus{Provider: p.ID()}, err
	}
	status.Provider = p.ID()
	status.Normalize()
	if len(status.Windows) == 0 {
		return model.ProviderStatus{Provider: p.ID()}, ErrNoQuotaData
	}
	return status, nil
}
