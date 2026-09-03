package claude

import (
	"context"
	"errors"
	"os"

	"github.com/chuongtrh/ai-quota/internal/config"
	"github.com/chuongtrh/ai-quota/internal/model"
	"github.com/chuongtrh/ai-quota/internal/storage"
)

var ErrNoQuotaData = errors.New("Claude Code has not sent quota data yet; send at least one prompt first")

type CacheProvider struct {
	paths config.Paths
}

func NewCacheProvider(paths config.Paths) *CacheProvider {
	return &CacheProvider{paths: paths}
}

func (p *CacheProvider) ID() model.Provider {
	return model.ProviderClaudeCode
}

func (p *CacheProvider) Fetch(ctx context.Context) (model.ProviderStatus, error) {
	select {
	case <-ctx.Done():
		return model.ProviderStatus{Provider: p.ID()}, ctx.Err()
	default:
	}
	var status model.ProviderStatus
	if err := storage.ReadJSON(p.paths.ClaudeCache(), &status); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.ProviderStatus{Provider: p.ID()}, ErrNoQuotaData
		}
		return model.ProviderStatus{Provider: p.ID()}, err
	}
	status.Provider = p.ID()
	status.Normalize()
	return status, nil
}
