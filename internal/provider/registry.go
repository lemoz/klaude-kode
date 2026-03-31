package provider

import (
	"context"
	"fmt"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

type Registry struct {
	adapters map[contracts.ProviderKind]Adapter
}

func NewRegistry(adapters ...Adapter) *Registry {
	registry := &Registry{
		adapters: make(map[contracts.ProviderKind]Adapter, len(adapters)),
	}
	for _, adapter := range adapters {
		registry.Register(adapter)
	}
	return registry
}

func DefaultRegistry() *Registry {
	return NewRegistry(
		NewAnthropicAdapter(),
		NewOpenRouterAdapter(),
	)
}

func (r *Registry) Register(adapter Adapter) {
	if adapter == nil {
		return
	}
	r.adapters[adapter.Kind()] = adapter
}

func (r *Registry) Get(kind contracts.ProviderKind) (Adapter, error) {
	adapter, ok := r.adapters[kind]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", kind)
	}
	return adapter, nil
}

func (r *Registry) ListModels(ctx context.Context, profile contracts.AuthProfile) ([]string, error) {
	adapter, err := r.Get(profile.Provider)
	if err != nil {
		return nil, err
	}
	return adapter.ListModels(ctx, profile)
}

func (r *Registry) CountTokens(ctx context.Context, profile contracts.AuthProfile, req contracts.TokenCountRequest) (contracts.TokenCountResult, error) {
	adapter, err := r.Get(profile.Provider)
	if err != nil {
		return contracts.TokenCountResult{}, err
	}
	return adapter.CountTokens(ctx, profile, req)
}

func (r *Registry) ValidateProfile(ctx context.Context, profile contracts.AuthProfile) (contracts.ProfileValidationResult, error) {
	adapter, err := r.Get(profile.Provider)
	if err != nil {
		return contracts.ProfileValidationResult{}, err
	}
	return adapter.ValidateProfile(ctx, profile)
}

func (r *Registry) Capabilities(ctx context.Context, profile contracts.AuthProfile, model string) (contracts.CapabilitySet, error) {
	adapter, err := r.Get(profile.Provider)
	if err != nil {
		return contracts.CapabilitySet{}, err
	}
	return adapter.Capabilities(ctx, profile, model)
}

func (r *Registry) Complete(ctx context.Context, profile contracts.AuthProfile, req contracts.CompletionRequest) (contracts.CompletionResult, error) {
	adapter, err := r.Get(profile.Provider)
	if err != nil {
		return contracts.CompletionResult{}, err
	}
	return adapter.Complete(ctx, profile, req)
}

func (r *Registry) StreamCompletion(ctx context.Context, profile contracts.AuthProfile, req contracts.CompletionRequest) (<-chan contracts.ProviderEvent, error) {
	adapter, err := r.Get(profile.Provider)
	if err != nil {
		return nil, err
	}
	return adapter.StreamCompletion(ctx, profile, req)
}
