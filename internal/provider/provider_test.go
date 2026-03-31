package provider

import (
	"context"
	"strings"
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

func TestCompleteReturnsAnthropicResponse(t *testing.T) {
	ctx := context.Background()
	registry := DefaultRegistry()
	profile := contracts.AuthProfile{
		ID:       "anthropic-main",
		Kind:     contracts.AuthProfileAnthropicAPIKey,
		Provider: contracts.ProviderAnthropic,
		Settings: map[string]string{
			"api_key": "secret",
		},
	}

	result, err := registry.Complete(ctx, profile, contracts.CompletionRequest{
		Model: "claude-sonnet-4-6",
		Messages: []contracts.CanonicalMessage{
			{Role: "user", Content: "hello from provider test"},
		},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if !strings.Contains(result.Message.Content, "Anthropic response from claude-sonnet-4-6") {
		t.Fatalf("expected anthropic response content, got %q", result.Message.Content)
	}
}

func TestCompleteAllowsOpenRouterCustomModels(t *testing.T) {
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

	result, err := registry.Complete(ctx, profile, contracts.CompletionRequest{
		Model: "my/custom-model",
		Messages: []contracts.CanonicalMessage{
			{Role: "user", Content: "route this through openrouter"},
		},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if !strings.Contains(result.Message.Content, "OpenRouter response from my/custom-model") {
		t.Fatalf("expected openrouter response content, got %q", result.Message.Content)
	}
}

func TestResolveSessionProfileInfersOpenRouterFromModel(t *testing.T) {
	profile := ResolveSessionProfile("shell-default", "openrouter/auto")
	if profile.Provider != contracts.ProviderOpenRouter {
		t.Fatalf("expected openrouter provider, got %s", profile.Provider)
	}
	if profile.Kind != contracts.AuthProfileOpenRouterAPIKey {
		t.Fatalf("expected openrouter api key profile kind, got %s", profile.Kind)
	}
}

func TestResolveSessionProfileDefaultsToAnthropic(t *testing.T) {
	profile := ResolveSessionProfile("shell-default", "shell-bootstrap-model")
	if profile.Provider != contracts.ProviderAnthropic {
		t.Fatalf("expected anthropic provider, got %s", profile.Provider)
	}
	if profile.DefaultModel != "shell-bootstrap-model" {
		t.Fatalf("expected session model passthrough, got %q", profile.DefaultModel)
	}
}
