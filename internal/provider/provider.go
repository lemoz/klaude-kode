package provider

import (
	"context"
	"errors"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

var ErrCompletionNotImplemented = errors.New("provider completion not implemented")

type Adapter interface {
	Kind() contracts.ProviderKind
	ListModels(ctx context.Context, profile contracts.AuthProfile) ([]string, error)
	CountTokens(ctx context.Context, profile contracts.AuthProfile, req contracts.TokenCountRequest) (contracts.TokenCountResult, error)
	StreamCompletion(ctx context.Context, profile contracts.AuthProfile, req contracts.CompletionRequest) (<-chan contracts.ProviderEvent, error)
	Complete(ctx context.Context, profile contracts.AuthProfile, req contracts.CompletionRequest) (contracts.CompletionResult, error)
	ValidateProfile(ctx context.Context, profile contracts.AuthProfile) (contracts.ProfileValidationResult, error)
	Capabilities(ctx context.Context, profile contracts.AuthProfile, model string) (contracts.CapabilitySet, error)
}
