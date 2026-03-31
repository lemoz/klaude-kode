package provider

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

var ErrCredentialUnavailable = errors.New("credential is unavailable")
var ErrCredentialRefUnsupported = errors.New("credential reference scheme is unsupported")

type resolvedCredential struct {
	Value  string
	Source string
	Mode   credentialMode
}

type credentialMode string

const (
	credentialModeAPIKey credentialMode = "api_key"
	credentialModeBearer credentialMode = "bearer"
)

func resolveLiveCredential(profile contracts.AuthProfile) (resolvedCredential, error) {
	for _, candidate := range credentialCandidates(profile.Kind) {
		if value := strings.TrimSpace(profile.Settings[candidate.Key]); value != "" {
			return resolvedCredential{
				Value:  value,
				Source: candidate.Key,
				Mode:   candidate.Mode,
			}, nil
		}
	}

	credentialRef := strings.TrimSpace(profile.Settings["credential_ref"])
	if credentialRef == "" {
		return resolvedCredential{}, nil
	}
	if strings.HasPrefix(credentialRef, "env://") {
		name := strings.TrimSpace(strings.TrimPrefix(credentialRef, "env://"))
		if name == "" {
			return resolvedCredential{}, &Error{
				Code:      ErrorCodeAuthUnavailable,
				Message:   "env credential name is empty",
				Retryable: false,
			}
		}
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			return resolvedCredential{}, &Error{
				Code:      ErrorCodeAuthUnavailable,
				Message:   fmt.Sprintf("credential is unavailable: %s", credentialRef),
				Retryable: false,
			}
		}
		return resolvedCredential{
			Value:  value,
			Source: credentialRef,
			Mode:   defaultCredentialModeForProfile(profile.Kind),
		}, nil
	}
	if strings.HasPrefix(credentialRef, "keychain://") {
		return resolvedCredential{}, fmt.Errorf("%w: %s", ErrCredentialRefUnsupported, credentialRef)
	}
	return resolvedCredential{}, fmt.Errorf("%w: %s", ErrCredentialRefUnsupported, credentialRef)
}

type credentialCandidate struct {
	Key  string
	Mode credentialMode
}

func credentialCandidates(kind contracts.AuthProfileKind) []credentialCandidate {
	switch kind {
	case contracts.AuthProfileAnthropicOAuth:
		return []credentialCandidate{
			{Key: "oauth_access_token", Mode: credentialModeBearer},
			{Key: "access_token", Mode: credentialModeBearer},
			{Key: "api_key", Mode: credentialModeAPIKey},
		}
	case contracts.AuthProfileOpenRouterAPIKey:
		return []credentialCandidate{
			{Key: "api_key", Mode: credentialModeBearer},
			{Key: "access_token", Mode: credentialModeBearer},
		}
	case contracts.AuthProfileAnthropicAPIKey:
		fallthrough
	default:
		return []credentialCandidate{
			{Key: "api_key", Mode: credentialModeAPIKey},
		}
	}
}

func defaultCredentialModeForProfile(kind contracts.AuthProfileKind) credentialMode {
	switch kind {
	case contracts.AuthProfileAnthropicOAuth, contracts.AuthProfileOpenRouterAPIKey:
		return credentialModeBearer
	default:
		return credentialModeAPIKey
	}
}
