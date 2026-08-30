package provider

import (
	"context"

	"github.com/chuongtrh/ai-quota/internal/model"
)

// Provider is the extension boundary for every AI quota source.
// Each connector returns only normalized data from official integration surfaces.
type Provider interface {
	ID() model.Provider
	Fetch(ctx context.Context) (model.ProviderStatus, error)
}
