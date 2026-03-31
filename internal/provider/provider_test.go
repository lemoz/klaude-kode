package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
			"oauth_host":     "https://claude.ai",
			"account_scope":  "claude",
		},
	})
	if err != nil {
		t.Fatalf("ValidateProfile returned error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid profile, got %#v", result)
	}
}

func TestValidateProfileRejectsAnthropicOAuthWithoutOAuthSettings(t *testing.T) {
	ctx := context.Background()
	registry := DefaultRegistry()

	result, err := registry.ValidateProfile(ctx, contracts.AuthProfile{
		ID:       "anthropic-missing-oauth",
		Kind:     contracts.AuthProfileAnthropicOAuth,
		Provider: contracts.ProviderAnthropic,
		Settings: map[string]string{
			"credential_ref": "keychain://anthropic-missing-oauth",
		},
	})
	if err != nil {
		t.Fatalf("ValidateProfile returned error: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected invalid profile, got %#v", result)
	}
	if !strings.Contains(result.Message, "oauth_host") {
		t.Fatalf("expected oauth_host validation error, got %q", result.Message)
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

func TestValidateProfileRejectsOpenRouterWithoutAPIBase(t *testing.T) {
	ctx := context.Background()
	registry := DefaultRegistry()

	result, err := registry.ValidateProfile(ctx, contracts.AuthProfile{
		ID:       "openrouter-main",
		Kind:     contracts.AuthProfileOpenRouterAPIKey,
		Provider: contracts.ProviderOpenRouter,
		Settings: map[string]string{
			"credential_ref": "keychain://openrouter-main",
		},
	})
	if err != nil {
		t.Fatalf("ValidateProfile returned error: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected invalid profile, got %#v", result)
	}
	if !strings.Contains(result.Message, "api_base") {
		t.Fatalf("expected api_base validation error, got %q", result.Message)
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
			"api_base":       "https://openrouter.ai/api/v1",
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

func TestValidateModelRejectsUnknownAnthropicModel(t *testing.T) {
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

	err := registry.ValidateModel(ctx, profile, "claude-not-real")
	if err == nil {
		t.Fatalf("expected invalid model error")
	}
	providerErr := AsError(err)
	if providerErr == nil || providerErr.Code != ErrorCodeInvalidModel {
		t.Fatalf("expected invalid model provider error, got %v", err)
	}
}

func TestValidateModelAllowsCustomOpenRouterModel(t *testing.T) {
	ctx := context.Background()
	registry := DefaultRegistry()
	profile := contracts.AuthProfile{
		ID:       "openrouter-main",
		Kind:     contracts.AuthProfileOpenRouterAPIKey,
		Provider: contracts.ProviderOpenRouter,
		Settings: map[string]string{
			"credential_ref": "keychain://openrouter-main",
			"api_base":       "https://openrouter.ai/api/v1",
		},
	}

	if err := registry.ValidateModel(ctx, profile, "my/custom-model"); err != nil {
		t.Fatalf("expected openrouter custom model to validate, got %v", err)
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
			"api_key":  "secret",
			"api_base": "https://openrouter.ai/api/v1",
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
			"api_base":       "https://openrouter.ai/api/v1",
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

func TestCompleteUsesLiveAnthropicAPIWhenEnvCredentialAndAPIBasePresent(t *testing.T) {
	ctx := context.Background()
	registry := DefaultRegistry()
	t.Setenv("ANTHROPIC_TEST_KEY", "anthropic-secret")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("expected /v1/messages path, got %s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "anthropic-secret" {
			t.Fatalf("expected x-api-key header, got %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Fatalf("expected anthropic-version header, got %q", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload["model"] != "claude-sonnet-4-6" {
			t.Fatalf("expected model claude-sonnet-4-6, got %#v", payload["model"])
		}

		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"live anthropic reply"}]}`))
	}))
	defer server.Close()

	profile := contracts.AuthProfile{
		ID:       "anthropic-live",
		Kind:     contracts.AuthProfileAnthropicAPIKey,
		Provider: contracts.ProviderAnthropic,
		Settings: map[string]string{
			"credential_ref": "env://ANTHROPIC_TEST_KEY",
			"api_base":       server.URL,
		},
	}

	result, err := registry.Complete(ctx, profile, contracts.CompletionRequest{
		Model: "claude-sonnet-4-6",
		Messages: []contracts.CanonicalMessage{
			{Role: "user", Content: "hello live anthropic"},
		},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if result.Message.Content != "live anthropic reply" {
		t.Fatalf("expected live anthropic reply, got %q", result.Message.Content)
	}
}

func TestCompleteUsesLiveAnthropicOAuthWhenAccessTokenPresent(t *testing.T) {
	ctx := context.Background()
	registry := DefaultRegistry()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("expected /v1/messages path, got %s", r.URL.Path)
		}
		if got := r.Header.Get("authorization"); got != "Bearer oauth-secret" {
			t.Fatalf("expected bearer authorization header, got %q", got)
		}
		if got := r.Header.Get("anthropic-beta"); got != "oauth-2025-04-20" {
			t.Fatalf("expected oauth beta header, got %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Fatalf("expected anthropic-version header, got %q", got)
		}

		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"live anthropic oauth reply"}]}`))
	}))
	defer server.Close()

	profile := contracts.AuthProfile{
		ID:           "anthropic-main",
		Kind:         contracts.AuthProfileAnthropicOAuth,
		Provider:     contracts.ProviderAnthropic,
		DefaultModel: "claude-sonnet-4-6",
		Settings: map[string]string{
			"oauth_access_token": "oauth-secret",
			"oauth_host":         "https://claude.ai",
			"account_scope":      "claude",
			"api_base":           server.URL,
		},
	}

	result, err := registry.Complete(ctx, profile, contracts.CompletionRequest{
		Model: "claude-sonnet-4-6",
		Messages: []contracts.CanonicalMessage{
			{Role: "user", Content: "hello live anthropic oauth"},
		},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if result.Message.Content != "live anthropic oauth reply" {
		t.Fatalf("expected live anthropic oauth reply, got %q", result.Message.Content)
	}
}

func TestCompleteUsesLiveOpenRouterAPIWhenEnvCredentialAndAPIBasePresent(t *testing.T) {
	ctx := context.Background()
	registry := DefaultRegistry()
	t.Setenv("OPENROUTER_TEST_KEY", "openrouter-secret")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("expected /chat/completions path, got %s", r.URL.Path)
		}
		if got := r.Header.Get("authorization"); got != "Bearer openrouter-secret" {
			t.Fatalf("expected authorization header, got %q", got)
		}
		if got := r.Header.Get("http-referer"); got != "https://local.cli" {
			t.Fatalf("expected http-referer header, got %q", got)
		}
		if got := r.Header.Get("x-title"); got != "Klaude Kode" {
			t.Fatalf("expected x-title header, got %q", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload["model"] != "openrouter/auto" {
			t.Fatalf("expected model openrouter/auto, got %#v", payload["model"])
		}

		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"live openrouter reply"}}]}`))
	}))
	defer server.Close()

	profile := contracts.AuthProfile{
		ID:       "openrouter-live",
		Kind:     contracts.AuthProfileOpenRouterAPIKey,
		Provider: contracts.ProviderOpenRouter,
		Settings: map[string]string{
			"credential_ref": "env://OPENROUTER_TEST_KEY",
			"api_base":       server.URL,
			"http_referer":   "https://local.cli",
			"app_name":       "Klaude Kode",
		},
	}

	result, err := registry.Complete(ctx, profile, contracts.CompletionRequest{
		Model: "openrouter/auto",
		Messages: []contracts.CanonicalMessage{
			{Role: "user", Content: "hello live openrouter"},
		},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if result.Message.Content != "live openrouter reply" {
		t.Fatalf("expected live openrouter reply, got %q", result.Message.Content)
	}
}

func TestCompleteFailsWhenEnvCredentialIsMissing(t *testing.T) {
	ctx := context.Background()
	registry := DefaultRegistry()

	_, err := registry.Complete(ctx, contracts.AuthProfile{
		ID:       "anthropic-live",
		Kind:     contracts.AuthProfileAnthropicAPIKey,
		Provider: contracts.ProviderAnthropic,
		Settings: map[string]string{
			"credential_ref": "env://ANTHROPIC_TEST_KEY_MISSING",
			"api_base":       "https://api.anthropic.com",
		},
	}, contracts.CompletionRequest{
		Model: "claude-sonnet-4-6",
		Messages: []contracts.CanonicalMessage{
			{Role: "user", Content: "hello missing credential"},
		},
	})
	if err == nil {
		t.Fatalf("expected missing env credential error")
	}
	providerErr := AsError(err)
	if providerErr == nil || providerErr.Code != ErrorCodeAuthUnavailable {
		t.Fatalf("expected auth_unavailable provider error, got %v", err)
	}
	if !strings.Contains(err.Error(), "env://ANTHROPIC_TEST_KEY_MISSING") {
		t.Fatalf("expected missing env credential in error, got %v", err)
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
