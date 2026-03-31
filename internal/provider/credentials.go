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
}

func resolveLiveCredential(profile contracts.AuthProfile) (resolvedCredential, error) {
	for _, key := range []string{"api_key", "oauth_access_token", "access_token"} {
		if value := strings.TrimSpace(profile.Settings[key]); value != "" {
			return resolvedCredential{
				Value:  value,
				Source: key,
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
			return resolvedCredential{}, fmt.Errorf("%w: env credential name is empty", ErrCredentialUnavailable)
		}
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			return resolvedCredential{}, fmt.Errorf("%w: %s", ErrCredentialUnavailable, credentialRef)
		}
		return resolvedCredential{
			Value:  value,
			Source: credentialRef,
		}, nil
	}
	if strings.HasPrefix(credentialRef, "keychain://") {
		return resolvedCredential{}, fmt.Errorf("%w: %s", ErrCredentialRefUnsupported, credentialRef)
	}
	return resolvedCredential{}, fmt.Errorf("%w: %s", ErrCredentialRefUnsupported, credentialRef)
}
