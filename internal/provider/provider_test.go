package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestDefaultRegistryContainsAnthropicAndOpenRouter(t *testing.T) {
	registry := DefaultRegistry()

	if _, err := registry.Get(contracts.ProviderAnthropic); err != nil {
		t.Fatalf("expected anthropic adapter, got error: %v", err)
	}
	if _, err := registry.Get(contracts.ProviderOpenRouter); err != nil {
		t.Fatalf("expected openrouter adapter, got error: %v", err)
	}
}

func TestValidateProfileAcceptsCredentialReference(t *testing.T) {
	ctx := context.Background()
	registry := DefaultRegistry()

	result, err := registry.ValidateProfile(ctx, contracts.AuthProfile{
		ID:           "anthropic-main",
		Kind:         contracts.AuthProfileAnthropicOAuth,
		Provider:     contracts.ProviderAnthropic,
		DisplayName:  "Anthropic Main",
		DefaultModel: "claude-sonnet-4-6",
		Settings: map[string]string{
			"credential_ref": "keychain://anthropic-main",
		},
	})
	if err != nil {
		t.Fatalf("ValidateProfile returned error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid profile, got %#v", result)
	}
}

func TestValidateProfileRejectsProviderMismatch(t *testing.T) {
	ctx := context.Background()
	registry := DefaultRegistry()

	result, err := registry.ValidateProfile(ctx, contracts.AuthProfile{
		ID:       "bad-openrouter",
		Kind:     contracts.AuthProfileOpenRouterAPIKey,
		Provider: contracts.ProviderAnthropic,
		Settings: map[string]string{
			"api_key": "secret",
		},
	})
	if err != nil {
		t.Fatalf("ValidateProfile returned error: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected invalid profile, got %#v", result)
	}
}

func TestOpenRouterCapabilitiesAllowCustomModels(t *testing.T) {
	ctx := context.Background()
	registry := DefaultRegistry()
	profile := contracts.AuthProfile{
		ID:       "openrouter-main",
		Kind:     contracts.AuthProfileOpenRouterAPIKey,
		Provider: contracts.ProviderOpenRouter,
		Settings: map[string]string{
			"credential_ref": "keychain://openrouter-main",
		},
	}

	capabilities, err := registry.Capabilities(ctx, profile, "my/custom-model")
	if err != nil {
		t.Fatalf("Capabilities returned error: %v", err)
	}
	if !capabilities.Streaming || !capabilities.ToolCalling {
		t.Fatalf("expected openrouter capabilities to enable streaming and tools, got %#v", capabilities)
	}
	if capabilities.PromptCaching {
		t.Fatalf("expected openrouter prompt caching to be disabled")
	}
}

func TestCountTokensReturnsDeterministicEstimate(t *testing.T) {
	ctx := context.Background()
	registry := DefaultRegistry()
	profile := contracts.AuthProfile{
		ID:       "openrouter-main",
		Kind:     contracts.AuthProfileOpenRouterAPIKey,
		Provider: contracts.ProviderOpenRouter,
		Settings: map[string]string{
			"api_key": "secret",
		},
	}

	result, err := registry.CountTokens(ctx, profile, contracts.TokenCountRequest{
		Model: "openrouter/auto",
		Messages: []contracts.CanonicalMessage{
			{Role: "system", Content: "be precise"},
			{Role: "user", Content: "count these words please"},
		},
	})
	if err != nil {
		t.Fatalf("CountTokens returned error: %v", err)
	}
	if result.InputTokens <= 0 {
		t.Fatalf("expected positive token count, got %d", result.InputTokens)
	}
}

func TestCompletionStubsAreExplicitlyUnimplemented(t *testing.T) {
	ctx := context.Background()
	adapter := NewAnthropicAdapter()
	profile := contracts.AuthProfile{
		ID:       "anthropic-main",
		Kind:     contracts.AuthProfileAnthropicAPIKey,
		Provider: contracts.ProviderAnthropic,
		Settings: map[string]string{
			"api_key": "secret",
		},
	}

	_, err := adapter.Complete(ctx, profile, contracts.CompletionRequest{})
	if !errors.Is(err, ErrCompletionNotImplemented) {
		t.Fatalf("expected ErrCompletionNotImplemented, got %v", err)
	}
}
