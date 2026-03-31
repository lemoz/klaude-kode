package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

type staticAdapter struct {
	kind          contracts.ProviderKind
	models        []string
	capabilities  contracts.CapabilitySet
	supportedAuth map[contracts.AuthProfileKind]struct{}
}

func NewAnthropicAdapter() Adapter {
	return &staticAdapter{
		kind: contracts.ProviderAnthropic,
		models: []string{
			"claude-sonnet-4-6",
			"claude-opus-4-6",
		},
		capabilities: contracts.CapabilitySet{
			Streaming:          true,
			ToolCalling:        true,
			StructuredOutputs:  true,
			TokenCounting:      true,
			PromptCaching:      true,
			ReasoningControls:  true,
			DeferredToolSearch: true,
			ImageInput:         true,
			DocumentInput:      true,
		},
		supportedAuth: map[contracts.AuthProfileKind]struct{}{
			contracts.AuthProfileAnthropicOAuth:  {},
			contracts.AuthProfileAnthropicAPIKey: {},
		},
	}
}

func NewOpenRouterAdapter() Adapter {
	return &staticAdapter{
		kind: contracts.ProviderOpenRouter,
		models: []string{
			"openrouter/auto",
			"anthropic/claude-sonnet-4.5",
			"anthropic/claude-opus-4.1",
		},
		capabilities: contracts.CapabilitySet{
			Streaming:          true,
			ToolCalling:        true,
			StructuredOutputs:  true,
			TokenCounting:      true,
			PromptCaching:      false,
			ReasoningControls:  true,
			DeferredToolSearch: false,
			ImageInput:         true,
			DocumentInput:      true,
		},
		supportedAuth: map[contracts.AuthProfileKind]struct{}{
			contracts.AuthProfileOpenRouterAPIKey: {},
		},
	}
}

func (a *staticAdapter) Kind() contracts.ProviderKind {
	return a.kind
}

func (a *staticAdapter) ListModels(_ context.Context, profile contracts.AuthProfile) ([]string, error) {
	if err := a.ensureProfile(profile); err != nil {
		return nil, err
	}
	return append([]string(nil), a.models...), nil
}

func (a *staticAdapter) CountTokens(_ context.Context, profile contracts.AuthProfile, req contracts.TokenCountRequest) (contracts.TokenCountResult, error) {
	if err := a.ensureProfile(profile); err != nil {
		return contracts.TokenCountResult{}, err
	}

	total := 0
	for _, message := range req.Messages {
		total += len(strings.Fields(message.Content)) + 4
	}
	if req.Model != "" {
		total += 2
	}
	return contracts.TokenCountResult{InputTokens: total}, nil
}

func (a *staticAdapter) StreamCompletion(_ context.Context, profile contracts.AuthProfile, _ contracts.CompletionRequest) (<-chan contracts.ProviderEvent, error) {
	if err := a.ensureProfile(profile); err != nil {
		return nil, err
	}
	return nil, ErrCompletionNotImplemented
}

func (a *staticAdapter) Complete(ctx context.Context, profile contracts.AuthProfile, req contracts.CompletionRequest) (contracts.CompletionResult, error) {
	if err := a.ensureProfile(profile); err != nil {
		return contracts.CompletionResult{}, err
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(profile.DefaultModel)
	}
	if model == "" && len(a.models) > 0 {
		model = a.models[0]
	}

	lastUserText := ""
	for index := len(req.Messages) - 1; index >= 0; index-- {
		if strings.EqualFold(req.Messages[index].Role, "user") {
			lastUserText = strings.TrimSpace(req.Messages[index].Content)
			break
		}
	}
	if lastUserText == "" {
		lastUserText = "no user message provided"
	}

	liveResult, liveUsed, err := a.maybeCompleteLive(ctx, profile, req, model)
	if err != nil {
		return contracts.CompletionResult{}, err
	}
	if liveUsed {
		return liveResult, nil
	}

	return contracts.CompletionResult{
		Message: contracts.CanonicalMessage{
			Role: "assistant",
			Content: fmt.Sprintf(
				"%s response from %s. Received: %s",
				a.responsePrefix(),
				model,
				lastUserText,
			),
		},
	}, nil
}

func (a *staticAdapter) ValidateProfile(_ context.Context, profile contracts.AuthProfile) (contracts.ProfileValidationResult, error) {
	if profile.Provider != a.kind {
		return contracts.ProfileValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("provider mismatch: expected %s", a.kind),
		}, nil
	}
	if profile.ID == "" {
		return contracts.ProfileValidationResult{
			Valid:   false,
			Message: "profile id is required",
		}, nil
	}
	if _, ok := a.supportedAuth[profile.Kind]; !ok {
		return contracts.ProfileValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("unsupported profile kind %s", profile.Kind),
		}, nil
	}
	if !hasCredential(profile) {
		return contracts.ProfileValidationResult{
			Valid:   false,
			Message: "credential reference or inline credential is required",
		}, nil
	}
	if err := validateCredentialReference(profile); err != nil {
		return contracts.ProfileValidationResult{
			Valid:   false,
			Message: err.Error(),
		}, nil
	}
	if err := validateProviderSettings(profile); err != nil {
		return contracts.ProfileValidationResult{
			Valid:   false,
			Message: err.Error(),
		}, nil
	}

	return contracts.ProfileValidationResult{
		Valid:   true,
		Message: "profile is valid",
	}, nil
}

func (a *staticAdapter) Capabilities(_ context.Context, profile contracts.AuthProfile, _ string) (contracts.CapabilitySet, error) {
	if err := a.ensureProfile(profile); err != nil {
		return contracts.CapabilitySet{}, err
	}
	return a.capabilities, nil
}

func (a *staticAdapter) ensureProfile(profile contracts.AuthProfile) error {
	if profile.Provider != a.kind {
		return fmt.Errorf("provider mismatch: expected %s, got %s", a.kind, profile.Provider)
	}
	return nil
}

func hasCredential(profile contracts.AuthProfile) bool {
	for _, key := range []string{
		"credential_ref",
		"api_key",
		"oauth_access_token",
		"access_token",
	} {
		if value := strings.TrimSpace(profile.Settings[key]); value != "" {
			return true
		}
	}
	return false
}

func validateCredentialReference(profile contracts.AuthProfile) error {
	credentialRef := strings.TrimSpace(profile.Settings["credential_ref"])
	if credentialRef == "" {
		return nil
	}
	if !strings.Contains(credentialRef, "://") {
		return fmt.Errorf("credential_ref must include a scheme")
	}
	return nil
}

func validateProviderSettings(profile contracts.AuthProfile) error {
	switch profile.Kind {
	case contracts.AuthProfileAnthropicOAuth:
		if strings.TrimSpace(profile.Settings["oauth_host"]) == "" {
			return fmt.Errorf("oauth_host is required for anthropic_oauth profiles")
		}
		if strings.TrimSpace(profile.Settings["account_scope"]) == "" {
			return fmt.Errorf("account_scope is required for anthropic_oauth profiles")
		}
	case contracts.AuthProfileAnthropicAPIKey:
		return nil
	case contracts.AuthProfileOpenRouterAPIKey:
		if strings.TrimSpace(profile.Settings["api_base"]) == "" {
			return fmt.Errorf("api_base is required for openrouter_api_key profiles")
		}
	}
	return nil
}

func (a *staticAdapter) responsePrefix() string {
	switch a.kind {
	case contracts.ProviderOpenRouter:
		return "OpenRouter"
	case contracts.ProviderAnthropic:
		fallthrough
	default:
		return "Anthropic"
	}
}
