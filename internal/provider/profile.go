package provider

import (
	"fmt"
	"strings"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func ResolveSessionProfile(profileID string, model string) contracts.AuthProfile {
	providerKind := inferProviderKind(profileID, model)
	normalizedProfileID := strings.TrimSpace(profileID)
	if normalizedProfileID == "" {
		normalizedProfileID = fmt.Sprintf("%s-default", providerKind)
	}

	authKind := contracts.AuthProfileAnthropicAPIKey
	switch providerKind {
	case contracts.ProviderOpenRouter:
		authKind = contracts.AuthProfileOpenRouterAPIKey
	case contracts.ProviderAnthropic:
		if strings.Contains(strings.ToLower(normalizedProfileID), "oauth") {
			authKind = contracts.AuthProfileAnthropicOAuth
		}
	}

	defaultModel := strings.TrimSpace(model)
	if defaultModel == "" {
		defaultModel = defaultModelForProvider(providerKind)
	}

	return contracts.AuthProfile{
		ID:           normalizedProfileID,
		Kind:         authKind,
		Provider:     providerKind,
		DisplayName:  normalizedProfileID,
		DefaultModel: defaultModel,
		Settings: map[string]string{
			"credential_ref": fmt.Sprintf("session://%s", normalizedProfileID),
		},
	}
}

func IsLegacyProfileID(profileID string) bool {
	switch strings.ToLower(strings.TrimSpace(profileID)) {
	case "", "default", "shell-default", "local-default", "headless-default", "anthropic-default", "openrouter-default":
		return true
	default:
		return false
	}
}

func defaultModelForProvider(kind contracts.ProviderKind) string {
	switch kind {
	case contracts.ProviderOpenRouter:
		return "openrouter/auto"
	case contracts.ProviderAnthropic:
		fallthrough
	default:
		return "claude-sonnet-4-6"
	}
}

func inferProviderKind(profileID string, model string) contracts.ProviderKind {
	lowerProfileID := strings.ToLower(strings.TrimSpace(profileID))
	lowerModel := strings.ToLower(strings.TrimSpace(model))

	switch {
	case strings.Contains(lowerProfileID, "openrouter"):
		return contracts.ProviderOpenRouter
	case strings.Contains(lowerProfileID, "anthropic"):
		return contracts.ProviderAnthropic
	case strings.Contains(lowerModel, "/"):
		return contracts.ProviderOpenRouter
	default:
		return contracts.ProviderAnthropic
	}
}
